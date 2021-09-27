package cluster

import (
	"fmt"
	"sort"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/generic"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capierror "sigs.k8s.io/cluster-api/errors"
)

const (
	capiClusterLabel = "cluster.x-k8s.io/cluster-name"
)

var (
	Provisioned = condition.Cond("Provisioned")
	Waiting     = condition.Cond("Waiting")
	Pending     = condition.Cond("Pending")
	Removed     = condition.Cond("Removed")
)

func (h *handler) OnMgmtClusterRemove(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	provisioningClusters, err := h.clusterCache.GetByIndex(ByCluster, cluster.Name)
	if err != nil {
		return nil, err
	}

	var legacyCluster bool
	for _, provisioningCluster := range provisioningClusters {
		legacyCluster = legacyCluster || h.isLegacyCluster(provisioningCluster)
		if err := h.clusters.Delete(provisioningCluster.Namespace, provisioningCluster.Name, nil); err != nil {
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

func (h *handler) updateClusterStatus(cluster *v1.Cluster, status v1.ClusterStatus, previousErr error) (*v1.Cluster, error) {
	if equality.Semantic.DeepEqual(status, cluster.Status) {
		return cluster, previousErr
	}
	cluster = cluster.DeepCopy()
	cluster.Status = status
	cluster, err := h.clusters.UpdateStatus(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, previousErr
}

func (h *handler) OnClusterRemove(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	status := cluster.Status.DeepCopy()
	if !Provisioned.IsTrue(status) || !Waiting.IsTrue(status) || !Pending.IsTrue(status) {
		// Ensure the Removed status appears in the UI.
		Provisioned.SetStatus(status, "True")
		Waiting.SetStatus(status, "True")
		Pending.SetStatus(status, "True")
	}
	message, err := h.doClusterRemove(cluster)
	if err != nil {
		Removed.SetError(status, "", err)
	} else if message == "" {
		Removed.SetStatusBool(status, true)
		Removed.Reason(status, "")
		Removed.Message(status, "")
	} else {
		Removed.SetStatus(status, "Unknown")
		Removed.Reason(status, "Waiting")
		Removed.Message(status, message)
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, 5*time.Second)
		// generic.ErrSkip will mark the cluster as reconciled, but not remove the finalizer.
		// The finalizer shouldn't be removed until other objects have all been removed.
		err = generic.ErrSkip
	}
	return h.updateClusterStatus(cluster, *status, err)
}

func (h *handler) doClusterRemove(cluster *v1.Cluster) (string, error) {
	if cluster.Status.ClusterName != "" {
		mgmtCluster, err := h.mgmtClusters.Get(cluster.Status.ClusterName, metav1.GetOptions{})
		if err != nil {
			// We do nothing if the management cluster does not exist (IsNotFound) because it's been deleted.
			if !apierrors.IsNotFound(err) {
				return "", err
			}
		} else if cluster.Namespace == mgmtCluster.Spec.FleetWorkspaceName {
			// We only delete the management cluster if its FleetWorkspaceName matches the provisioning cluster's
			// namespace. The reason: if there's a mismatch, we know that the provisioning cluster needs to be migrated
			// because the user moved the Fleet cluster (and provisioning cluster, by extension) to another
			// FleetWorkspace. Ultimately, the aforementioned cluster objects are re-created in another namespace.
			err := h.mgmtClusters.Delete(cluster.Status.ClusterName, nil)
			if err != nil && !apierrors.IsNotFound(err) {
				return "", err
			}

			if h.isLegacyCluster(cluster) {
				// If this is a legacy cluster (i.e. RKE1 cluster) then we should wait to remove the provisioning cluster until the v3.Cluster is gone.
				_, err = h.mgmtClusterCache.Get(cluster.Status.ClusterName)
				if !apierrors.IsNotFound(err) {
					return fmt.Sprintf("waiting for cluster [%s] to delete", cluster.Status.ClusterName), nil
				}
			}
		}
	}

	capiCluster, capiClusterErr := h.capiClustersCache.Get(cluster.Namespace, cluster.Name)
	if capiClusterErr != nil && !apierrors.IsNotFound(capiClusterErr) {
		return "", capiClusterErr
	}

	if capiCluster != nil && capiCluster.DeletionTimestamp == nil {
		// Deleting the CAPI cluster will start the process of deleting Machines, Bootstraps, etc.
		if err := h.capiClusters.Delete(capiCluster.Namespace, capiCluster.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return "", err
		}
	}

	// Machines will delete first so report their status, if any exist.
	machines, err := h.capiMachinesCache.List(cluster.Namespace, labels.SelectorFromSet(labels.Set{
		capiClusterLabel: cluster.Name,
	}))
	if err != nil {
		return "", err
	}
	sort.Slice(machines, func(i, j int) bool {
		return machines[i].Name < machines[j].Name
	})
	for _, machine := range machines {
		if machine.Status.FailureReason != nil && *machine.Status.FailureReason == capierror.DeleteMachineError {
			return "", fmt.Errorf("error deleting machine [%s], machine must be deleted manually", machine.Name)
		}
		return fmt.Sprintf("waiting for machine [%s] to delete", machine.Name), nil
	}

	if capiClusterErr == nil {
		return fmt.Sprintf("waiting for cluster-api cluster [%s] to delete", cluster.Name), nil
	}

	return "", h.kubeconfigManager.DeleteUser(cluster.Namespace, cluster.Name)
}
