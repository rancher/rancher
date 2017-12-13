package client

const (
	LoginInputType                          = "loginInput"
	LoginInputFieldDescription              = "description"
	LoginInputFieldGithubCredential         = "githubCredential"
	LoginInputFieldIdentityRefreshTTLMillis = "identityRefreshTTL"
	LoginInputFieldLocalCredential          = "localCredential"
	LoginInputFieldResponseType             = "responseType"
	LoginInputFieldTTLMillis                = "ttl"
)

type LoginInput struct {
	Description              string            `json:"description,omitempty"`
	GithubCredential         *GithubCredential `json:"githubCredential,omitempty"`
	IdentityRefreshTTLMillis string            `json:"identityRefreshTTL,omitempty"`
	LocalCredential          *LocalCredential  `json:"localCredential,omitempty"`
	ResponseType             string            `json:"responseType,omitempty"`
	TTLMillis                string            `json:"ttl,omitempty"`
}
