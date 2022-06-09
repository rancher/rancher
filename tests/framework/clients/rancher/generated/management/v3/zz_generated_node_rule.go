package client

const (
	NodeRuleType              = "nodeRule"
	NodeRuleFieldCPUThreshold = "cpuThreshold"
	NodeRuleFieldCondition    = "condition"
	NodeRuleFieldMemThreshold = "memThreshold"
	NodeRuleFieldNodeID       = "nodeId"
	NodeRuleFieldSelector     = "selector"
)

type NodeRule struct {
	CPUThreshold int64             `json:"cpuThreshold,omitempty" yaml:"cpuThreshold,omitempty"`
	Condition    string            `json:"condition,omitempty" yaml:"condition,omitempty"`
	MemThreshold int64             `json:"memThreshold,omitempty" yaml:"memThreshold,omitempty"`
	NodeID       string            `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Selector     map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
}
