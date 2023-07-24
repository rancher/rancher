package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKEBootstrap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RKEBootstrapSpec   `json:"spec"`
	Status            RKEBootstrapStatus `json:"status,omitempty"`
}

type Encoding string

const (
	// Base64 implies the contents of the file are encoded as base64.
	Base64 Encoding = "base64"
	// Gzip implies the contents of the file are encoded with gzip.
	Gzip Encoding = "gzip"
	// GzipBase64 implies the contents of the file are first base64 encoded and then gzip encoded.
	GzipBase64 Encoding = "gzip+base64"
)

type CloudInitFile struct {
	Encoding    Encoding `json:"encoding,omitempty"`
	Content     string   `json:"content,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	Path        string   `json:"path"`
	Permissions string   `json:"permissions,omitempty"`
}

type RKEBootstrapSpec struct {
	ClusterName string          `json:"clusterName,omitempty"`
	Files       []CloudInitFile `json:"files,omitempty"`
}

type RKEBootstrapStatus struct {
	// Ready indicates the BootstrapData field is ready to be consumed
	Ready bool `json:"ready,omitempty"`

	// DataSecretName is the name of the secret that stores the bootstrap data script.
	// +optional
	DataSecretName *string `json:"dataSecretName,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKEBootstrapTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RKEBootstrapTemplateSpec `json:"spec" wrangler:"required"`
}

type RKEBootstrapTemplateSpec struct {
	ClusterName string       `json:"clusterName,omitempty"`
	Template    RKEBootstrap `json:"template,omitempty" wrangler:"required"`
}
