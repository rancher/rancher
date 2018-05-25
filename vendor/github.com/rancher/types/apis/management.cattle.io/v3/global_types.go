package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Setting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Value      string `json:"value" norman:"required"`
	Default    string `json:"default" norman:"nocreate,noupdate"`
	Customized bool   `json:"customized" norman:"nocreate,noupdate"`
}

type ListenConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName    string            `json:"displayName,omitempty"`
	Description    string            `json:"description,omitempty"`
	Mode           string            `json:"mode,omitempty" norman:"type=enum,options=https|http|acme"`
	CACerts        string            `json:"caCerts,omitempty"`
	CACert         string            `json:"caCert,omitempty"`
	CAKey          string            `json:"caKey,omitempty"`
	Cert           string            `json:"cert,omitempty"`
	Key            string            `json:"key,omitempty" norman:"writeOnly"`
	Domains        []string          `json:"domains,omitempty"`
	TOS            []string          `json:"tos,omitempty" norman:"default=auto"`
	KnownIPs       []string          `json:"knownIps" norman:"nocreate,noupdate"`
	GeneratedCerts map[string]string `json:"generatedCerts" norman:"nocreate,noupdate"`
	Enabled        bool              `json:"enabled,omitempty" norman:"default=true"`

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

type CattleInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Token             string `json:"token,omitempty" norman:"nocreate,noupdate"`
	Identity          string `json:"identity,omitempty" norman:"nocreate,noupdate"`
}
