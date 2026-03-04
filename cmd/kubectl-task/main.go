package main

import (
	"context"
	"fmt"
	"os"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	miniclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	if len(os.Args) < 3 {
		fmt.Println("Use: kubectl task start <taskName>")
		os.Exit(1)
	}

	command := os.Args[1]
	taskName := os.Args[2]

	if command != "start" {
		fmt.Println("Invalid Command")
		os.Exit(1)
	}

	// Build config from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		fmt.Println("Error building kubeconfig:", err)
		os.Exit(1)
	}

	// client for creating taskRun
	client, err := miniclient.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating client:", err)
		os.Exit(1)
	}

	ctx := context.Background()

	taskRun := &miniv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: taskName + "-" + "run" + "-",
			Namespace:    "default",
		},
		Spec: miniv1.TaskRunSpec{
			TaskRef: taskName,
		},
	}

	createdTr, err := client.MinitaskV1().TaskRuns("default").Create(ctx, taskRun, metav1.CreateOptions{})

	if err != nil {
		fmt.Println("Error creating TaskRun:", err)
		os.Exit(1)
	}

	fmt.Printf("TaskRun %v created successfully\n", createdTr.Name)
}
