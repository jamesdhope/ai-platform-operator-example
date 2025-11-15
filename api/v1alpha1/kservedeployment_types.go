package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KServeDeploymentSpec defines the desired state of KServe deployment
type KServeDeploymentSpec struct {
	// Version of KServe to deploy
	Version string `json:"version"`

	// Components to deploy (kserve, knative, istio, cert-manager)
	Components []string `json:"components,omitempty"`

	// Namespace where KServe will be installed
	// +kubebuilder:default=kserve
	Namespace string `json:"namespace,omitempty"`

	// Configuration for KServe components
	Config *KServeConfig `json:"config,omitempty"`
}

// KServeConfig defines configuration options for KServe
type KServeConfig struct {
	// IngressDomain for KServe endpoints
	IngressDomain string `json:"ingressDomain,omitempty"`

	// EnableIstio for service mesh integration
	EnableIstio bool `json:"enableIstio,omitempty"`

	// EnableKnative for serverless serving
	EnableKnative bool `json:"enableKnative,omitempty"`
}

// KServeDeploymentStatus defines the observed state of KServe deployment
type KServeDeploymentStatus struct {
	// Phase of the deployment (Pending, Installing, Ready, Failed)
	// +kubebuilder:validation:Enum=Pending;Installing;Ready;Failed
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// InstalledVersion is the currently installed version
	InstalledVersion string `json:"installedVersion,omitempty"`

	// InstalledComponents lists the successfully installed components
	InstalledComponents []string `json:"installedComponents,omitempty"`

	// LastUpdated timestamp
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ksd
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// KServeDeployment is the Schema for deploying KServe
type KServeDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KServeDeploymentSpec   `json:"spec,omitempty"`
	Status KServeDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KServeDeploymentList contains a list of KServeDeployment
type KServeDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KServeDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KServeDeployment{}, &KServeDeploymentList{})
}
