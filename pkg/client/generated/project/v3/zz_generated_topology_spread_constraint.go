package client

const (
	TopologySpreadConstraintType                    = "topologySpreadConstraint"
	TopologySpreadConstraintFieldLabelSelector      = "labelSelector"
	TopologySpreadConstraintFieldMatchLabelKeys     = "matchLabelKeys"
	TopologySpreadConstraintFieldMaxSkew            = "maxSkew"
	TopologySpreadConstraintFieldMinDomains         = "minDomains"
	TopologySpreadConstraintFieldNodeAffinityPolicy = "nodeAffinityPolicy"
	TopologySpreadConstraintFieldNodeTaintsPolicy   = "nodeTaintsPolicy"
	TopologySpreadConstraintFieldTopologyKey        = "topologyKey"
	TopologySpreadConstraintFieldWhenUnsatisfiable  = "whenUnsatisfiable"
)

type TopologySpreadConstraint struct {
	LabelSelector      *LabelSelector `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	MatchLabelKeys     []string       `json:"matchLabelKeys,omitempty" yaml:"matchLabelKeys,omitempty"`
	MaxSkew            int64          `json:"maxSkew,omitempty" yaml:"maxSkew,omitempty"`
	MinDomains         *int64         `json:"minDomains,omitempty" yaml:"minDomains,omitempty"`
	NodeAffinityPolicy string         `json:"nodeAffinityPolicy,omitempty" yaml:"nodeAffinityPolicy,omitempty"`
	NodeTaintsPolicy   string         `json:"nodeTaintsPolicy,omitempty" yaml:"nodeTaintsPolicy,omitempty"`
	TopologyKey        string         `json:"topologyKey,omitempty" yaml:"topologyKey,omitempty"`
	WhenUnsatisfiable  string         `json:"whenUnsatisfiable,omitempty" yaml:"whenUnsatisfiable,omitempty"`
}
