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

type AstroStarType string

const (
	AstroStarDocker AstroStarType = "docker"
)

type AstroStar struct {
	Name string        `json:"name"`
	Type AstroStarType `json:"type"`
	// +optional
	Dependencies []string `json:"dependencies,omitempty"`

	// docker type configuration

	// +optional
	Action string `json:"action,omitempty"`
	// +optional
	Target string `json:"target,omitempty"`
	// +optional
	Image string `json:"image,omitempty"`
	// +optional
	Port int32 `json:"port,omitempty"`
}

// AstroSpec is the spec for a Astro resource
type AstroSpec struct {
	Stars []AstroStar `json:"stars,omitempty"`
}

type AstroConditionType string

const (
	AstroConditionInitializing AstroConditionType = "initializing"
	AstroConditionRunning      AstroConditionType = "running"
	AstroConditionFailed       AstroConditionType = "failed"
	AstroConditionSucceeded    AstroConditionType = "succeeded"
)

// Condition defines an observation of a Cluster API resource operational state.
type AstroCondition struct {
	// Workflow status
	Type AstroConditionType `json:"type"`

	// Last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed. If that is not known, then using the time when
	// the API field changed is acceptable.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// AstroStatus is the status for a Astro resource
type AstroStatus struct {
	// +optional
	Conditions []AstroCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AstroList is a list of Astro resources
type AstroList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Astro `json:"items"`
}
