package imported

import v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

func IsAdministratedByProvisioningCluster(cluster *v3.Cluster) bool {
	return (cluster.Status.Driver == v3.ClusterDriverImported || cluster.Status.Driver == "") && cluster.Annotations["provisioning.cattle.io/administrated"] == "true"
}
