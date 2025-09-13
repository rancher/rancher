package clusterupstreamrefresher

import (
	alicontroller "github.com/rancher/ali-operator/controller"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

func BuildAlibabaUpstreamSpec(secretsCache wranglerv1.SecretCache, cluster *apimgmtv3.Cluster) (*aliv1.AliClusterConfigSpec, error) {
	upstreamSpec, err := alicontroller.BuildUpstreamClusterState(secretsCache, cluster.Spec.AliConfig)
	if err != nil {
		return nil, err
	}

	upstreamSpec.ClusterName = cluster.Spec.AliConfig.ClusterName
	upstreamSpec.RegionID = cluster.Spec.AliConfig.RegionID
	upstreamSpec.AlibabaCredentialSecret = cluster.Spec.AliConfig.AlibabaCredentialSecret
	upstreamSpec.Imported = cluster.Spec.AliConfig.Imported

	return upstreamSpec, nil
}
