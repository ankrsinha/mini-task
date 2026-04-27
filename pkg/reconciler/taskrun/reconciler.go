package taskrun

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/logging"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	minitaskclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
	minitaskv1listers "github.com/ankrsinha/mini-task/pkg/generated/listers/minitask/v1"
)

type Reconciler struct {
	kubeClient    kubernetes.Interface
	customClient  minitaskclient.Interface
	taskRunLister minitaskv1listers.TaskRunLister
	taskLister    minitaskv1listers.TaskLister
	podLister     corelisters.PodLister
}

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", key)
	}

	logger.Infof("Reconciling TaskRun %s/%s", namespace, name)

	original, err := r.taskRunLister.TaskRuns(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Infof("TaskRun %s deleted, skipping", key)
			return nil
		}
		return err
	}

	tr := original.DeepCopy()

	logger.Infof("Current phase: %s", tr.Status.Phase)

	switch tr.Status.Phase {
	case "Succeeded", "Failed":
		logger.Infof("TaskRun already completed, skipping")
		return nil
	case "":
		return r.handleNewTaskRun(ctx, tr)
	case "Pending", "Running":
		return r.handleActiveTaskRun(ctx, tr)
	default:
		logger.Infof("Unknown phase: %s", tr.Status.Phase)
		return nil
	}
}

func (r *Reconciler) handleNewTaskRun(ctx context.Context, tr *miniv1.TaskRun) error {
	logger := logging.FromContext(ctx)

	task, err := r.taskLister.Tasks(tr.Namespace).Get(tr.Spec.TaskRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Errorf("Referenced Task %s not found", tr.Spec.TaskRef)
			tr.Status.Phase = "Failed"
			_, updateErr := r.customClient.MinitaskV1().TaskRuns(tr.Namespace).UpdateStatus(ctx, tr, metav1.UpdateOptions{})
			return updateErr
		}
		return err
	}

	podName := tr.Name + "-pod"

	_, err = r.podLister.Pods(tr.Namespace).Get(podName)
	if err == nil {
		logger.Infof("Pod %s already exists", podName)
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	pod := r.buildPod(podName, tr, task)

	logger.Infof("Creating Pod %s", podName)
	_, err = r.kubeClient.CoreV1().Pods(tr.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Infof("Pod %s already exists (race condition)", podName)
			return nil
		}
		return fmt.Errorf("failed to create pod: %w", err)
	}

	logger.Infof("Pod %s created successfully", podName)
	tr.Status.Phase = "Pending"
	tr.Status.PodName = podName
	_, err = r.customClient.MinitaskV1().TaskRuns(tr.Namespace).UpdateStatus(ctx, tr, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) handleActiveTaskRun(ctx context.Context, tr *miniv1.TaskRun) error {
	logger := logging.FromContext(ctx)

	pod, err := r.podLister.Pods(tr.Namespace).Get(tr.Status.PodName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Pod missing, marking TaskRun as Failed")
			tr.Status.Phase = "Failed"
			now := metav1.Now()
			tr.Status.FinishTime = &now
			_, updateErr := r.customClient.MinitaskV1().TaskRuns(tr.Namespace).UpdateStatus(ctx, tr, metav1.UpdateOptions{})
			return updateErr
		}
		return err
	}

	oldPhase := tr.Status.Phase
	newPhase := oldPhase

	switch pod.Status.Phase {
	case corev1.PodPending:
		newPhase = "Pending"
	case corev1.PodRunning:
		newPhase = "Running"
		if tr.Status.StartTime == nil {
			now := metav1.Now()
			tr.Status.StartTime = &now
		}
	case corev1.PodSucceeded:
		newPhase = "Succeeded"
		now := metav1.Now()
		tr.Status.FinishTime = &now
	case corev1.PodFailed:
		newPhase = "Failed"
		now := metav1.Now()
		tr.Status.FinishTime = &now
	}

	if oldPhase != newPhase {
		logger.Infof("Phase transition: %s -> %s", oldPhase, newPhase)
		tr.Status.Phase = newPhase
		_, err = r.customClient.MinitaskV1().TaskRuns(tr.Namespace).UpdateStatus(ctx, tr, metav1.UpdateOptions{})
		return err
	}

	return nil
}

func (r *Reconciler) buildPod(podName string, tr *miniv1.TaskRun, task *miniv1.Task) *corev1.Pod {
	var containers []corev1.Container
	for _, step := range task.Spec.Steps {
		containers = append(containers, corev1.Container{
			Name:    step.Name,
			Image:   step.Image,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{step.Script},
		})
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: tr.Namespace,
			Labels: map[string]string{
				"minitask": tr.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(tr, miniv1.SchemeGroupVersion.WithKind("TaskRun")),
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    containers,
		},
	}
}