package client

const (
	EKSClusterConfigSpecType                        = "eksClusterConfigSpec"
	EKSClusterConfigSpecFieldAmazonCredentialSecret = "amazonCredentialSecret"
	EKSClusterConfigSpecFieldDisplayName            = "displayName"
	EKSClusterConfigSpecFieldEBSCSIDriver           = "ebsCSIDriver"
	EKSClusterConfigSpecFieldImported               = "imported"
	EKSClusterConfigSpecFieldKmsKey                 = "kmsKey"
	EKSClusterConfigSpecFieldKubernetesVersion      = "kubernetesVersion"
	EKSClusterConfigSpecFieldLoggingTypes           = "loggingTypes"
	EKSClusterConfigSpecFieldNodeGroups             = "nodeGroups"
	EKSClusterConfigSpecFieldPrivateAccess          = "privateAccess"
	EKSClusterConfigSpecFieldPublicAccess           = "publicAccess"
	EKSClusterConfigSpecFieldPublicAccessSources    = "publicAccessSources"
	EKSClusterConfigSpecFieldRegion                 = "region"
	EKSClusterConfigSpecFieldSecretsEncryption      = "secretsEncryption"
	EKSClusterConfigSpecFieldSecurityGroups         = "securityGroups"
	EKSClusterConfigSpecFieldServiceRole            = "serviceRole"
	EKSClusterConfigSpecFieldSubnets                = "subnets"
	EKSClusterConfigSpecFieldTags                   = "tags"
)

type EKSClusterConfigSpec struct {
	AmazonCredentialSecret string            `json:"amazonCredentialSecret,omitempty" yaml:"amazonCredentialSecret,omitempty"`
	DisplayName            string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	EBSCSIDriver           *bool             `json:"ebsCSIDriver,omitempty" yaml:"ebsCSIDriver,omitempty"`
	Imported               bool              `json:"imported,omitempty" yaml:"imported,omitempty"`
	KmsKey                 *string           `json:"kmsKey,omitempty" yaml:"kmsKey,omitempty"`
	KubernetesVersion      *string           `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	LoggingTypes           []string          `json:"loggingTypes,omitempty" yaml:"loggingTypes,omitempty"`
	NodeGroups             []NodeGroup       `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	PrivateAccess          *bool             `json:"privateAccess,omitempty" yaml:"privateAccess,omitempty"`
	PublicAccess           *bool             `json:"publicAccess,omitempty" yaml:"publicAccess,omitempty"`
	PublicAccessSources    []string          `json:"publicAccessSources,omitempty" yaml:"publicAccessSources,omitempty"`
	Region                 string            `json:"region,omitempty" yaml:"region,omitempty"`
	SecretsEncryption      *bool             `json:"secretsEncryption,omitempty" yaml:"secretsEncryption,omitempty"`
	SecurityGroups         []string          `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
	ServiceRole            *string           `json:"serviceRole,omitempty" yaml:"serviceRole,omitempty"`
	Subnets                []string          `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	Tags                   map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}
