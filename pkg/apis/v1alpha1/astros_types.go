package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=astros
// +kubebuilder:resource:shortName=astro
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Astro is a specification for a Astro resource
type Astro struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AstroSpec   `json:"spec,omitempty"`
	Status AstroStatus `json:"status,omitempty"`
}

// AstroSpec is the spec for a Astro resource
type AstroSpec struct {
}

// AstroStatus is the status for a Astro resource
type AstroStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AstroList is a list of Astro resources
type AstroList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Astro `json:"items"`
}
