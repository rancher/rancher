package imported

import v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

// AdministratedAnnotation is set on a management cluster when it is created as the
// management-side mirror of a v2prov (CAPR) provisioning cluster. It is written once at
// creation and never changes, which makes it safe to key on from code that must not
// observe the cluster changing state (see systemtemplate.isProvisionedRKE2OrK3s).
const AdministratedAnnotation = "provisioning.cattle.io/administrated"

func IsAdministratedByProvisioningCluster(cluster *v3.Cluster) bool {
	return (cluster.Status.Driver == v3.ClusterDriverImported || cluster.Status.Driver == "") && cluster.Annotations[AdministratedAnnotation] == "true"
}
