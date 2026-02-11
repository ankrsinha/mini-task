package v1

// task -> apiVersion, kind, metadata, spec
// TypeMeta -> apiVersion, kind
// ObjectMeta -> metadata(name, labels, namespace)
// spec -> list of steps
// step -> name, image, script
// taskList -> for getting list of all tasks

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Step struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Script string `json:"script"`
}

type TaskSpec struct {
	Steps []Step `json:"steps"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TaskSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}
