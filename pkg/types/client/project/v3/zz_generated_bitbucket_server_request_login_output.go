package client

const (
	BitbucketServerRequestLoginOutputType          = "bitbucketServerRequestLoginOutput"
	BitbucketServerRequestLoginOutputFieldLoginURL = "loginUrl"
)

type BitbucketServerRequestLoginOutput struct {
	LoginURL string `json:"loginUrl,omitempty" yaml:"loginUrl,omitempty"`
}
