package client

const (
	ClusterSecretsType                                  = "clusterSecrets"
	ClusterSecretsFieldAADClientCertSecret              = "aadClientCertSecret"
	ClusterSecretsFieldAADClientSecret                  = "aadClientSecret"
	ClusterSecretsFieldACIAPICUserKeySecret             = "aciAPICUserKeySecret"
	ClusterSecretsFieldACIKafkaClientKeySecret          = "aciKafkaClientKeySecret"
	ClusterSecretsFieldACITokenSecret                   = "aciTokenSecret"
	ClusterSecretsFieldBastionHostSSHKeySecret          = "bastionHostSSHKeySecret"
	ClusterSecretsFieldKubeletExtraEnvSecret            = "kubeletExtraEnvSecret"
	ClusterSecretsFieldOpenStackSecret                  = "openStackSecret"
	ClusterSecretsFieldPrivateRegistryECRSecret         = "privateRegistryECRSecret"
	ClusterSecretsFieldPrivateRegistrySecret            = "privateRegistrySecret"
	ClusterSecretsFieldPrivateRegistryURL               = "privateRegistryURL"
	ClusterSecretsFieldS3CredentialSecret               = "s3CredentialSecret"
	ClusterSecretsFieldSecretsEncryptionProvidersSecret = "secretsEncryptionProvidersSecret"
	ClusterSecretsFieldVirtualCenterSecret              = "virtualCenterSecret"
	ClusterSecretsFieldVsphereSecret                    = "vsphereSecret"
	ClusterSecretsFieldWeavePasswordSecret              = "weavePasswordSecret"
)

type ClusterSecrets struct {
	AADClientCertSecret              string `json:"aadClientCertSecret,omitempty" yaml:"aadClientCertSecret,omitempty"`
	AADClientSecret                  string `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	ACIAPICUserKeySecret             string `json:"aciAPICUserKeySecret,omitempty" yaml:"aciAPICUserKeySecret,omitempty"`
	ACIKafkaClientKeySecret          string `json:"aciKafkaClientKeySecret,omitempty" yaml:"aciKafkaClientKeySecret,omitempty"`
	ACITokenSecret                   string `json:"aciTokenSecret,omitempty" yaml:"aciTokenSecret,omitempty"`
	BastionHostSSHKeySecret          string `json:"bastionHostSSHKeySecret,omitempty" yaml:"bastionHostSSHKeySecret,omitempty"`
	KubeletExtraEnvSecret            string `json:"kubeletExtraEnvSecret,omitempty" yaml:"kubeletExtraEnvSecret,omitempty"`
	OpenStackSecret                  string `json:"openStackSecret,omitempty" yaml:"openStackSecret,omitempty"`
	PrivateRegistryECRSecret         string `json:"privateRegistryECRSecret,omitempty" yaml:"privateRegistryECRSecret,omitempty"`
	PrivateRegistrySecret            string `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	PrivateRegistryURL               string `json:"privateRegistryURL,omitempty" yaml:"privateRegistryURL,omitempty"`
	S3CredentialSecret               string `json:"s3CredentialSecret,omitempty" yaml:"s3CredentialSecret,omitempty"`
	SecretsEncryptionProvidersSecret string `json:"secretsEncryptionProvidersSecret,omitempty" yaml:"secretsEncryptionProvidersSecret,omitempty"`
	VirtualCenterSecret              string `json:"virtualCenterSecret,omitempty" yaml:"virtualCenterSecret,omitempty"`
	VsphereSecret                    string `json:"vsphereSecret,omitempty" yaml:"vsphereSecret,omitempty"`
	WeavePasswordSecret              string `json:"weavePasswordSecret,omitempty" yaml:"weavePasswordSecret,omitempty"`
}
