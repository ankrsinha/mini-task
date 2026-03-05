package main

import (
	"context"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	miniv1 "github.com/ankrsinha/mini-task/pkg/apis/minitask/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TaskRunReconciler struct {
	client.Client
	scheme *runtime.Scheme
}

var logf = ctrl.Log.WithName("mini-task-controller")

func main() {

	// Initializing logger
	ctrl.SetLogger(zap.New())
	logf.Info("Logger Initialized")

	// Creating runtime scheme
	scheme := runtime.NewScheme()

	// Registering core Kubernetes APIs
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		logf.Error(err, "Unable to register Kubernetes scheme")
		os.Exit(1)
	}

	// Registering custom resource APIs
	err = miniv1.AddToScheme(scheme)
	if err != nil {
		logf.Error(err, "Unable to register mini-task scheme")
		os.Exit(1)
	}

	logf.Info("Scheme Registered")

	// Creating New Manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		logf.Error(err, "Unable to create manager")
		os.Exit(1)
	}

	logf.Info("Manager created successfully")

	// Creating New controller and attaching it to the manager
	err = ctrl.NewControllerManagedBy(mgr).
		For(&miniv1.TaskRun{}).
		Owns(&corev1.Pod{}).
		Complete(&TaskRunReconciler{
			Client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
		})

	if err != nil {
		os.Exit(1)
	}

	// Starting manager
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		logf.Error(err, "Problem running manager")
		os.Exit(1)
	}

	logf.Info("Manager Started Successfully")
}

func (r *TaskRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
