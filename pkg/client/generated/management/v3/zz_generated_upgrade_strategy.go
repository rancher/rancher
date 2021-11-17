package client


	


import (
	
)

const (
    UpgradeStrategyType = "upgradeStrategy"
	UpgradeStrategyFieldRollingUpdate = "rollingUpdate"
)

type UpgradeStrategy struct {
        RollingUpdate *RollingUpdate `json:"rollingUpdate,omitempty" yaml:"rollingUpdate,omitempty"`
}

