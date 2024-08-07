package tokens

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RancherToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RancherTokenSpec   `json:"spec"`
	Status RancherTokenStatus `json:"status"`
}

func (r *RancherToken) Default() any {
	return &RancherToken{}
}

type RancherTokenSpec struct {
	UserID      string `json:"userID"`
	ClusterName string `json:"clusterName"`
	TTL         string `json:"ttl"`
	Enabled     string `json:"enabled"`
}

type RancherTokenStatus struct {
	PlaintextToken string `json:"plaintextToken,omitempty"`
	HashedToken    string `json:"hashedToken"`
}
