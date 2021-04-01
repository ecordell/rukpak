package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bundle is a specification for a Bundle resource
type Bundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BundleSpec   `json:"spec"`
	Status BundleStatus `json:"status"`
}

// BundleSpec is the spec for a Bundle resource
type BundleSpec struct {
	Class string `json:"class"`
	Refs []string `json:"refs"`
	VolumeMounts []v1.VolumeMount `json:"volumeMounts"`
}

// BundleStatus is the status for a Bundle resource
type BundleStatus struct {
	Phase string `json:"phase"`
	ObservedGeneration int64
	Conditions []metav1.Condition `json:"conditions"`
	Objects []v1.ObjectReference `json:"objects"`
}

// BundleObjectReference points to an object in cluster that contains unpacked bundle content
type BundleObjectReference struct {
	Group string `json:"group"`
	Kind string `json:"kind"`
	Namespace string `json:"namespace"`
	Name string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BundleList is a list of Bundle resources
type BundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Bundle `json:"items"`
}


