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

	// VersionManagementPausedAnno temporarily suspends version management for a cluster, without
	// changing whether it is enabled. While it is set to "true", k3sbasedupgrade renders nothing:
	// no system-upgrade-controller plans, no plan cleanup, no upgrade condition churn.
	//
	// It exists for etcd snapshot restores. A kubernetesVersion or all restore rewrites the
	// cluster's desired version to the version the snapshot was taken on, and the restore
	// reinstalls the distro on the nodes itself, as part of resetting etcd. Version management
	// observing that desired version mid-restore would start draining nodes to roll out plans of
	// its own, against a cluster whose etcd is being replaced underneath it. The restore sets
	// this on the way in and removes it on the way out, by which point the nodes already match
	// the desired version and there is nothing left to roll out.
	//
	// Expected values: "true", or absent. Any other value is treated as absent.
	VersionManagementPausedAnno = "management.cattle.io/imported-cluster-version-management-paused"
)

// Paused reports whether version management is currently suspended for the cluster. Unlike Enabled
// this has no cluster-independent default: absent means not paused.
func Paused(cluster *mgmtv3.Cluster) bool {
	if cluster == nil {
		return false
	}
	return cluster.Annotations[VersionManagementPausedAnno] == "true"
}

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
