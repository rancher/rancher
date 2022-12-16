package resources

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

// CreateNodePool is a helper function that creates a node pool.
func CreateNodePool(aksHostCluster *management.AKSClusterConfigSpec) (*management.AKSNodePool, error) {
	nodePool := management.AKSNodePool{
		AvailabilityZones: aksHostCluster.NodePools[0].AvailabilityZones,
		Count:             aksHostCluster.NodePools[0].Count,
		EnableAutoScaling: aksHostCluster.NodePools[0].EnableAutoScaling,
		MaxCount:          aksHostCluster.NodePools[0].MaxCount,
		MaxPods:           aksHostCluster.NodePools[0].MaxPods,
		MinCount:          aksHostCluster.NodePools[0].MinCount,
		Mode:              aksHostCluster.NodePools[0].Mode,
		Name:              aksHostCluster.NodePools[0].Name,
		OsDiskSizeGB:      aksHostCluster.NodePools[0].OsDiskSizeGB,
		OsDiskType:        aksHostCluster.NodePools[0].OsDiskType,
		OsType:            aksHostCluster.NodePools[0].OsType,
		VMSize:            aksHostCluster.NodePools[0].VMSize,
	}

	return &nodePool, nil
}
