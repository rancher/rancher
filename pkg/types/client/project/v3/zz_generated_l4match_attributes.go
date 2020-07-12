package client

const (
	L4MatchAttributesType                    = "l4MatchAttributes"
	L4MatchAttributesFieldDestinationSubnets = "destinationSubnets"
	L4MatchAttributesFieldGateways           = "gateways"
	L4MatchAttributesFieldPort               = "port"
	L4MatchAttributesFieldSourceLabels       = "sourceLabels"
)

type L4MatchAttributes struct {
	DestinationSubnets []string          `json:"destinationSubnets,omitempty" yaml:"destinationSubnets,omitempty"`
	Gateways           []string          `json:"gateways,omitempty" yaml:"gateways,omitempty"`
	Port               int64             `json:"port,omitempty" yaml:"port,omitempty"`
	SourceLabels       map[string]string `json:"sourceLabels,omitempty" yaml:"sourceLabels,omitempty"`
}
