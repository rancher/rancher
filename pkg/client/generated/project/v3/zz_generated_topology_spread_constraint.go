package client

const (
	TopologySpreadConstraintType                   = "topologySpreadConstraint"
	TopologySpreadConstraintFieldLabelSelector     = "labelSelector"
	TopologySpreadConstraintFieldMaxSkew           = "maxSkew"
	TopologySpreadConstraintFieldMinDomains        = "minDomains"
	TopologySpreadConstraintFieldTopologyKey       = "topologyKey"
	TopologySpreadConstraintFieldWhenUnsatisfiable = "whenUnsatisfiable"
)

type TopologySpreadConstraint struct {
	LabelSelector     *LabelSelector `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	MaxSkew           int64          `json:"maxSkew,omitempty" yaml:"maxSkew,omitempty"`
	MinDomains        *int64         `json:"minDomains,omitempty" yaml:"minDomains,omitempty"`
	TopologyKey       string         `json:"topologyKey,omitempty" yaml:"topologyKey,omitempty"`
	WhenUnsatisfiable string         `json:"whenUnsatisfiable,omitempty" yaml:"whenUnsatisfiable,omitempty"`
}
