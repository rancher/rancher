package client

const (
	DaemonSetUpdateStrategyType               = "daemonSetUpdateStrategy"
	DaemonSetUpdateStrategyFieldRollingUpdate = "rollingUpdate"
	DaemonSetUpdateStrategyFieldStrategy      = "strategy"
)

type DaemonSetUpdateStrategy struct {
	RollingUpdate *RollingUpdateDaemonSet `json:"rollingUpdate,omitempty" yaml:"rollingUpdate,omitempty"`
	Strategy      string                  `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
