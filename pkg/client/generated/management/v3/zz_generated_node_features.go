package client

const (
	NodeFeaturesType                          = "nodeFeatures"
	NodeFeaturesFieldSupplementalGroupsPolicy = "supplementalGroupsPolicy"
)

type NodeFeatures struct {
	SupplementalGroupsPolicy *bool `json:"supplementalGroupsPolicy,omitempty" yaml:"supplementalGroupsPolicy,omitempty"`
}
