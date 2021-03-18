package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type APIService struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIServiceSpec   `json:"spec"`
	Status APIServiceStatus `json:"status,omitempty"`
}

type APIServiceSpec struct {
	PathPrefixes []string `json:"pathPrefixes,omitempty"`
	Paths        []string `json:"paths,omitempty"`

	// SecretName refers to a secret that will be created that can be read by a local aggregation client
	SecretName      string `json:"secretName,omitempty"`
	SecretNamespace string `json:"secretNamespace,omitempty"`
}

type APIServiceStatus struct {
	ServiceAccountName      string `json:"serviceAccountName,omitempty"`
	ServiceAccountNamespace string `json:"serviceAccountNamespace,omitempty"`
}
