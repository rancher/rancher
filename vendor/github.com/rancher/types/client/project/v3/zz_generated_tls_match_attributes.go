package client

const (
	TLSMatchAttributesType                    = "tlsMatchAttributes"
	TLSMatchAttributesFieldDestinationSubnets = "destinationSubnets"
	TLSMatchAttributesFieldGateways           = "gateways"
	TLSMatchAttributesFieldPort               = "port"
	TLSMatchAttributesFieldSniHosts           = "sniHosts"
	TLSMatchAttributesFieldSourceLabels       = "sourceLabels"
)

type TLSMatchAttributes struct {
	DestinationSubnets []string          `json:"destinationSubnets,omitempty" yaml:"destinationSubnets,omitempty"`
	Gateways           []string          `json:"gateways,omitempty" yaml:"gateways,omitempty"`
	Port               int64             `json:"port,omitempty" yaml:"port,omitempty"`
	SniHosts           []string          `json:"sniHosts,omitempty" yaml:"sniHosts,omitempty"`
	SourceLabels       map[string]string `json:"sourceLabels,omitempty" yaml:"sourceLabels,omitempty"`
}
