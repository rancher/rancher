package client

const (
	GithubCredentialType      = "githubCredential"
	GithubCredentialFieldCode = "code"
)

type GithubCredential struct {
	Code string `json:"code,omitempty"`
}
