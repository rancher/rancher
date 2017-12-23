package client

const (
	EndpointSubsetType                   = "endpointSubset"
	EndpointSubsetFieldAddresses         = "addresses"
	EndpointSubsetFieldNotReadyAddresses = "notReadyAddresses"
	EndpointSubsetFieldPorts             = "ports"
)

type EndpointSubset struct {
	Addresses         []EndpointAddress `json:"addresses,omitempty"`
	NotReadyAddresses []EndpointAddress `json:"notReadyAddresses,omitempty"`
	Ports             []EndpointPort    `json:"ports,omitempty"`
}
