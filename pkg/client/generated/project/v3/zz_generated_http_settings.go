package client

const (
	HTTPSettingsType                          = "httpSettings"
	HTTPSettingsFieldHTTP1MaxPendingRequests  = "http1MaxPendingRequests"
	HTTPSettingsFieldHTTP2MaxRequests         = "http2MaxRequests"
	HTTPSettingsFieldMaxRequestsPerConnection = "maxRequestsPerConnection"
	HTTPSettingsFieldMaxRetries               = "maxRetries"
)

type HTTPSettings struct {
	HTTP1MaxPendingRequests  int64 `json:"http1MaxPendingRequests,omitempty" yaml:"http1MaxPendingRequests,omitempty"`
	HTTP2MaxRequests         int64 `json:"http2MaxRequests,omitempty" yaml:"http2MaxRequests,omitempty"`
	MaxRequestsPerConnection int64 `json:"maxRequestsPerConnection,omitempty" yaml:"maxRequestsPerConnection,omitempty"`
	MaxRetries               int64 `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
}
