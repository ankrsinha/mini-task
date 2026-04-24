package v1

// taskrun -> apiVersion, kind, metadata, spec, status
// TypeMeta -> apiVersion, kind
// ObjectMeta -> metadata(name, labels, namespace)
// spec -> taskRef
// status -> Phase, PodName, StartTime, FinishTime
// taskrunList -> for getting list of all taskruns

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TaskRunSpec struct {
	TaskRef string `json:"taskRef"`
}

type TaskRunStatus struct {
	Phase      string       `json:"phase,omitempty"`
	PodName    string       `json:"podName,omitempty"`
	StartTime  *metav1.Time `json:"startTime,omitempty"`
	FinishTime *metav1.Time `json:"finishTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TaskRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TaskRunSpec   `json:"spec,omitempty"`
	Status            TaskRunStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TaskRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TaskRun `json:"items"`
}
