package client

const (
	CloudCredentialSpecType             = "cloudCredentialSpec"
	CloudCredentialSpecFieldDescription = "description"
	CloudCredentialSpecFieldDisplayName = "displayName"
)

type CloudCredentialSpec struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
}
