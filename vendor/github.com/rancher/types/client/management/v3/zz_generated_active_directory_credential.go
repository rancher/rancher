package client

const (
	ActiveDirectoryCredentialType          = "activeDirectoryCredential"
	ActiveDirectoryCredentialFieldPassword = "password"
	ActiveDirectoryCredentialFieldUsername = "username"
)

type ActiveDirectoryCredential struct {
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`
}
