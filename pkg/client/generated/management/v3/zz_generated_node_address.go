package client

const (
	NodeAddressType         = "nodeAddress"
	NodeAddressFieldAddress = "address"
	NodeAddressFieldType    = "type"
)

type NodeAddress struct {
	Address string `json:"address,omitempty" yaml:"address,omitempty"`
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
}
