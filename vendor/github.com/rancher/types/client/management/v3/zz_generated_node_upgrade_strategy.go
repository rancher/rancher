package client

const (
	NodeUpgradeStrategyType                = "nodeUpgradeStrategy"
	NodeUpgradeStrategyFieldDrain          = "drain"
	NodeUpgradeStrategyFieldDrainInput     = "nodeDrainInput"
	NodeUpgradeStrategyFieldMaxUnavailable = "maxUnavailable"
)

type NodeUpgradeStrategy struct {
	Drain          bool            `json:"drain,omitempty" yaml:"drain,omitempty"`
	DrainInput     *NodeDrainInput `json:"nodeDrainInput,omitempty" yaml:"nodeDrainInput,omitempty"`
	MaxUnavailable string          `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
}
