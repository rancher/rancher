package client

const (
	LoginInputType                  = "loginInput"
	LoginInputFieldDescription      = "description"
	LoginInputFieldGithubCredential = "githubCredential"
	LoginInputFieldLocalCredential  = "localCredential"
	LoginInputFieldResponseType     = "responseType"
	LoginInputFieldTTLMillis        = "ttl"
)

type LoginInput struct {
	Description      string            `json:"description,omitempty"`
	GithubCredential *GithubCredential `json:"githubCredential,omitempty"`
	LocalCredential  *LocalCredential  `json:"localCredential,omitempty"`
	ResponseType     string            `json:"responseType,omitempty"`
	TTLMillis        *int64            `json:"ttl,omitempty"`
}
