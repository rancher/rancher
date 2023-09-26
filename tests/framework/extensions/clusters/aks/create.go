package aks

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

// CreateAKSHostedCluster is a helper function that creates an AKS hosted cluster.
func CreateAKSHostedCluster(client *rancher.Client, displayName, cloudCredentialID string, enableClusterAlerting, enableClusterMonitoring, enableNetworkPolicy, windowsPreferedCluster bool, labels map[string]string) (*management.Cluster, error) {
	aksHostCluster := HostClusterConfig(displayName, cloudCredentialID)
	cluster := &management.Cluster{
		AKSConfig:               aksHostCluster,
		DockerRootDir:           "/var/lib/docker",
		EnableClusterAlerting:   enableClusterAlerting,
		EnableClusterMonitoring: enableClusterMonitoring,
		EnableNetworkPolicy:     &enableNetworkPolicy,
		Labels:                  labels,
		Name:                    displayName,
		WindowsPreferedCluster:  windowsPreferedCluster,
	}

	clusterResp, err := client.Management.Cluster.Create(cluster)
	if err != nil {
		return nil, err
	}

	return clusterResp, err
}
