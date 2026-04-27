package taskrun

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
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

	taskRunGVK := schema.GroupVersionKind{
		Group:   miniv1.SchemeGroupVersion.Group,
		Version: miniv1.SchemeGroupVersion.Version,
		Kind:    "TaskRun",
	}
	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGVK(taskRunGVK),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	return impl
}