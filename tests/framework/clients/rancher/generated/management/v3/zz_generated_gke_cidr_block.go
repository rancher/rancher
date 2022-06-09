package client

const (
	GKECidrBlockType             = "gkeCidrBlock"
	GKECidrBlockFieldCidrBlock   = "cidrBlock"
	GKECidrBlockFieldDisplayName = "displayName"
)

type GKECidrBlock struct {
	CidrBlock   string `json:"cidrBlock,omitempty" yaml:"cidrBlock,omitempty"`
	DisplayName string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
}
