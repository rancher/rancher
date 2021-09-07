package client

const (
	ArbitraryFSAccessThroughSMsConfigType      = "arbitraryFSAccessThroughSMsConfig"
	ArbitraryFSAccessThroughSMsConfigFieldDeny = "deny"
)

type ArbitraryFSAccessThroughSMsConfig struct {
	Deny bool `json:"deny,omitempty" yaml:"deny,omitempty"`
}
