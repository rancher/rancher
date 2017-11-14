package client

const (
	CapabilitiesType         = "capabilities"
	CapabilitiesFieldCapAdd  = "capAdd"
	CapabilitiesFieldCapDrop = "capDrop"
)

type Capabilities struct {
	CapAdd  []string `json:"capAdd,omitempty"`
	CapDrop []string `json:"capDrop,omitempty"`
}
