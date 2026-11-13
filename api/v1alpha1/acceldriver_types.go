/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AccelDriverSpec defines the desired state of AccelDriver
type AccelDriverSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html
	// Desired driver version, e.g. "1.2.0".
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`

	// Nodes this driver applies to.
	// +kubebuilder:validation:Required
	NodeSelector *metav1.LabelSelector `json:"nodeSelector"`
}

// AccelDriverStatus defines the observed state of AccelDriver.
// DriverGroupStatus — сводка по одной OS-группе (одному DaemonSet'у).
type DriverGroupStatus struct {
	// Group key, e.g. "ubuntu-24.04".
	Name string `json:"name"`
	// Managed DaemonSet name.
	DaemonSet string `json:"daemonSet"`
	// Nodes we selected for this group.
	SelectedNodes int32 `json:"selectedNodes"`
	// Pods the DaemonSet controller decided to run (excludes unschedulable nodes).
	DesiredPods int32 `json:"desiredPods"`
	// Pods passing their readiness probe.
	ReadyPods int32 `json:"readyPods"`
	// Pods running the current template.
	UpdatedPods int32 `json:"updatedPods"`
}

type AccelDriverStatus struct {
	// Generation of the spec this status was computed from.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	SelectedNodes int32 `json:"selectedNodes"`
	// +optional
	DesiredPods int32 `json:"desiredPods"`
	// +optional
	ReadyPods int32 `json:"readyPods"`

	// +optional
	Groups []DriverGroupStatus `json:"groups,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.selectedNodes`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredPods`
// +kubebuilder:printcolumn:name="Ready-Pods",type=integer,JSONPath=`.status.readyPods`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AccelDriver is the Schema for the acceldrivers API
type AccelDriver struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of AccelDriver
	// +required
	Spec AccelDriverSpec `json:"spec"`

	// status defines the observed state of AccelDriver
	// +optional
	Status AccelDriverStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AccelDriverList contains a list of AccelDriver
type AccelDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AccelDriver `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &AccelDriver{}, &AccelDriverList{})
		return nil
	})
}
