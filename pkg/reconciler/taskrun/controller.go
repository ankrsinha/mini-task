package taskrun

import (
	"context"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"

	minitaskclient "github.com/ankrsinha/mini-task/pkg/generated/clientset/versioned"
	minitaskinformers "github.com/ankrsinha/mini-task/pkg/generated/informers/externalversions"
)

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	kubeClient := kubeclient.Get(ctx)
	podInformer := podinformer.Get(ctx)

	cfg := injection.GetConfig(ctx)
	customClient := minitaskclient.NewForConfigOrDie(cfg)
	factory := minitaskinformers.NewSharedInformerFactory(customClient, controller.GetResyncPeriod(ctx))

	taskRunInformer := factory.Minitask().V1().TaskRuns()
	taskInformer := factory.Minitask().V1().Tasks()

	r := &Reconciler{
		kubeClient:    kubeClient,
		customClient:  customClient,
		taskRunLister: taskRunInformer.Lister(),
		taskLister:    taskInformer.Lister(),
		podLister:     podInformer.Lister(),
	}

	impl := controller.NewContext(ctx, r, controller.ControllerOptions{
		WorkQueueName: "TaskRuns",
		Logger:        logger.Named("taskrun"),
	})

	logger.Info("Setting up event handlers")

	taskRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	podInformer.Informer().AddEventHandler(controller.HandleAll(impl.EnqueueControllerOf))

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	return impl
}