package client

const (
	CloudCredentialSpecType             = "cloudCredentialSpec"
	CloudCredentialSpecFieldDescription = "description"
)

type CloudCredentialSpec struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}
