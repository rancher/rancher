package client


	

	


import (
	
)

const (
    GKENodePoolManagementType = "gkeNodePoolManagement"
	GKENodePoolManagementFieldAutoRepair = "autoRepair"
	GKENodePoolManagementFieldAutoUpgrade = "autoUpgrade"
)

type GKENodePoolManagement struct {
        AutoRepair bool `json:"autoRepair,omitempty" yaml:"autoRepair,omitempty"`
        AutoUpgrade bool `json:"autoUpgrade,omitempty" yaml:"autoUpgrade,omitempty"`
}

