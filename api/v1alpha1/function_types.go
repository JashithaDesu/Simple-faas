package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FunctionSpec defines the desired state of a Function.
type FunctionSpec struct {
	// Image is the container image running the function code.
	Image string `json:"image"`

	// Command overrides the container entrypoint. Optional.
	// +optional
	Command []string `json:"command,omitempty"`

	// Port is the container port the function listens on.
	Port int32 `json:"port"`

	// IdleTimeoutSeconds is how long a function may sit idle
	// (no requests) before the controller scales it to zero.
	// +kubebuilder:default=300
	IdleTimeoutSeconds int32 `json:"idleTimeoutSeconds,omitempty"`

	// MinReplicas is the floor the controller will not scale below.
	// Set to 0 to allow true scale-to-zero (the default for this project).
	// +kubebuilder:default=0
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas caps how far the controller will scale up.
	// +kubebuilder:default=3
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
}

// FunctionPhase describes the current lifecycle phase of a Function.
type FunctionPhase string

const (
	PhaseCold    FunctionPhase = "Cold"    // 0 replicas running
	PhaseScaling FunctionPhase = "Scaling" // pod(s) starting, not yet ready
	PhaseWarm    FunctionPhase = "Warm"    // at least one replica ready
)

// FunctionStatus defines the observed state of a Function.
// This is what your controller writes back — NOT raw pod status,
// which Kubernetes already tracks in etcd via the API server.
type FunctionStatus struct {
	// Phase is the current lifecycle state.
	Phase FunctionPhase `json:"phase,omitempty"`

	// Replicas is the current replica count of the backing Deployment.
	Replicas int32 `json:"replicas"`

	// LastRequestTime is updated by the gateway on every invocation.
	// The controller reads this to decide when to scale to zero.
	// +optional
	LastRequestTime *metav1.Time `json:"lastRequestTime,omitempty"`

	// ObservedGeneration lets you tell whether status reflects the
	// most recent spec change (standard Kubernetes status pattern).
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`

// Function is the Schema for the functions API.
type Function struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FunctionSpec   `json:"spec,omitempty"`
	Status FunctionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FunctionList contains a list of Function.
type FunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Function `json:"items"`
}
