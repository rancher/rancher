package client

const (
	NodeUpgradeStrategyType               = "nodeUpgradeStrategy"
	NodeUpgradeStrategyFieldRollingUpdate = "rollingUpdateStrategy"
)

type NodeUpgradeStrategy struct {
	RollingUpdate *RollingUpdateStrategy `json:"rollingUpdateStrategy,omitempty" yaml:"rollingUpdateStrategy,omitempty"`
}
