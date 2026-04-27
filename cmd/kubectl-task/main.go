package main

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	minitaskclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
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

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		fmt.Println("Error loading kubeconfig:", err)
		os.Exit(1)
	}

	client, err := minitaskclient.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating client:", err)
		os.Exit(1)
	}

	taskRun := &miniv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: taskName + "-run-",
			Namespace:    "default",
		},
		Spec: miniv1.TaskRunSpec{
			TaskRef: taskName,
		},
	}

	result, err := client.MinitaskV1().TaskRuns("default").Create(
		context.Background(), taskRun, metav1.CreateOptions{},
	)
	if err != nil {
		fmt.Println("Error creating TaskRun:", err)
		os.Exit(1)
	}

	fmt.Printf("TaskRun %s created successfully\n", result.Name)
}
