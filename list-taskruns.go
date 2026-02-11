package main

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	miniclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
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

	// get all TaskRuns
	taskRuns, err := clientset.
		MinitaskV1().
		TaskRuns("default").
		List(ctx, metav1.ListOptions{})

	if err != nil {
		fmt.Println("Error listing TaskRuns:", err)
		os.Exit(1)
	}

	fmt.Println("TaskRuns in default namespace:")
	for _, tr := range taskRuns.Items {
		fmt.Println("-", tr.Name)
	}
}
