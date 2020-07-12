package client

const (
	CorsPolicyType                  = "corsPolicy"
	CorsPolicyFieldAllowCredentials = "allowCredentials"
	CorsPolicyFieldAllowHeaders     = "allowHeaders"
	CorsPolicyFieldAllowMethods     = "allowMethods"
	CorsPolicyFieldAllowOrigin      = "allowOrigin"
	CorsPolicyFieldExposeHeaders    = "exposeHeaders"
	CorsPolicyFieldMaxAge           = "maxAge"
)

type CorsPolicy struct {
	AllowCredentials bool     `json:"allowCredentials,omitempty" yaml:"allowCredentials,omitempty"`
	AllowHeaders     []string `json:"allowHeaders,omitempty" yaml:"allowHeaders,omitempty"`
	AllowMethods     []string `json:"allowMethods,omitempty" yaml:"allowMethods,omitempty"`
	AllowOrigin      []string `json:"allowOrigin,omitempty" yaml:"allowOrigin,omitempty"`
	ExposeHeaders    []string `json:"exposeHeaders,omitempty" yaml:"exposeHeaders,omitempty"`
	MaxAge           string   `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
}
