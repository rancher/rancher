package client

const (
	LocalCredentialType          = "localCredential"
	LocalCredentialFieldPassword = "password"
	LocalCredentialFieldUsername = "username"
)

type LocalCredential struct {
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`
}
