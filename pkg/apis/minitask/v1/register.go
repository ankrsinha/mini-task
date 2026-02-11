package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	minitask "github.com/ankrsinha/mini-task/pkg/apis/minitask"
)

// SchemeGroupVersion defines group + version
var SchemeGroupVersion = schema.GroupVersion{
	Group:   minitask.GroupName,
	Version: "v1",
}

// Kind returns a GroupKind for a given kind string
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource returns a GroupResource for a given resource string
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder registers types to scheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme exposes registration method
	AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes registers API types into scheme
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&Task{},
		&TaskList{},
		&TaskRun{},
		&TaskRunList{},
	)

	// Register version in scheme
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)

	return nil
}
