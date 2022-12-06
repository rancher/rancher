package resources

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

// CreateNodePool is a helper function that creates a node pool.
func CreateNodePool() (*management.AKSNodePool, error) {
	autoScaling := true
	diskSize := int64(120)
	maxCount := int64(3)
	maxPods := int64(110)
	minCount := int64(1)
	name := "nodepool"
	nodeCount := int64(3)
	zones := []string{"1", "2", "3"}

	nodePool := management.AKSNodePool{
		AvailabilityZones: &zones,
		Count:             &nodeCount,
		EnableAutoScaling: &autoScaling,
		MaxCount:          &maxCount,
		MaxPods:           &maxPods,
		MinCount:          &minCount,
		Mode:              "System",
		Name:              &name,
		OsDiskSizeGB:      &diskSize,
		OsDiskType:        "Managed",
		OsType:            "Linux",
		VMSize:            "Standard_DS2_v2",
	}

	return &nodePool, nil
}
