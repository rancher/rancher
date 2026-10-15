package cluster

import (
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
)

// ImportedVersionManagementPaused reports whether version management is currently suspended for the cluster. Unlike ImportedVersionManagementEnabled
// this has no cluster-independent default: absent means not paused.
func ImportedVersionManagementPaused(cluster *v3.Cluster) bool {
	if cluster == nil {
		return false
	}
	return cluster.Annotations[v3.ClusterImportedVersionManagementPausedAnnotation] == "true"
}

// ImportedVersionManagementEnabled checks if version management is enabled for a given cluster
func ImportedVersionManagementEnabled(cluster *v3.Cluster) bool {
	if cluster == nil {
		return false
	}

	switch cluster.Annotations[v3.ClusterImportedVersionManagementAnnotation] {
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
