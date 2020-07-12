package client

const (
	SourceCodeCredentialStatusType        = "sourceCodeCredentialStatus"
	SourceCodeCredentialStatusFieldLogout = "logout"
)

type SourceCodeCredentialStatus struct {
	Logout bool `json:"logout,omitempty" yaml:"logout,omitempty"`
}
