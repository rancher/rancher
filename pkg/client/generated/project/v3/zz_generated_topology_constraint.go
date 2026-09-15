package client

const (
	TopologyConstraintType     = "topologyConstraint"
	TopologyConstraintFieldKey = "key"
)

type TopologyConstraint struct {
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
}
