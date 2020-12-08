package indexers

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

const ClusterByGenericEngineConfigKey = "genericEngineConfig"

// clusterByKontainerDriver is an indexer function that uses the cluster genericEngineConfig
// driverName field
func clusterByKontainerDriver(cluster *v3.Cluster) ([]string, error) {
	engineConfig := cluster.Spec.GenericEngineConfig
	if engineConfig == nil {
		return []string{}, nil
	}
	driverName, ok := (*engineConfig)["driverName"].(string)
	if !ok {
		return []string{}, nil
	}

	return []string{driverName}, nil
}
