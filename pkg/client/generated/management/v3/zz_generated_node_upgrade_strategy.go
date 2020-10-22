package client

const (
	NodeUpgradeStrategyType                            = "nodeUpgradeStrategy"
	NodeUpgradeStrategyFieldDrain                      = "drain"
	NodeUpgradeStrategyFieldDrainInput                 = "nodeDrainInput"
	NodeUpgradeStrategyFieldMaxUnavailableControlplane = "maxUnavailableControlplane"
	NodeUpgradeStrategyFieldMaxUnavailableWorker       = "maxUnavailableWorker"
)

type NodeUpgradeStrategy struct {
	Drain                      *bool           `json:"drain,omitempty" yaml:"drain,omitempty"`
	DrainInput                 *NodeDrainInput `json:"nodeDrainInput,omitempty" yaml:"nodeDrainInput,omitempty"`
	MaxUnavailableControlplane string          `json:"maxUnavailableControlplane,omitempty" yaml:"maxUnavailableControlplane,omitempty"`
	MaxUnavailableWorker       string          `json:"maxUnavailableWorker,omitempty" yaml:"maxUnavailableWorker,omitempty"`
}
