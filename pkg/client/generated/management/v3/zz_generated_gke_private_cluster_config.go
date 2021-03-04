package client

const (
	GKEPrivateClusterConfigType                       = "gkePrivateClusterConfig"
	GKEPrivateClusterConfigFieldEnablePrivateEndpoint = "enablePrivateEndpoint"
	GKEPrivateClusterConfigFieldEnablePrivateNodes    = "enablePrivateNodes"
	GKEPrivateClusterConfigFieldMasterIpv4CidrBlock   = "masterIpv4CidrBlock"
	GKEPrivateClusterConfigFieldPrivateEndpoint       = "privateEndpoint"
	GKEPrivateClusterConfigFieldPublicEndpoint        = "publicEndpoint"
)

type GKEPrivateClusterConfig struct {
	EnablePrivateEndpoint bool   `json:"enablePrivateEndpoint,omitempty" yaml:"enablePrivateEndpoint,omitempty"`
	EnablePrivateNodes    bool   `json:"enablePrivateNodes,omitempty" yaml:"enablePrivateNodes,omitempty"`
	MasterIpv4CidrBlock   string `json:"masterIpv4CidrBlock,omitempty" yaml:"masterIpv4CidrBlock,omitempty"`
	PrivateEndpoint       string `json:"privateEndpoint,omitempty" yaml:"privateEndpoint,omitempty"`
	PublicEndpoint        string `json:"publicEndpoint,omitempty" yaml:"publicEndpoint,omitempty"`
}
