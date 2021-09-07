package client

const (
	FluentServerType           = "fluentServer"
	FluentServerFieldEndpoint  = "endpoint"
	FluentServerFieldHostname  = "hostname"
	FluentServerFieldPassword  = "password"
	FluentServerFieldSharedKey = "sharedKey"
	FluentServerFieldStandby   = "standby"
	FluentServerFieldUsername  = "username"
	FluentServerFieldWeight    = "weight"
)

type FluentServer struct {
	Endpoint  string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Hostname  string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Password  string `json:"password,omitempty" yaml:"password,omitempty"`
	SharedKey string `json:"sharedKey,omitempty" yaml:"sharedKey,omitempty"`
	Standby   bool   `json:"standby,omitempty" yaml:"standby,omitempty"`
	Username  string `json:"username,omitempty" yaml:"username,omitempty"`
	Weight    int64  `json:"weight,omitempty" yaml:"weight,omitempty"`
}
