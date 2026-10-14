package cluster

import "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

func IsAdministrated(cluster *v3.Cluster) bool {
	return (cluster.Status.Driver == v3.ClusterDriverImported || cluster.Status.Driver == "") && cluster.Annotations[v3.ClusterAdministratedAnnotation] == "true"
}
