package main

import (
	"context"
	"fmt"
	"os"

	miniclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
	miniInformers "github.com/ankrsinha/mini-task/pkg/generated/informers/externalversions"
	minilisterv1 "github.com/ankrsinha/mini-task/pkg/generated/listers/minitask/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	ctx context.Context

	miniClient *miniclient.Clientset
	coreClient *kubernetes.Clientset

	trInformer  cache.SharedIndexInformer
	podInformer cache.SharedIndexInformer

	trLister  minilisterv1.TaskRunLister
	podLister corelistersv1.PodLister

	queue workqueue.TypedRateLimitingInterface[cache.ObjectName]
}

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

	// creating informers factory
	// miniFactory: for our custom resources (TaskRun, Task)
	// kubeFactory: for core K8s resources (Pod)
	miniFactory := miniInformers.NewSharedInformerFactory(miniClient, 0)
	coreFactory := informers.NewSharedInformerFactory(coreClient, 0)

	// creating custom controller

	controller := &Controller{
		ctx:         ctx,
		miniClient:  miniClient,
		coreClient:  coreClient,
		trInformer:  miniFactory.Minitask().V1().TaskRuns().Informer(),
		podInformer: coreFactory.Core().V1().Pods().Informer(),
		trLister:    miniFactory.Minitask().V1().TaskRuns().Lister(),
		podLister:   coreFactory.Core().V1().Pods().Lister(),
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[cache.ObjectName](),
		),
	}

	// attaching event handlers to the informers

	controller.trInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueTaskRun,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueTaskRun(new)
		},
		DeleteFunc: controller.enqueueTaskRun,
	})

	controller.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.handlePodUpdate,
		DeleteFunc: controller.handlePodDelete,
	})

	// start informers
	stopCh := make(chan struct{})

	go controller.trInformer.Run(stopCh)
	go controller.podInformer.Run(stopCh)

	// wait for cache sync
	if !cache.WaitForCacheSync(stopCh,
		controller.trInformer.HasSynced,
		controller.podInformer.HasSynced,
	) {
		fmt.Println("Failed to sync caches")
		os.Exit(1)
	}

	// start worker
	go controller.runWorker()

	// block forever
	select {}

}

func (c *Controller) enqueueTaskRun(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return
	}

	objName := cache.ObjectName{
		Namespace: namespace,
		Name:      name,
	}

	c.queue.Add(objName)

}

func (c *Controller) handlePodUpdate(oldObj, newObj interface{}) {
	pod := newObj.(*corev1.Pod)

	trName := pod.Labels["minitask"]
	if trName == "" {
		return
	}

	key := pod.Namespace + "/" + trName
	fmt.Println("Pod updated, enqueue TaskRun:", key)

	objName := cache.ObjectName{
		Namespace: pod.Namespace,
		Name:      trName,
	}

	c.queue.Add(objName)

}

func (c *Controller) handlePodDelete(obj interface{}) {

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	trName := pod.Labels["minitask"]
	if trName == "" {
		return
	}

	fmt.Println("Pod deleted, enqueue TaskRun:", pod.Namespace+"/"+trName)

	objName := cache.ObjectName{
		Namespace: pod.Namespace,
		Name:      trName,
	}

	c.queue.Add(objName)
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {

	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Done(key)

	err := c.reconcile(key)
	if err != nil {
		fmt.Println("Error reconciling:", err)
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	return true
}

func (c *Controller) reconcile(key cache.ObjectName) error {

	namespace := key.Namespace
	name := key.Name

	fmt.Println("Reconciling:", namespace+"/"+name)

	//////////////////////////////

	return nil
}
