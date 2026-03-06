package main

import (
	"context"
	"fmt"
	"os"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: kubectl task start <taskName>")
		os.Exit(1)
	}

	command := os.Args[1]
	taskName := os.Args[2]

	if command != "start" {
		fmt.Println("Invalid command. Supported: start")
		os.Exit(1)
	}

	// Create runtime scheme
	scheme := runtime.NewScheme()

	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		fmt.Println("Error adding core scheme:", err)
		os.Exit(1)
	}

	err = miniv1.AddToScheme(scheme)
	if err != nil {
		fmt.Println("Error adding minitask scheme:", err)
		os.Exit(1)
	}

	// Load kubeconfig
	config, err := ctrl.GetConfig()
	if err != nil {
		fmt.Println("Error loading kubeconfig:", err)
		os.Exit(1)
	}

	// Create controller-runtime client
	k8sClient, err := ctrlclient.New(config, ctrlclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		fmt.Println("Error creating client:", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Create TaskRun
	taskRun := &miniv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: taskName + "-run-",
			Namespace:    "default",
		},
		Spec: miniv1.TaskRunSpec{
			TaskRef: taskName,
		},
	}

	err = k8sClient.Create(ctx, taskRun)

	if err != nil {
		fmt.Println("Error creating TaskRun:", err)
		os.Exit(1)
	}

	fmt.Printf("TaskRun %s created successfully\n", taskRun.Name)
}
