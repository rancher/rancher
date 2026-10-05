package cluster

import (
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// OnMgmtClusterRemove asks for the provisioning cluster that created a management cluster to be deleted
// when the management cluster is deleted on its own, since the provisioning cluster would otherwise create
// it again. It doesn't wait: the provisioning cluster removes the management cluster it created, see
// removeManagementClusterFinalizer, and a provisioning cluster created by the management cluster is
// removed by removeProvisioningClusterFinalizer.
func (h *handler) OnMgmtClusterRemove(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	namespace, name, ok := util.CreatedByProvisioningCluster(cluster)
	if !ok {
		return cluster, nil
	}
	provCluster, err := h.clusterCache.Get(namespace, name)
	if apierrors.IsNotFound(err) {
		return cluster, nil
	}
	if err != nil {
		return nil, err
	}
	if provCluster.DeletionTimestamp != nil || provCluster.Status.ClusterName != cluster.Name {
		return cluster, nil
	}
	uid := provCluster.UID
	if err := h.clusters.Delete(provCluster.Namespace, provCluster.Name, &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return nil, err
	}
	return cluster, nil
}

func (h *handler) OnClusterRemove(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	oldStatus := cluster.Status
	cluster = cluster.DeepCopy()

	err := capr.DoRemoveAndUpdateStatus(cluster, h.doClusterRemove(cluster), h.clusters.EnqueueAfter)

	if equality.Semantic.DeepEqual(oldStatus, cluster.Status) {
		return cluster, err
	}

	cluster, updateErr := h.clusters.UpdateStatus(cluster)
	if updateErr != nil {
		return cluster, updateErr
	}

	return cluster, err
}

func (h *handler) doClusterRemove(cluster *v1.Cluster) func() (string, error) {
	return func() (string, error) {
		if cluster.Status.ClusterName != "" {
			mgmtCluster, err := h.mgmtClusters.Get(cluster.Status.ClusterName, metav1.GetOptions{})
			if err != nil {
				// We do nothing if the management cluster does not exist (IsNotFound) because it's been deleted.
				if !apierrors.IsNotFound(err) {
					return "", err
				}
			} else if util.CreatedBy(cluster, util.ManagementClusterGVK, "", mgmtCluster.Name) {
				// This provisioning cluster was created by its management cluster, which would create it
				// again: ask for the management cluster to be deleted too. It removes this provisioning
				// cluster itself, see removeProvisioningClusterFinalizer, so there's nothing to wait for.
				if mgmtCluster.DeletionTimestamp == nil {
					uid := mgmtCluster.UID
					err := h.mgmtClusters.Delete(mgmtCluster.Name, &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
					if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
						return "", err
					}
				}
			} else if cluster.Namespace == mgmtCluster.Spec.FleetWorkspaceName {
				// The management cluster this provisioning cluster created is removed once the CAPI cluster and
				// machines below are gone, see removeManagementClusterFinalizer. A management cluster in another
				// fleet workspace means the provisioning cluster is being migrated and is left alone.
				if err = h.updateFeatureLockedValue(false); err != nil {
					return "", err
				}
			}
		}

		// The CAPI cluster informer cache may not be synced yet immediately after a
		// Rancher restart, which would produce a false "not found" below and cause us
		// to release this finalizer without ever deleting the CAPI cluster. If the
		// cache is not synced, re-enqueue the provisioning cluster and try again later.
		if !h.capiClusters.Informer().HasSynced() {
			h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, 5*time.Second)
			return "", generic.ErrSkip
		}

		capiCluster, capiClusterErr := h.capiClustersCache.Get(cluster.Namespace, cluster.Name)
		if capiClusterErr != nil && !apierrors.IsNotFound(capiClusterErr) {
			return "", capiClusterErr
		}

		if capiCluster != nil {
			if capiCluster.DeletionTimestamp == nil {
				// Deleting the CAPI cluster will start the process of deleting Machines, Bootstraps, etc.
				if err := h.capiClusters.Delete(capiCluster.Namespace, capiCluster.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					return "", err
				}
			}

			_, err := h.rkeControlPlanesCache.Get(cluster.Namespace, cluster.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				return "", err
			} else if err == nil {
				return "", generic.ErrSkip
			}
		}

		machines, err := h.capiMachinesCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name}))
		if err != nil {
			return "", err
		}

		// Machines will delete first so report their status, if any exist.
		if len(machines) > 0 {
			return capr.GetMachineDeletionStatus(machines)
		}

		if capiClusterErr == nil {
			return fmt.Sprintf("waiting for cluster-api cluster [%s] to delete", cluster.Name), nil
		}

		return "", h.kubeconfigManager.DeleteUser(cluster.Namespace, cluster.Name)
	}
}
