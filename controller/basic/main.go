package main

import (
	"context"
	"fmt"
	"os"
	"time"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	miniclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := context.Background()

	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := clientcmd.RecommendedHomeFile
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			fmt.Println("Error building kubeconfig:", err)
			os.Exit(1)
		}
	}

	// create generated client
	miniClient, err := miniclient.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating mini client:", err)
		os.Exit(1)
	}

	// core Kubernetes client (for Pods)
	coreClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating core client:", err)
		os.Exit(1)
	}

	// infinite loop
	for {
		// get all taskrun
		taskRuns, err := miniClient.
			MinitaskV1().
			TaskRuns("default").
			List(ctx, metav1.ListOptions{})

		if err != nil {
			fmt.Println("Error listing TaskRuns:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// check/reconcile each taskrun
		for _, tr := range taskRuns.Items {
			fmt.Println("--------------------------------------------------")
			fmt.Println("Reconciling: ", tr.Name)
			fmt.Println("Current Phase: ", tr.Status.Phase)
			// 	// reconcile logic-
			// 	// If no Pod → create Pod
			// 	// If Pod finished → update status

			// Check current phase
			// Check actual Pod phase
			// Decide if transition needed
			// Update status

			switch tr.Status.Phase {

			case "":
				handleNewTaskRun(ctx, miniClient, coreClient, &tr)

			case "Pending", "Running":
				handleActiveTaskRun(ctx, miniClient, coreClient, &tr)

			case "Succeeded", "Failed":
				fmt.Println("TaskRun already completed. Skipping.")

			default:
				fmt.Println("Unknown Phase:", tr.Status.Phase)
			}

		}

		// sleep for 5s
		time.Sleep(5 * time.Second)
	}
}

func handleNewTaskRun(ctx context.Context, miniClient *miniclient.Clientset, coreClient *kubernetes.Clientset, tr *miniv1.TaskRun) {

	// create pod

	podName := tr.Name + "-pod"

	// Check if pod already exists
	fmt.Println("Checking if Pod exists...")
	_, err := coreClient.CoreV1().
		Pods("default").
		Get(ctx, podName, metav1.GetOptions{})

	if err == nil {
		fmt.Println("Pod already exists. Skipping creation.")
		return
	}

	if !apierrors.IsNotFound(err) {
		fmt.Println("Error checking Pod existence:", err)
		return
	}

	fmt.Println("Creating Pod:", podName)

	task, err := miniClient.
		MinitaskV1().
		Tasks("default").
		Get(ctx, tr.Spec.TaskRef, metav1.GetOptions{})

	if err != nil {
		fmt.Println("Error fetching Task:", err)
		return
	}

	containers := []corev1.Container{}

	for _, step := range task.Spec.Steps {
		container := corev1.Container{
			Name:    step.Name,
			Image:   step.Image,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{step.Script},
		}
		containers = append(containers, container)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
			Labels: map[string]string{
				"minitask": tr.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    containers,
		},
	}

	_, err = coreClient.CoreV1().
		Pods("default").
		Create(ctx, pod, metav1.CreateOptions{})

	if err != nil {
		fmt.Println("Error creating Pod:", err)
		return
	}

	fmt.Println("Pod Created: ", podName)

	// update status -> pending

	trCopy := tr.DeepCopy()
	trCopy.Status.Phase = "Pending"
	trCopy.Status.PodName = podName

	_, err = miniClient.
		MinitaskV1().
		TaskRuns("default").
		UpdateStatus(ctx, trCopy, metav1.UpdateOptions{})

	if err != nil {
		fmt.Println("Error updating TaskRun status:", err)
	}

	fmt.Println("Status updated -> Pending")
}

func handleActiveTaskRun(ctx context.Context, miniClient *miniclient.Clientset, coreClient *kubernetes.Clientset, tr *miniv1.TaskRun) {
	podName := tr.Status.PodName

	fmt.Println("Checking Pod status...")
	pod, err := coreClient.CoreV1().
		Pods("default").
		Get(ctx, podName, metav1.GetOptions{})

	// if err != nil {
	// 	fmt.Println("Error fetching Pod:", err)
	// 	break
	// }

	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Println("Pod missing. Marking TaskRun as Failed.")

			trCopy := tr.DeepCopy()
			trCopy.Status.Phase = "Failed"
			now := metav1.Now()
			trCopy.Status.FinishTime = &now

			miniClient.
				MinitaskV1().
				TaskRuns("default").
				UpdateStatus(ctx, trCopy, metav1.UpdateOptions{})

			return
		}

		fmt.Println("Error fetching Pod:", err)
		return
	}

	fmt.Println("Pod Phase:", pod.Status.Phase)

	oldPhase := tr.Status.Phase
	newPhase := oldPhase

	trCopy := tr.DeepCopy()

	switch pod.Status.Phase {

	case corev1.PodPending:
		newPhase = "Pending"

	case corev1.PodRunning:
		newPhase = "Running"
		if trCopy.Status.StartTime == nil {
			now := metav1.Now()
			trCopy.Status.StartTime = &now
			fmt.Println("Start time set")
		}

	case corev1.PodSucceeded:
		newPhase = "Succeeded"
		now := metav1.Now()
		trCopy.Status.FinishTime = &now
		fmt.Println("Finished Successfully!!")

	case corev1.PodFailed:
		newPhase = "Failed"
		now := metav1.Now()
		trCopy.Status.FinishTime = &now
		fmt.Println("Failed!!")
	}

	if oldPhase != newPhase {
		fmt.Printf("Phase Transition %s -> %s\n", oldPhase, newPhase)
		trCopy.Status.Phase = newPhase

		_, err = miniClient.
			MinitaskV1().
			TaskRuns("default").
			UpdateStatus(ctx, trCopy, metav1.UpdateOptions{})

		if err != nil {
			fmt.Println("Error updating TaskRun status:", err)
		}

		fmt.Println("Status updated")

	} else {
		fmt.Println("No Phase Change!")
	}
}
