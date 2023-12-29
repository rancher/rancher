package gke

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

// CreateGKEHostedCluster is a helper function that creates an GKE hosted cluster
func CreateGKEHostedCluster(client *rancher.Client, displayName, cloudCredentialID string, enableClusterAlerting, enableClusterMonitoring, enableNetworkPolicy, windowsPreferedCluster bool, labels map[string]string) (*management.Cluster, error) {
	gkeHostCluster := gkeHostClusterConfig(displayName, cloudCredentialID)
	cluster := &management.Cluster{
		DockerRootDir:          "/var/lib/docker",
		GKEConfig:              gkeHostCluster,
		Name:                   displayName,
		EnableClusterAlerting:  enableClusterAlerting,
		EnableNetworkPolicy:    &enableNetworkPolicy,
		Labels:                 labels,
		WindowsPreferedCluster: windowsPreferedCluster,
	}

	clusterResp, err := client.Management.Cluster.Create(cluster)
	if err != nil {
		return nil, err
	}
	return clusterResp, err
}
