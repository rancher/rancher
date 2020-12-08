package indexers

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

const ClusterByPSPTKey = "clusterByPSPT"

func clusterByPSPT(cluster *v3.Cluster) ([]string, error) {
	return []string{cluster.Spec.DefaultPodSecurityPolicyTemplateName}, nil
}
