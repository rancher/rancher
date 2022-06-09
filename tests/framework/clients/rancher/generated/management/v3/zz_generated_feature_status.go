package client

const (
	FeatureStatusType             = "featureStatus"
	FeatureStatusFieldDefault     = "default"
	FeatureStatusFieldDescription = "description"
	FeatureStatusFieldDynamic     = "dynamic"
	FeatureStatusFieldLockedValue = "lockedValue"
)

type FeatureStatus struct {
	Default     bool   `json:"default,omitempty" yaml:"default,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Dynamic     bool   `json:"dynamic,omitempty" yaml:"dynamic,omitempty"`
	LockedValue *bool  `json:"lockedValue,omitempty" yaml:"lockedValue,omitempty"`
}
