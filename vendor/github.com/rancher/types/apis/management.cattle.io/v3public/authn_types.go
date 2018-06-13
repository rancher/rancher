package v3public

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuthProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type string `json:"type"`
}

type GenericLogin struct {
	TTLMillis    int64  `json:"ttl,omitempty"`
	Description  string `json:"description,omitempty" norman:"type=string,required"`
	ResponseType string `json:"responseType,omitempty" norman:"type=string,required"` //json or cookie
}

type BasicLogin struct {
	GenericLogin `json:",inline"`
	Username     string `json:"username" norman:"type=string,required"`
	Password     string `json:"password" norman:"type=string,required"`
}

type LocalProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`
}

type GithubProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
}

type GithubLogin struct {
	GenericLogin `json:",inline"`
	Code         string `json:"code" norman:"type=string,required"`
}

type ActiveDirectoryProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	DefaultLoginDomain string `json:"defaultLoginDomain,omitempty"`
}

type AzureADProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
}

type AzureADLogin struct {
	GenericLogin `json:",inline"`
	Code         string `json:"code" norman:"type=string,required"`
}
