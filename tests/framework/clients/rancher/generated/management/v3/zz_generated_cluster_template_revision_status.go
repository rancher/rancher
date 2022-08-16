package client

const (
	ClusterTemplateRevisionStatusType                       = "clusterTemplateRevisionStatus"
	ClusterTemplateRevisionStatusFieldAADClientCertSecret   = "aadClientCertSecret"
	ClusterTemplateRevisionStatusFieldAADClientSecret       = "aadClientSecret"
	ClusterTemplateRevisionStatusFieldConditions            = "conditions"
	ClusterTemplateRevisionStatusFieldOpenStackSecret       = "openStackSecret"
	ClusterTemplateRevisionStatusFieldPrivateRegistrySecret = "privateRegistrySecret"
	ClusterTemplateRevisionStatusFieldS3CredentialSecret    = "s3CredentialSecret"
	ClusterTemplateRevisionStatusFieldVirtualCenterSecret   = "virtualCenterSecret"
	ClusterTemplateRevisionStatusFieldVsphereSecret         = "vsphereSecret"
	ClusterTemplateRevisionStatusFieldWeavePasswordSecret   = "weavePasswordSecret"
)

type ClusterTemplateRevisionStatus struct {
	AADClientCertSecret   string                             `json:"aadClientCertSecret,omitempty" yaml:"aadClientCertSecret,omitempty"`
	AADClientSecret       string                             `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	Conditions            []ClusterTemplateRevisionCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	OpenStackSecret       string                             `json:"openStackSecret,omitempty" yaml:"openStackSecret,omitempty"`
	PrivateRegistrySecret string                             `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	S3CredentialSecret    string                             `json:"s3CredentialSecret,omitempty" yaml:"s3CredentialSecret,omitempty"`
	VirtualCenterSecret   string                             `json:"virtualCenterSecret,omitempty" yaml:"virtualCenterSecret,omitempty"`
	VsphereSecret         string                             `json:"vsphereSecret,omitempty" yaml:"vsphereSecret,omitempty"`
	WeavePasswordSecret   string                             `json:"weavePasswordSecret,omitempty" yaml:"weavePasswordSecret,omitempty"`
}
