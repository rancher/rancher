package client

const (
	GithubLoginType              = "githubLogin"
	GithubLoginFieldCode         = "code"
	GithubLoginFieldDescription  = "description"
	GithubLoginFieldResponseType = "responseType"
	GithubLoginFieldTTLMillis    = "ttl"
)

type GithubLogin struct {
	Code         string `json:"code,omitempty"`
	Description  string `json:"description,omitempty"`
	ResponseType string `json:"responseType,omitempty"`
	TTLMillis    *int64 `json:"ttl,omitempty"`
}
