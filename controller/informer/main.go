package main

import (
	"context"
	"fmt"
	"os"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	miniclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
	miniInformers "github.com/ankrsinha/mini-task/pkg/generated/informers/externalversions"
	minilisterv1 "github.com/ankrsinha/mini-task/pkg/generated/listers/minitask/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Controller struct {
	ctx context.Context

	miniClient *miniclient.Clientset
	coreClient *kubernetes.Clientset

	trInformer   cache.SharedIndexInformer
	podInformer  cache.SharedIndexInformer
	taskInformer cache.SharedIndexInformer

	trLister   minilisterv1.TaskRunLister
	podLister  corelistersv1.PodLister
	taskLister minilisterv1.TaskLister

	queue workqueue.TypedRateLimitingInterface[cache.ObjectName]
}

func main() {
	// Provides shared execution context.
	ctx := context.Background()

	// Allows Running controller locally,
	// Allows controller to talk to cluster
	kubeconfig := clientcmd.RecommendedHomeFile
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Println("Error building kubeconfig:", err)
		os.Exit(1)
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

	// creating custom controller, which will act as central orchestrator
	controller := &Controller{
		ctx:          ctx,
		miniClient:   miniClient,
		coreClient:   coreClient,
		trInformer:   miniFactory.Minitask().V1().TaskRuns().Informer(),
		taskInformer: miniFactory.Minitask().V1().Tasks().Informer(),
		podInformer:  coreFactory.Core().V1().Pods().Informer(),
		trLister:     miniFactory.Minitask().V1().TaskRuns().Lister(),
		podLister:    coreFactory.Core().V1().Pods().Lister(),
		taskLister:   miniFactory.Minitask().V1().Tasks().Lister(),
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[cache.ObjectName](),
		),
	}

	// attaching event handlers to the informers

	controller.trInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueTaskRun,
		UpdateFunc: func(old, new interface{}) {
			oldTr := old.(*miniv1.TaskRun)
			newTr := new.(*miniv1.TaskRun)

			if oldTr.Status.Phase == newTr.Status.Phase {
				return
			}

			controller.enqueueTaskRun(new)
		},
		DeleteFunc: controller.enqueueTaskRun,
	})

	controller.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.handlePodUpdate,
		DeleteFunc: controller.handlePodDelete,
	})

	// start informers
	stopCh := make(chan struct{}) // channel used to stop exec (graceful shutdown)
	defer close(stopCh)

	go controller.trInformer.Run(stopCh)
	go controller.podInformer.Run(stopCh)
	go controller.taskInformer.Run(stopCh)

	// wait for initial cache sync
	if !cache.WaitForCacheSync(stopCh, controller.trInformer.HasSynced, controller.podInformer.HasSynced, controller.taskInformer.HasSynced) {
		fmt.Println("Failed to sync caches")
		os.Exit(1)
	}

	// start worker
	// go controller.runWorker()
	for i := 0; i < 2; i++ {
		go controller.runWorker()
	}

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
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	if oldPod.Status.Phase == newPod.Status.Phase {
		return
	}

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

	fmt.Println("--------------------------------------------------")
	fmt.Println("Reconciling:", name)

	tr, err := c.trLister.TaskRuns(namespace).Get(name)
	if err != nil {
		fmt.Println("TaskRun not found")
		return nil
	}

	fmt.Println("Current Phase:", tr.Status.Phase)

	switch tr.Status.Phase {

	case "":
		return c.handleNewTaskRun(tr)

	case "Pending", "Running":
		return c.handleActiveTaskRun(tr)

	case "Succeeded", "Failed":
		fmt.Println("TaskRun already completed. Skipping.")
		return nil
	}

	return nil
}

func (c *Controller) handleNewTaskRun(tr *miniv1.TaskRun) error {

	// create pod

	namespace := tr.Namespace
	podName := tr.Name + "-pod"

	fmt.Println("Creating Pod for:", tr.Name)

	_, err := c.podLister.Pods(namespace).Get(podName)

	if err == nil {
		fmt.Println("Pod already exists. Skipping creation.")
		return nil
	}

	if !apierrors.IsNotFound(err) {
		fmt.Println("Error checking Pod existence:", err)
		return err
	}

	task, err := c.taskLister.
		Tasks(namespace).
		Get(tr.Spec.TaskRef)

	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Println("Referenced Task not found")
			return nil
		}
		return err
	}

	var containers []corev1.Container

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
			Namespace: namespace,
			Labels: map[string]string{
				"minitask": tr.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(
					tr,
					miniv1.SchemeGroupVersion.WithKind("TaskRun"),
				),
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    containers,
		},
	}

	_, err = c.coreClient.CoreV1().Pods(namespace).Create(c.ctx, pod, metav1.CreateOptions{})
	if err != nil {
		// handle race condition
		if apierrors.IsAlreadyExists(err) {
			fmt.Println("Pod already exists (race). Skipping.")
			return nil
		}
		return err
	}

	fmt.Println("Pod created:", podName)

	// update status

	trCopy := tr.DeepCopy()
	trCopy.Status.Phase = "Pending"
	trCopy.Status.PodName = podName

	_, err = c.miniClient.MinitaskV1().TaskRuns(namespace).UpdateStatus(c.ctx, trCopy, metav1.UpdateOptions{})

	return err
}

func (c *Controller) handleActiveTaskRun(tr *miniv1.TaskRun) error {

	namespace := tr.Namespace
	podName := tr.Status.PodName

	fmt.Println("Checking Pod status...")

	pod, err := c.podLister.Pods(namespace).Get(podName)
	if err != nil {

		if apierrors.IsNotFound(err) {
			fmt.Println("Pod missing. Marking TaskRun as Failed.")

			trCopy := tr.DeepCopy()
			trCopy.Status.Phase = "Failed"
			now := metav1.Now()
			trCopy.Status.FinishTime = &now

			_, updateErr := c.miniClient.MinitaskV1().TaskRuns(namespace).UpdateStatus(c.ctx, trCopy, metav1.UpdateOptions{})

			return updateErr
		}

		fmt.Println("Error fetching Pod:", err)
		return err
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
		}

	case corev1.PodSucceeded:
		newPhase = "Succeeded"
		now := metav1.Now()
		trCopy.Status.FinishTime = &now

	case corev1.PodFailed:
		newPhase = "Failed"
		now := metav1.Now()
		trCopy.Status.FinishTime = &now
	}

	if oldPhase != newPhase {

		fmt.Printf("Phase Transition %s -> %s\n", oldPhase, newPhase)

		trCopy.Status.Phase = newPhase

		_, err := c.miniClient.MinitaskV1().TaskRuns(namespace).UpdateStatus(c.ctx, trCopy, metav1.UpdateOptions{})

		if err != nil {
			return err
		}

		fmt.Println("Status updated")
		return nil
	}

	fmt.Println("No Phase Change!")
	return nil
}
