package clusterupstreamrefresher

import (
	"context"

	gkecontroller "github.com/rancher/gke-operator/controller"
	gkev1 "github.com/rancher/gke-operator/pkg/apis/gke.cattle.io/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wranglerv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
)

func BuildGKEUpstreamSpec(secretsCache wranglerv1.SecretCache, secretClient wranglerv1.SecretClient, cluster *mgmtv3.Cluster) (*gkev1.GKEClusterConfigSpec, error) {
	ctx := context.Background()
	upstreamSpec, err := gkecontroller.BuildUpstreamClusterState(ctx, secretsCache, secretClient, cluster.Spec.GKEConfig)
	if err != nil {
		return nil, err
	}

	upstreamSpec.ClusterName = cluster.Spec.GKEConfig.ClusterName
	upstreamSpec.Region = cluster.Spec.GKEConfig.Region
	upstreamSpec.Zone = cluster.Spec.GKEConfig.Zone
	upstreamSpec.GoogleCredentialSecret = cluster.Spec.GKEConfig.GoogleCredentialSecret
	upstreamSpec.ProjectID = cluster.Spec.GKEConfig.ProjectID
	upstreamSpec.Imported = cluster.Spec.GKEConfig.Imported

	return upstreamSpec, nil
}
