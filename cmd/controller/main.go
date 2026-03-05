package main

import (
	"context"
	"os"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type TaskRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var logf = ctrl.Log.WithName("mini-task-controller")

func main() {

	// Initializing logger
	ctrl.SetLogger(zap.New())
	logf.Info("Logger Initialized")

	// Creating runtime scheme
	scheme := runtime.NewScheme()

	// Registering core Kubernetes APIs
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		logf.Error(err, "Unable to register Kubernetes scheme")
		os.Exit(1)
	}

	// Registering custom resource APIs
	err = miniv1.AddToScheme(scheme)
	if err != nil {
		logf.Error(err, "Unable to register mini-task scheme")
		os.Exit(1)
	}

	logf.Info("Scheme Registered")

	// Creating New Manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		logf.Error(err, "Unable to create manager")
		os.Exit(1)
	}

	logf.Info("Manager created successfully")

	// Creating New controller and attaching it to the manager
	err = ctrl.NewControllerManagedBy(mgr).
		For(&miniv1.TaskRun{}).
		Owns(&corev1.Pod{}).
		Complete(&TaskRunReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		})

	if err != nil {
		os.Exit(1)
	}

	// Starting manager
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		logf.Error(err, "Problem running manager")
		os.Exit(1)
	}

	logf.Info("Manager Started Successfully")
}

// Reconciler

func (r *TaskRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// log := ctrl.Log.WithName("reconciler").WithValues("taskrun", req.Name)

	// Fetch TaskRun
	var tr miniv1.TaskRun
	if err := r.Get(ctx, req.NamespacedName, &tr); err != nil {
		// If deleted then ignore
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch tr.Status.Phase {

	case "Succeeded", "Failed":
		logf.Info("TaskRun already completed. Skipping.")
		return ctrl.Result{}, nil

	case "":
		logf.Info("New TaskRun found! Creating Pod...")
		return r.handleNewTaskRun(ctx, &tr)

	case "Pending", "Running":
		logf.Info("TaskRun is active. Checking Pod status...")
		return r.handleActiveTaskRun(ctx, &tr)

	default:
		logf.Info("Unknown Phase", "phase", tr.Status.Phase)
		return ctrl.Result{}, nil
	}

}

// Handle new taskrun

func (r *TaskRunReconciler) handleNewTaskRun(ctx context.Context, tr *miniv1.TaskRun) (ctrl.Result, error) {
	// log := ctrl.Log.WithName("handleNew")

	// Find Task
	var task miniv1.Task

	taskKey := types.NamespacedName{Name: tr.Spec.TaskRef, Namespace: tr.Namespace}
	if err := r.Get(ctx, taskKey, &task); err != nil {
		logf.Error(err, "Referenced Task not found", "task", tr.Spec.TaskRef)
		// If task not found then status = failed
		tr.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, tr)
		return ctrl.Result{}, nil
	}

	// Pod Manifest
	podName := tr.Name + "-pod"
	pod := r.buildPod(podName, tr, &task)

	// Create Pod
	if err := r.Create(ctx, pod); err != nil && !apierrors.IsAlreadyExists(err) {
		logf.Error(err, "Failed to create Pod")
		return ctrl.Result{}, err
	}

	// Update TaskRun status to Pending
	logf.Info("Pod created successfully", "pod", podName)
	tr.Status.Phase = "Pending"
	tr.Status.PodName = podName
	now := metav1.Now()
	tr.Status.StartTime = &now

	if err := r.Status().Update(ctx, tr); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Handle active taskrun

func (r *TaskRunReconciler) handleActiveTaskRun(ctx context.Context, tr *miniv1.TaskRun) (ctrl.Result, error) {
	// log := ctrl.Log.WithName("handleActive")

	// Find corresponding Pod
	var pod corev1.Pod
	podKey := types.NamespacedName{Name: tr.Status.PodName, Namespace: tr.Namespace}
	err := r.Get(ctx, podKey, &pod)

	// If pod not found
	if err != nil {
		if apierrors.IsNotFound(err) {
			logf.Info("Pod missing! Marking TaskRun as Failed.")
			tr.Status.Phase = "Failed"
			now := metav1.Now()
			tr.Status.FinishTime = &now
			_ = r.Status().Update(ctx, tr)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update TaskRun according to the Pod status
	oldPhase := tr.Status.Phase
	newPhase := oldPhase

	switch pod.Status.Phase {
	case corev1.PodPending:
		newPhase = "Pending"
	case corev1.PodRunning:
		newPhase = "Running"
	case corev1.PodSucceeded:
		newPhase = "Succeeded"
		now := metav1.Now()
		tr.Status.FinishTime = &now
	case corev1.PodFailed:
		newPhase = "Failed"
		now := metav1.Now()
		tr.Status.FinishTime = &now
	}

	// If Status changed then save it to API Server
	if oldPhase != newPhase {
		logf.Info("Phase Transition", "From", oldPhase, "To", newPhase)
		tr.Status.Phase = newPhase
		if err := r.Status().Update(ctx, tr); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// Pod Builder

func (r *TaskRunReconciler) buildPod(podName string, tr *miniv1.TaskRun, task *miniv1.Task) *corev1.Pod {

	// Converting steps (of Task) to containers
	var containers []corev1.Container
	for _, step := range task.Spec.Steps {
		containers = append(containers, corev1.Container{
			Name:    step.Name,
			Image:   step.Image,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{step.Script},
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: tr.Namespace,
			Labels: map[string]string{
				"minitask": tr.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    containers,
		},
	}

	// Making Pod as child of TaskRun ( adv1. If tr gets deleted then pod also gets deleted,
	// adv2. If status of pod changes then reconcile loop automatically starts executing)

	_ = ctrl.SetControllerReference(tr, pod, r.Scheme)

	return pod
}
