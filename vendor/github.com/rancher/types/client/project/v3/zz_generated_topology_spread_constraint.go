package client

const (
	TopologySpreadConstraintType                   = "topologySpreadConstraint"
	TopologySpreadConstraintFieldLabelSelector     = "labelSelector"
	TopologySpreadConstraintFieldMaxSkew           = "maxSkew"
	TopologySpreadConstraintFieldTopologyKey       = "topologyKey"
	TopologySpreadConstraintFieldWhenUnsatisfiable = "whenUnsatisfiable"
)

type TopologySpreadConstraint struct {
	LabelSelector     *LabelSelector `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	MaxSkew           int64          `json:"maxSkew,omitempty" yaml:"maxSkew,omitempty"`
	TopologyKey       string         `json:"topologyKey,omitempty" yaml:"topologyKey,omitempty"`
	WhenUnsatisfiable string         `json:"whenUnsatisfiable,omitempty" yaml:"whenUnsatisfiable,omitempty"`
}
