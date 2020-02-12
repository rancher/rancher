package client

const (
	FeatureStatusType         = "featureStatus"
	FeatureStatusFieldDefault = "default"
	FeatureStatusFieldDynamic = "dynamic"
)

type FeatureStatus struct {
	Default bool `json:"default,omitempty" yaml:"default,omitempty"`
	Dynamic bool `json:"dynamic,omitempty" yaml:"dynamic,omitempty"`
}
