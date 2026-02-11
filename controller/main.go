package main

import (
	"context"
	"fmt"
	"os"
	"time"

	miniclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clientset, err := miniclient.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating mini client:", err)
		os.Exit(1)
	}

	// infinite loop
	for {
		// get all taskrun
		taskRuns, err := clientset.
			MinitaskV1().
			TaskRuns("default").
			List(ctx, metav1.ListOptions{})

		if err != nil {
			fmt.Println("Error listing TaskRuns:", err)
			os.Exit(1)
		}

		// fmt.Println(taskRuns.Items)

		// check/reconcile each taskrun
		for _, tr := range taskRuns.Items {
			fmt.Println("Reconciling: ", tr.Name)

			// 	// reconcile logic-
			// 	// If no Pod → create Pod
			// 	// If Pod finished → update status

			switch tr.Status.Phase {

			case "":
				fmt.Println("TaskRun", tr.Name, "is new → need execution (should create Pod)")
				// create pod

			case "Pending":
				fmt.Println("TaskRun", tr.Name, "Pod created, waiting to start")

			case "Running":
				fmt.Println("TaskRun", tr.Name, "is currently running")

			case "Succeeded":
				fmt.Println("TaskRun", tr.Name, "already completed successfully")

			case "Failed":
				fmt.Println("TaskRun", tr.Name, "already failed")

			default:
				fmt.Println("TaskRun", tr.Name, "has unknown phase:", tr.Status.Phase)
			}

		}

		// sleep for 5s
		time.Sleep(5 * time.Second)
	}
}
