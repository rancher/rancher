package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterTemplateRevisionConditionSecretsMigrated    condition.Cond = "SecretsMigrated"
	ClusterTemplateRevisionConditionACISecretsMigrated condition.Cond = "ACISecretsMigrated"
	ClusterTemplateRevisionConditionRKESecretsMigrated condition.Cond = "RKESecretsMigrated"
)

type ClusterTemplateRevisionConditionType string

type ClusterTemplateRevisionCondition struct {
	// Type of cluster template revision condition.
	Type ClusterTemplateRevisionConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterTemplate struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterTemplateSpec `json:"spec"`
}

type ClusterTemplateSpec struct {
	DisplayName         string `json:"displayName" norman:"required"`
	Description         string `json:"description"`
	DefaultRevisionName string `json:"defaultRevisionName,omitempty" norman:"type=reference[clusterTemplateRevision]"`

	Members []Member `json:"members,omitempty"`
}

type Member struct {
	UserName           string `json:"userName,omitempty" norman:"type=reference[user]"`
	UserPrincipalName  string `json:"userPrincipalName,omitempty" norman:"type=reference[principal]"`
	DisplayName        string `json:"displayName,omitempty"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"type=reference[principal]"`
	AccessType         string `json:"accessType,omitempty" norman:"type=enum,options=owner|member|read-only"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterTemplateRevision struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec   ClusterTemplateRevisionSpec   `json:"spec"`
	Status ClusterTemplateRevisionStatus `json:"status"`
}

type ClusterTemplateRevisionSpec struct {
	DisplayName         string `json:"displayName" norman:"required"`
	Enabled             *bool  `json:"enabled,omitempty" norman:"default=true"`
	ClusterTemplateName string `json:"clusterTemplateName,omitempty" norman:"type=reference[clusterTemplate],required,noupdate"`

	Questions     []Question       `json:"questions,omitempty"`
	ClusterConfig *ClusterSpecBase `json:"clusterConfig" norman:"required"`
}

type Question struct {
	Variable          string        `json:"variable,omitempty" yaml:"variable,omitempty"`
	Label             string        `json:"label,omitempty" yaml:"label,omitempty"`
	Description       string        `json:"description,omitempty" yaml:"description,omitempty"`
	Type              string        `json:"type,omitempty" yaml:"type,omitempty"`
	Required          bool          `json:"required,omitempty" yaml:"required,omitempty"`
	Default           string        `json:"default,omitempty" yaml:"default,omitempty"`
	Group             string        `json:"group,omitempty" yaml:"group,omitempty"`
	MinLength         int           `json:"minLength,omitempty" yaml:"min_length,omitempty"`
	MaxLength         int           `json:"maxLength,omitempty" yaml:"max_length,omitempty"`
	Min               int           `json:"min,omitempty" yaml:"min,omitempty"`
	Max               int           `json:"max,omitempty" yaml:"max,omitempty"`
	Options           []string      `json:"options,omitempty" yaml:"options,omitempty"`
	ValidChars        string        `json:"validChars,omitempty" yaml:"valid_chars,omitempty"`
	InvalidChars      string        `json:"invalidChars,omitempty" yaml:"invalid_chars,omitempty"`
	Subquestions      []SubQuestion `json:"subquestions,omitempty" yaml:"subquestions,omitempty"`
	ShowIf            string        `json:"showIf,omitempty" yaml:"show_if,omitempty"`
	ShowSubquestionIf string        `json:"showSubquestionIf,omitempty" yaml:"show_subquestion_if,omitempty"`
	Satisfies         string        `json:"satisfies,omitempty" yaml:"satisfies,omitempty"`
}

type SubQuestion struct {
	Variable     string   `json:"variable,omitempty" yaml:"variable,omitempty"`
	Label        string   `json:"label,omitempty" yaml:"label,omitempty"`
	Description  string   `json:"description,omitempty" yaml:"description,omitempty"`
	Type         string   `json:"type,omitempty" yaml:"type,omitempty"`
	Required     bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Default      string   `json:"default,omitempty" yaml:"default,omitempty"`
	Group        string   `json:"group,omitempty" yaml:"group,omitempty"`
	MinLength    int      `json:"minLength,omitempty" yaml:"min_length,omitempty"`
	MaxLength    int      `json:"maxLength,omitempty" yaml:"max_length,omitempty"`
	Min          int      `json:"min,omitempty" yaml:"min,omitempty"`
	Max          int      `json:"max,omitempty" yaml:"max,omitempty"`
	Options      []string `json:"options,omitempty" yaml:"options,omitempty"`
	ValidChars   string   `json:"validChars,omitempty" yaml:"valid_chars,omitempty"`
	InvalidChars string   `json:"invalidChars,omitempty" yaml:"invalid_chars,omitempty"`
	ShowIf       string   `json:"showIf,omitempty" yaml:"show_if,omitempty"`
	Satisfies    string   `json:"satisfies,omitempty" yaml:"satisfies,omitempty"`
}

type ClusterTemplateRevisionStatus struct {
	PrivateRegistrySecret            string                             `json:"privateRegistrySecret,omitempty" norman:"nocreate,noupdate"`
	S3CredentialSecret               string                             `json:"s3CredentialSecret,omitempty" norman:"nocreate,noupdate"`
	WeavePasswordSecret              string                             `json:"weavePasswordSecret,omitempty" norman:"nocreate,noupdate"`
	VsphereSecret                    string                             `json:"vsphereSecret,omitempty" norman:"nocreate,noupdate"`
	VirtualCenterSecret              string                             `json:"virtualCenterSecret,omitempty" norman:"nocreate,noupdate"`
	OpenStackSecret                  string                             `json:"openStackSecret,omitempty" norman:"nocreate,noupdate"`
	AADClientSecret                  string                             `json:"aadClientSecret,omitempty" norman:"nocreate,noupdate"`
	AADClientCertSecret              string                             `json:"aadClientCertSecret,omitempty" norman:"nocreate,noupdate"`
	ACIAPICUserKeySecret             string                             `json:"aciAPICUserKeySecret,omitempty" norman:"nocreate,noupdate"`
	ACITokenSecret                   string                             `json:"aciTokenSecret,omitempty" norman:"nocreate,noupdate"`
	ACIKafkaClientKeySecret          string                             `json:"aciKafkaClientKeySecret,omitempty" norman:"nocreate,noupdate"`
	SecretsEncryptionProvidersSecret string                             `json:"secretsEncryptionProvidersSecret,omitempty" norman:"nocreate,noupdate"`
	BastionHostSSHKeySecret          string                             `json:"bastionHostSSHKeySecret,omitempty" norman:"nocreate,noupdate"`
	KubeletExtraEnvSecret            string                             `json:"kubeletExtraEnvSecret,omitempty" norman:"nocreate,noupdate"`
	PrivateRegistryECRSecret         string                             `json:"privateRegistryECRSecret,omitempty" norman:"nocreate,noupdate"`
	Conditions                       []ClusterTemplateRevisionCondition `json:"conditions,omitempty"`
}

type ClusterTemplateQuestionsOutput struct {
	Questions []Question `json:"questions,omitempty"`
}
