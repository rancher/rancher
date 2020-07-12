package client

const (
	HTTPRewriteType           = "httpRewrite"
	HTTPRewriteFieldAuthority = "authority"
	HTTPRewriteFieldURI       = "uri"
)

type HTTPRewrite struct {
	Authority string `json:"authority,omitempty" yaml:"authority,omitempty"`
	URI       string `json:"uri,omitempty" yaml:"uri,omitempty"`
}
