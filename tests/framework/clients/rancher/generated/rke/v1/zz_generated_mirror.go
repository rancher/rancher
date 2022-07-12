package client

const (
	MirrorType           = "mirror"
	MirrorFieldEndpoints = "endpoint"
	MirrorFieldRewrites  = "rewrite"
)

type Mirror struct {
	Endpoints []string          `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Rewrites  map[string]string `json:"rewrite,omitempty" yaml:"rewrite,omitempty"`
}
