package client

const (
	ClusterTemplateRevisionStatusType                                  = "clusterTemplateRevisionStatus"
	ClusterTemplateRevisionStatusFieldAADClientCertSecret              = "aadClientCertSecret"
	ClusterTemplateRevisionStatusFieldAADClientSecret                  = "aadClientSecret"
	ClusterTemplateRevisionStatusFieldACIAPICUserKeySecret             = "aciAPICUserKeySecret"
	ClusterTemplateRevisionStatusFieldACIKafkaClientKeySecret          = "aciKafkaClientKeySecret"
	ClusterTemplateRevisionStatusFieldACITokenSecret                   = "aciTokenSecret"
	ClusterTemplateRevisionStatusFieldBastionHostSSHKeySecret          = "bastionHostSSHKeySecret"
	ClusterTemplateRevisionStatusFieldConditions                       = "conditions"
	ClusterTemplateRevisionStatusFieldKubeletExtraEnvSecret            = "kubeletExtraEnvSecret"
	ClusterTemplateRevisionStatusFieldOpenStackSecret                  = "openStackSecret"
	ClusterTemplateRevisionStatusFieldPrivateRegistryECRSecret         = "privateRegistryECRSecret"
	ClusterTemplateRevisionStatusFieldPrivateRegistrySecret            = "privateRegistrySecret"
	ClusterTemplateRevisionStatusFieldS3CredentialSecret               = "s3CredentialSecret"
	ClusterTemplateRevisionStatusFieldSecretsEncryptionProvidersSecret = "secretsEncryptionProvidersSecret"
	ClusterTemplateRevisionStatusFieldVirtualCenterSecret              = "virtualCenterSecret"
	ClusterTemplateRevisionStatusFieldVsphereSecret                    = "vsphereSecret"
	ClusterTemplateRevisionStatusFieldWeavePasswordSecret              = "weavePasswordSecret"
)

type ClusterTemplateRevisionStatus struct {
	AADClientCertSecret              string                             `json:"aadClientCertSecret,omitempty" yaml:"aadClientCertSecret,omitempty"`
	AADClientSecret                  string                             `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	ACIAPICUserKeySecret             string                             `json:"aciAPICUserKeySecret,omitempty" yaml:"aciAPICUserKeySecret,omitempty"`
	ACIKafkaClientKeySecret          string                             `json:"aciKafkaClientKeySecret,omitempty" yaml:"aciKafkaClientKeySecret,omitempty"`
	ACITokenSecret                   string                             `json:"aciTokenSecret,omitempty" yaml:"aciTokenSecret,omitempty"`
	BastionHostSSHKeySecret          string                             `json:"bastionHostSSHKeySecret,omitempty" yaml:"bastionHostSSHKeySecret,omitempty"`
	Conditions                       []ClusterTemplateRevisionCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	KubeletExtraEnvSecret            string                             `json:"kubeletExtraEnvSecret,omitempty" yaml:"kubeletExtraEnvSecret,omitempty"`
	OpenStackSecret                  string                             `json:"openStackSecret,omitempty" yaml:"openStackSecret,omitempty"`
	PrivateRegistryECRSecret         string                             `json:"privateRegistryECRSecret,omitempty" yaml:"privateRegistryECRSecret,omitempty"`
	PrivateRegistrySecret            string                             `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	S3CredentialSecret               string                             `json:"s3CredentialSecret,omitempty" yaml:"s3CredentialSecret,omitempty"`
	SecretsEncryptionProvidersSecret string                             `json:"secretsEncryptionProvidersSecret,omitempty" yaml:"secretsEncryptionProvidersSecret,omitempty"`
	VirtualCenterSecret              string                             `json:"virtualCenterSecret,omitempty" yaml:"virtualCenterSecret,omitempty"`
	VsphereSecret                    string                             `json:"vsphereSecret,omitempty" yaml:"vsphereSecret,omitempty"`
	WeavePasswordSecret              string                             `json:"weavePasswordSecret,omitempty" yaml:"weavePasswordSecret,omitempty"`
}
