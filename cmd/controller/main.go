package main

import (
	"github.com/ankrsinha/mini-task/pkg/reconciler/taskrun"

	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	sharedmain.Main("mini-task-controller",
		taskrun.NewController,
	)
}