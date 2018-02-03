package v3public

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuthProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type string `json:"type"`
}

type GithubProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`
}

type LocalProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`
}

type GenericLogin struct {
	TTLMillis    int    `json:"ttl,omitempty"`
	Description  string `json:"description,omitempty" norman:"type=string,required"`
	ResponseType string `json:"responseType,omitempty" norman:"type=string,required"` //json or cookie
}

type GithubLogin struct {
	GenericLogin `json:",inline"`
	Code         string `json:"code" norman:"type=string,required"`
}

type LocalLogin struct {
	GenericLogin `json:",inline"`
	Username     string `json:"username" norman:"type=string,required"`
	Password     string `json:"password" norman:"type=string,required"`
}
