package client

const (
	GKEMasterAuthorizedNetworksConfigType            = "gkeMasterAuthorizedNetworksConfig"
	GKEMasterAuthorizedNetworksConfigFieldCidrBlocks = "cidrBlocks"
	GKEMasterAuthorizedNetworksConfigFieldEnabled    = "enabled"
)

type GKEMasterAuthorizedNetworksConfig struct {
	CidrBlocks []GKECidrBlock `json:"cidrBlocks,omitempty" yaml:"cidrBlocks,omitempty"`
	Enabled    bool           `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}
