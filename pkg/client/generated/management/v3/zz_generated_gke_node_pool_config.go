package client

const (
	GKENodePoolConfigType                   = "gkeNodePoolConfig"
	GKENodePoolConfigFieldAutoscaling       = "autoscaling"
	GKENodePoolConfigFieldConfig            = "config"
	GKENodePoolConfigFieldInitialNodeCount  = "initialNodeCount"
	GKENodePoolConfigFieldManagement        = "management"
	GKENodePoolConfigFieldMaxPodsConstraint = "maxPodsConstraint"
	GKENodePoolConfigFieldName              = "name"
	GKENodePoolConfigFieldVersion           = "version"
)

type GKENodePoolConfig struct {
	Autoscaling       *GKENodePoolAutoscaling `json:"autoscaling,omitempty" yaml:"autoscaling,omitempty"`
	Config            *GKENodeConfig          `json:"config,omitempty" yaml:"config,omitempty"`
	InitialNodeCount  *int64                  `json:"initialNodeCount,omitempty" yaml:"initialNodeCount,omitempty"`
	Management        *GKENodePoolManagement  `json:"management,omitempty" yaml:"management,omitempty"`
	MaxPodsConstraint *int64                  `json:"maxPodsConstraint,omitempty" yaml:"maxPodsConstraint,omitempty"`
	Name              *string                 `json:"name,omitempty" yaml:"name,omitempty"`
	Version           *string                 `json:"version,omitempty" yaml:"version,omitempty"`
}
