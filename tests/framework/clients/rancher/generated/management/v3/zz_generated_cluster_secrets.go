package client

const (
	ClusterSecretsType                       = "clusterSecrets"
	ClusterSecretsFieldAADClientCertSecret   = "aadClientCertSecret"
	ClusterSecretsFieldAADClientSecret       = "aadClientSecret"
	ClusterSecretsFieldOpenStackSecret       = "openStackSecret"
	ClusterSecretsFieldPrivateRegistrySecret = "privateRegistrySecret"
	ClusterSecretsFieldPrivateRegistryURL    = "privateRegistryURL"
	ClusterSecretsFieldS3CredentialSecret    = "s3CredentialSecret"
	ClusterSecretsFieldVirtualCenterSecret   = "virtualCenterSecret"
	ClusterSecretsFieldVsphereSecret         = "vsphereSecret"
	ClusterSecretsFieldWeavePasswordSecret   = "weavePasswordSecret"
)

type ClusterSecrets struct {
	AADClientCertSecret   string `json:"aadClientCertSecret,omitempty" yaml:"aadClientCertSecret,omitempty"`
	AADClientSecret       string `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	OpenStackSecret       string `json:"openStackSecret,omitempty" yaml:"openStackSecret,omitempty"`
	PrivateRegistrySecret string `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	PrivateRegistryURL    string `json:"privateRegistryURL,omitempty" yaml:"privateRegistryURL,omitempty"`
	S3CredentialSecret    string `json:"s3CredentialSecret,omitempty" yaml:"s3CredentialSecret,omitempty"`
	VirtualCenterSecret   string `json:"virtualCenterSecret,omitempty" yaml:"virtualCenterSecret,omitempty"`
	VsphereSecret         string `json:"vsphereSecret,omitempty" yaml:"vsphereSecret,omitempty"`
	WeavePasswordSecret   string `json:"weavePasswordSecret,omitempty" yaml:"weavePasswordSecret,omitempty"`
}
