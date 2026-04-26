package client

const (
	KMSConfigurationType            = "kmsConfiguration"
	KMSConfigurationFieldAPIVersion = "apiVersion"
	KMSConfigurationFieldCacheSize  = "cacheSize"
	KMSConfigurationFieldEndpoint   = "endpoint"
	KMSConfigurationFieldName       = "name"
	KMSConfigurationFieldTimeout    = "timeout"
)

type KMSConfiguration struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	CacheSize  *int64 `json:"cacheSize,omitempty" yaml:"cacheSize,omitempty"`
	Endpoint   string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	Timeout    string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
