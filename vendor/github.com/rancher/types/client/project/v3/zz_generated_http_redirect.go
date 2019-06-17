package client

const (
	HTTPRedirectType           = "httpRedirect"
	HTTPRedirectFieldAuthority = "authority"
	HTTPRedirectFieldURI       = "uri"
)

type HTTPRedirect struct {
	Authority string `json:"authority,omitempty" yaml:"authority,omitempty"`
	URI       string `json:"uri,omitempty" yaml:"uri,omitempty"`
}
