package cluster

import (
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

// DownstreamCleanupRequired reports whether Rancher has to clean up after itself in the downstream cluster
// when cluster is removed, because the downstream cluster outlives it: imported clusters, and hosted
// clusters that were imported. The infrastructure of other clusters, such as the ones a provisioning cluster
// or a hosted provider created, is removed with them.
//
// It doesn't cover the local cluster, which is never removed this way.
func DownstreamCleanupRequired(cluster *apimgmtv3.Cluster) bool {
	driver := cluster.Status.Driver
	return driver == apimgmtv3.ClusterDriverK3s ||
		driver == apimgmtv3.ClusterDriverK3os ||
		driver == apimgmtv3.ClusterDriverRke2 ||
		driver == apimgmtv3.ClusterDriverRancherD ||
		// Imported clusters administered by a provisioning cluster are the ones it created.
		(driver == apimgmtv3.ClusterDriverImported && cluster.Annotations["provisioning.cattle.io/administrated"] != "true") ||
		(cluster.Status.AKSStatus.UpstreamSpec != nil && cluster.Status.AKSStatus.UpstreamSpec.Imported) ||
		(cluster.Status.EKSStatus.UpstreamSpec != nil && cluster.Status.EKSStatus.UpstreamSpec.Imported) ||
		(cluster.Status.GKEStatus.UpstreamSpec != nil && cluster.Status.GKEStatus.UpstreamSpec.Imported) ||
		(cluster.Status.AliStatus.UpstreamSpec != nil && cluster.Status.AliStatus.UpstreamSpec.Imported)
}

// NeverConnected reports whether cluster has never been reached by Rancher, so nothing was ever deployed to
// it. Its API endpoint and CA certificate are only known once its agent has connected.
func NeverConnected(cluster *apimgmtv3.Cluster) bool {
	return cluster.Status.APIEndpoint == "" || cluster.Status.CACert == ""
}

// SkipDownstreamCleanupOnRemoval reports whether what a ClusterRoleTemplateBinding or
// ProjectRoleTemplateBinding granted in the downstream cluster is left in place when the binding goes away
// with cluster, which is being removed, and why. The downstream cluster is only cleaned up while the
// removal of the cluster still allows it: it is skipped for clusters whose downstream resources go away
// with them or that never connected, and once the removal has moved past removing the bindings.
func SkipDownstreamCleanupOnRemoval(cluster *apimgmtv3.Cluster) (bool, string) {
	switch {
	case !DownstreamCleanupRequired(cluster):
		return true, "its downstream resources are removed with it"
	case NeverConnected(cluster):
		return true, "it never connected"
	case ConditionConcluded(cluster, apimgmtv3.ClusterConditionRoleTemplateBindingsRemoved):
		return true, "its removal has moved past removing role template bindings"
	}
	return false, ""
}
