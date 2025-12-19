package importedclusterversionmanagement

import (
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
)

const (
	// VersionManagementAnno indicates whether the version management is enabled for a cluster.
	// It defines the cluster-level behavior and takes precedence over the 'imported-cluster-version-management' setting.
	// If its value is system-default, the value of the 'imported-cluster-version-management' setting will be used.
	// It is only recognized on imported RKE2/K3s clusters and the local cluster if it is an RKE2/k3s cluster.
	// It is ignored if found on a mgmt v3 cluster for other types of clusters.
	// Expected values: "true", "false", or "system-default" (type: string)
	VersionManagementAnno = "rancher.io/imported-cluster-version-management"
)

// Enabled checks if version management is enabled for a given cluster
func Enabled(cluster *mgmtv3.Cluster) bool {
	if cluster == nil {
		return false
	}
	switch cluster.Annotations[VersionManagementAnno] {
	case "true":
		return true
	case "false":
		return false
	case "system-default":
		fallthrough
	default:
		return settings.ImportedClusterVersionManagement.Get() == "true"
	}
}
