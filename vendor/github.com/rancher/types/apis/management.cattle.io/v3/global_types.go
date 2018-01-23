package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Setting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Value     string `json:"value" norman:"required"`
	HideValue bool   `json:"hideValue" norman:"noupdate"`
	ReadOnly  bool   `json:"readOnly" norman:"nocreate,noupdate"`
}

type ListenConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName string   `json:"displayName,omitempty"`
	Description string   `json:"description,omitempty"`
	Mode        string   `json:"mode,omitempty" norman:"type=enum,options=https|http|acme"`
	CACerts     string   `json:"caCerts,omitempty"`
	Cert        string   `json:"cert,omitempty"`
	Key         string   `json:"key,omitempty" norman:"writeOnly"`
	Domains     []string `json:"domains,omitempty"`
	TOS         []string `json:"tos,omitempty" norman:"default=auto"`
	Enabled     bool     `json:"enabled,omitempty" norman:"default=true"`

	CertFingerprint         string   `json:"certFingerprint,omitempty" norman:"nocreate,noupdate"`
	CN                      string   `json:"cn,omitempty" norman:"nocreate,noupdate"`
	Version                 int      `json:"version,omitempty" norman:"nocreate,noupdate"`
	ExpiresAt               string   `json:"expiresAt,omitempty" norman:"nocreate,noupdate"`
	Issuer                  string   `json:"issuer,omitempty" norman:"nocreate,noupdate"`
	IssuedAt                string   `json:"issuedAt,omitempty" norman:"nocreate,noupdate"`
	Algorithm               string   `json:"algorithm,omitempty" norman:"nocreate,noupdate"`
	SerialNumber            string   `json:"serialNumber,omitempty" norman:"nocreate,noupdate"`
	KeySize                 int      `json:"keySize,omitempty" norman:"nocreate,noupdate"`
	SubjectAlternativeNames []string `json:"subjectAlternativeNames,omitempty" norman:"nocreate,noupdate"`
}
