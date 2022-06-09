package client

const (
	KMSConfigurationType           = "kmsConfiguration"
	KMSConfigurationFieldCacheSize = "cachesize"
	KMSConfigurationFieldEndpoint  = "endpoint"
	KMSConfigurationFieldName      = "name"
	KMSConfigurationFieldTimeout   = "timeout"
)

type KMSConfiguration struct {
	CacheSize *int64    `json:"cachesize,omitempty" yaml:"cachesize,omitempty"`
	Endpoint  string    `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Name      string    `json:"name,omitempty" yaml:"name,omitempty"`
	Timeout   *Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
