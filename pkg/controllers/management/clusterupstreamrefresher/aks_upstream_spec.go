package clusterupstreamrefresher

import (
	"context"

	akscontroller "github.com/rancher/aks-operator/controller"
	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
)

func BuildAKSUpstreamSpec(secretsCache wranglerv1.SecretCache, cluster *mgmtv3.Cluster) (*aksv1.AKSClusterConfigSpec, error) {
	ctx := context.Background()
	upstreamSpec, err := akscontroller.BuildUpstreamClusterState(ctx, secretsCache, cluster.Spec.AKSConfig)
	if err != nil {
		return nil, err
	}

	upstreamSpec.ClusterName = cluster.Spec.AKSConfig.ClusterName
	upstreamSpec.ResourceLocation = cluster.Spec.AKSConfig.ResourceLocation
	upstreamSpec.ResourceGroup = cluster.Spec.AKSConfig.ResourceGroup
	upstreamSpec.TenantID = cluster.Spec.AKSConfig.TenantID
	upstreamSpec.AzureCredentialSecret = cluster.Spec.AKSConfig.AzureCredentialSecret
	upstreamSpec.Imported = cluster.Spec.AKSConfig.Imported

	return upstreamSpec, nil
}
