package client

const (
	SubsetType               = "subset"
	SubsetFieldLabels        = "labels"
	SubsetFieldName          = "name"
	SubsetFieldTrafficPolicy = "trafficPolicy"
)

type Subset struct {
	Labels        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name          string            `json:"name,omitempty" yaml:"name,omitempty"`
	TrafficPolicy *TrafficPolicy    `json:"trafficPolicy,omitempty" yaml:"trafficPolicy,omitempty"`
}
