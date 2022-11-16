package cluster

import (
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/pkg/generic"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func (h *handler) OnMgmtClusterRemove(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	provisioningClusters, err := h.clusterCache.GetByIndex(ByCluster, cluster.Name)
	if err != nil {
		return nil, err
	}

	var legacyCluster bool
	for _, provisioningCluster := range provisioningClusters {
		legacyCluster = legacyCluster || isLegacyCluster(provisioningCluster.Name)
		if err = h.clusters.Delete(provisioningCluster.Namespace, provisioningCluster.Name, nil); err != nil {
			return nil, err
		}
	}

	if len(provisioningClusters) == 0 || legacyCluster {
		// If any of the provisioning clusters are legacy clusters (i.e. RKE1 clusters) then we don't wait for the
		// provisioning clusters to be deleted because the provisioning cluster is waiting for the management cluster to delete.
		return cluster, nil
	}

	h.mgmtClusters.EnqueueAfter(cluster.Name, 5*time.Second)
	// generic.ErrSkip will mark the cluster object as reconciled, but won't remove the finalizer.
	// The finalizer should be removed after the provisioning cluster is gone.
	return cluster, generic.ErrSkip
}

func (h *handler) OnClusterRemove(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	cluster = cluster.DeepCopy()

	if cluster.Status.ClusterName != "" {
		mgmtCluster, err := h.mgmtClusters.Get(cluster.Status.ClusterName, metav1.GetOptions{})
		if err != nil {
			// We do nothing if the management cluster does not exist (IsNotFound) because it's been deleted.
			if !apierrors.IsNotFound(err) {
				return cluster, err
			}
		} else if cluster.Namespace == mgmtCluster.Spec.FleetWorkspaceName {
			// We only delete the management cluster if its FleetWorkspaceName matches the provisioning cluster's
			// namespace. The reason: if there's a mismatch, we know that the provisioning cluster needs to be migrated
			// because the user moved the Fleet cluster (and provisioning cluster, by extension) to another
			// FleetWorkspace. Ultimately, the aforementioned cluster objects are re-created in another namespace.
			err = h.mgmtClusters.Delete(cluster.Status.ClusterName, nil)
			if err != nil && !apierrors.IsNotFound(err) {
				return cluster, err
			}

			if isLegacyCluster(cluster.Name) {
				// If this is a legacy cluster (i.e. RKE1 cluster) then we should wait to remove the provisioning cluster until the v3.Cluster is gone.
				_, err = h.mgmtClusterCache.Get(cluster.Status.ClusterName)
				if err != nil {
					if !apierrors.IsNotFound(err) {
						return cluster, err
					}
					return cluster, nil
				} else {
					rke2.Removed.SetStatus(cluster, "Unknown")
					rke2.Removed.Reason(cluster, "Waiting")
					rke2.Removed.Message(cluster, fmt.Sprintf("waiting for management cluster [%s] to delete", cluster.Status.ClusterName))
					cluster, err = h.clusters.UpdateStatus(cluster)
					if err != nil {
						return cluster, err
					}
					return cluster, generic.ErrSkip
				}
			} else {
				if err = h.updateFeatureLockedValue(false); err != nil {
					return cluster, err
				}
			}
		}
	}

	if !isLegacyCluster(cluster.Name) {
		if !features.RKE2.Enabled() {
			return cluster, fmt.Errorf("cannot delete cluster %s while %s is disabled", cluster.Name, features.RKE2.Name())
		}
		capiCluster, capiClusterErr := h.capiClustersCache.Get(cluster.Namespace, cluster.Name)
		if capiClusterErr != nil && !apierrors.IsNotFound(capiClusterErr) {
			return cluster, capiClusterErr
		}

		if capiCluster != nil {
			if capiCluster.DeletionTimestamp == nil {
				// Deleting the CAPI cluster will start the process of deleting Machines, Bootstraps, etc.
				err := h.capiClusters.Delete(capiCluster.Namespace, capiCluster.Name, &metav1.DeleteOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return cluster, err
				}
			}

			_, err := h.rkeControlPlanesCache.Get(cluster.Namespace, cluster.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				return cluster, err
			} else if err == nil {
				return cluster, generic.ErrSkip
			}
		}

		machines, err := h.capiMachinesCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterLabelName: cluster.Name}))
		if err != nil {
			return cluster, err
		}

		// Machines will delete first so report their status, if any exist.
		if len(machines) > 0 {
			msg, err := rke2.GetMachineDeletionStatus(machines)
			if err != nil {
				return cluster, err
			}
			rke2.Removed.SetStatus(cluster, "Unknown")
			rke2.Removed.Reason(cluster, "Waiting")
			rke2.Removed.Message(cluster, msg)
			cluster, err = h.clusters.UpdateStatus(cluster)
			if err != nil {
				return cluster, err
			}
			return cluster, generic.ErrSkip
		}

		if capiClusterErr == nil {
			rke2.Removed.SetStatus(cluster, "Unknown")
			rke2.Removed.Reason(cluster, "Waiting")
			rke2.Removed.Message(cluster, fmt.Sprintf("waiting for cluster-api cluster [%s] to delete", cluster.Name))
			cluster, err = h.clusters.UpdateStatus(cluster)
			if err != nil {
				return cluster, err
			}
			return cluster, generic.ErrSkip
		}
	}

	err := h.kubeconfigManager.DeleteUser(cluster.Namespace, cluster.Name)
	if err != nil {
		return cluster, err
	}

	return cluster, nil
}
