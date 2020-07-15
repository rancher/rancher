package client

const (
	TargetNodeType              = "targetNode"
	TargetNodeFieldCPUThreshold = "cpuThreshold"
	TargetNodeFieldCondition    = "condition"
	TargetNodeFieldMemThreshold = "memThreshold"
	TargetNodeFieldNodeID       = "nodeId"
	TargetNodeFieldSelector     = "selector"
)

type TargetNode struct {
	CPUThreshold int64             `json:"cpuThreshold,omitempty" yaml:"cpuThreshold,omitempty"`
	Condition    string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	MemThreshold int64             `json:"memThreshold,omitempty" yaml:"memThreshold,omitempty"`
	NodeID       string            `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Selector     map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
}
