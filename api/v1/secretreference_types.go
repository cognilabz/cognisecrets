package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AuthorizationAnnotation = "cognisecrets.cognilabz.com/allowed-namespaces"
	ManagedByLabel          = "app.kubernetes.io/managed-by"
	ManagedByValue          = "cognisecrets"

	// ReadyConditionType is the only condition type owned by CogniSecrets.
	ReadyConditionType = "Ready"

	ReasonSynced                = "Synced"
	ReasonSourceNotFound        = "SourceNotFound"
	ReasonAccessDenied          = "AccessDenied"
	ReasonSourceKeyNotFound     = "SourceKeyNotFound"
	ReasonDuplicateTargetKey    = "DuplicateTargetKey"
	ReasonTargetAlreadyExists   = "TargetAlreadyExists"
	ReasonManagedSourceRejected = "ManagedSourceRejected"
	ReasonTargetRejected        = "TargetRejected"
	ReasonWriteFailed           = "WriteFailed"
)

// SecretReferenceSpec defines the desired state of SecretReference.
type SecretReferenceSpec struct {
	// Type is copied to the generated Secret type field.
	// +kubebuilder:default=Opaque
	// +optional
	Type corev1.SecretType `json:"type,omitempty"`

	// Sources defines the source Secrets used to compose the target Secret.
	// +kubebuilder:validation:MinItems=1
	Sources []SecretSource `json:"sources"`
}

// SecretSource identifies one source Secret and optional key mappings.
type SecretSource struct {
	// Namespace is the namespace of the source Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace"`

	// Name is the name of the source Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Keys selects and optionally renames keys from the source Secret.
	// If omitted, all source data keys are copied.
	// +kubebuilder:validation:MinItems=1
	// +optional
	Keys []SecretKeyMapping `json:"keys,omitempty"`
}

// SecretKeyMapping maps one source Secret data key to one target Secret data key.
type SecretKeyMapping struct {
	// Name is the key name in the source Secret data map.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Target is the key name in the generated target Secret.
	// If omitted, the controller resolves it to Name during composition.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Target string `json:"target,omitempty"`
}

// SecretReferenceStatus defines the observed state of SecretReference.
type SecretReferenceStatus struct {
	// Conditions contains the Ready condition owned by CogniSecrets.
	// Other condition types are preserved when present.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sr
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SecretReference describes one target Secret composed from one or more source Secrets.
type SecretReference struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   SecretReferenceSpec   `json:"spec"`
	Status SecretReferenceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretReferenceList contains a list of SecretReference resources.
type SecretReferenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretReference `json:"items"`
}
