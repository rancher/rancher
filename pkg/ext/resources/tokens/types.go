package tokens

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RancherToken
type RancherToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the spec of RancherToken
	Spec RancherTokenSpec `json:"spec"`
	// +optional
	Status RancherTokenStatus `json:"status"`
}

type RancherTokenSpec struct {
	// UserID is the user id
	UserID string `json:"userID"`
	// ClusterName is the cluster that the token is scoped to. If empty, the token
	// can be used for all clusters.
	// +optional
	ClusterName string `json:"clusterName"`
	// TTL is the time-to-live of the token.
	TTL     string `json:"ttl"`
	Enabled string `json:"enabled"`
}

type RancherTokenStatus struct {
	PlaintextToken string `json:"plaintextToken,omitempty"`
	HashedToken    string `json:"hashedToken"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RancherTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RancherToken `json:"items"`
}
