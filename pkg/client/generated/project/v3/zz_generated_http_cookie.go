package client

const (
	HTTPCookieType      = "httpCookie"
	HTTPCookieFieldName = "name"
	HTTPCookieFieldPath = "path"
	HTTPCookieFieldTTL  = "ttl"
)

type HTTPCookie struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	TTL  string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}
