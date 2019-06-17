package client

const (
	DestinationRuleSpecType               = "destinationRuleSpec"
	DestinationRuleSpecFieldHost          = "host"
	DestinationRuleSpecFieldSubsets       = "subsets"
	DestinationRuleSpecFieldTrafficPolicy = "trafficPolicy"
)

type DestinationRuleSpec struct {
	Host          string         `json:"host,omitempty" yaml:"host,omitempty"`
	Subsets       []Subset       `json:"subsets,omitempty" yaml:"subsets,omitempty"`
	TrafficPolicy *TrafficPolicy `json:"trafficPolicy,omitempty" yaml:"trafficPolicy,omitempty"`
}
