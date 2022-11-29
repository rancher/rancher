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

// +genclient
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
