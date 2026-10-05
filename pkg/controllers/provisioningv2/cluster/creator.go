package cluster

import (
	"fmt"
	"slices"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A provisioning cluster and its management cluster are created from one another, in one of two
// directions: a v2prov provisioning cluster creates its management cluster, and an imported or hosted
// management cluster creates its provisioning cluster. The object that created the other carries a
// finalizer, added before it creates the other, and removes the object it created when it is deleted.
// The created object never waits for its creator; deleted on its own, it only asks for its creator to be
// deleted.
const (
	// removeManagementClusterFinalizer is set on a provisioning cluster that created its management cluster.
	removeManagementClusterFinalizer = "provisioning.cattle.io/remove-management-cluster"
	// removeProvisioningClusterFinalizer is set on a management cluster that created its provisioning cluster.
	removeProvisioningClusterFinalizer = "management.cattle.io/remove-provisioning-cluster"

	creatorRequeue = 5 * time.Second
)

// errFinalizerAdded is returned by a generating handler that has just added its creator finalizer, so that
// nothing is applied yet. The update requeues the object, and the next run creates the other object.
var errFinalizerAdded = generic.ErrSkip

// ensureProvisioningClusterFinalizer adds removeManagementClusterFinalizer to cluster, and reports whether
// it had to. A provisioning cluster must carry it before it creates its management cluster.
func (h *handler) ensureProvisioningClusterFinalizer(cluster *v1.Cluster) (bool, error) {
	if slices.Contains(cluster.Finalizers, removeManagementClusterFinalizer) {
		return false, nil
	}
	cluster = cluster.DeepCopy()
	cluster.Finalizers = append(cluster.Finalizers, removeManagementClusterFinalizer)
	if _, err := h.clusters.Update(cluster); err != nil {
		return false, fmt.Errorf("adding finalizer %s to provisioning cluster %s/%s: %w", removeManagementClusterFinalizer, cluster.Namespace, cluster.Name, err)
	}
	return true, nil
}

// ensureManagementClusterFinalizer adds removeProvisioningClusterFinalizer to cluster, and reports whether
// it had to. A management cluster must carry it before it creates its provisioning cluster.
func (h *handler) ensureManagementClusterFinalizer(cluster *v3.Cluster) (bool, error) {
	if slices.Contains(cluster.Finalizers, removeProvisioningClusterFinalizer) {
		return false, nil
	}
	cluster = cluster.DeepCopy()
	cluster.Finalizers = append(cluster.Finalizers, removeProvisioningClusterFinalizer)
	if _, err := h.mgmtClusters.Update(cluster); err != nil {
		return false, fmt.Errorf("adding finalizer %s to management cluster %s: %w", removeProvisioningClusterFinalizer, cluster.Name, err)
	}
	return true, nil
}

// checkManagementClusterAdoptable returns an error if the management cluster named name exists and must
// not be taken over by cluster: it is being deleted, or it was created by a different provisioning
// cluster. A management cluster that wasn't created by any provisioning cluster can still be adopted.
func (h *handler) checkManagementClusterAdoptable(cluster *v1.Cluster, name string) error {
	if name == "" {
		return nil
	}
	existing, err := h.mgmtClusterCache.Get(name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing.DeletionTimestamp != nil {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, creatorRequeue)
		return fmt.Errorf("waiting for management cluster %s to be removed before creating it again", name)
	}
	if namespace, owner, ok := util.CreatedByProvisioningCluster(existing); ok && (namespace != cluster.Namespace || owner != cluster.Name) {
		return fmt.Errorf("management cluster %s belongs to provisioning cluster %s/%s", name, namespace, owner)
	}
	return nil
}

// onProvisioningClusterRemove removes the management cluster a provisioning cluster created, once the
// provisioning cluster's CAPI cluster and machines are gone, and lets the provisioning cluster go once
// the management cluster is gone.
func (h *handler) onProvisioningClusterRemove(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp == nil || !slices.Contains(cluster.Finalizers, removeManagementClusterFinalizer) {
		return cluster, nil
	}

	// The CAPI cluster and machines are torn down by OnClusterRemove. Machines are drained through the
	// management cluster's tunnel, so the management cluster has to stay until they are gone.
	if slices.Contains(cluster.Finalizers, util.ProvisioningClusterRemoveFinalizer) {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, creatorRequeue)
		return cluster, nil
	}

	if gone, err := h.removeCreatedManagementCluster(cluster); err != nil || !gone {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, creatorRequeue)
		return cluster, err
	}

	cluster = cluster.DeepCopy()
	cluster.Finalizers = slices.DeleteFunc(cluster.Finalizers, func(f string) bool { return f == removeManagementClusterFinalizer })
	return h.clusters.Update(cluster)
}

// removeCreatedManagementCluster deletes the management cluster cluster created, and reports whether it is
// gone. A management cluster that cluster didn't create, or that moved to another fleet workspace, is left
// alone.
func (h *handler) removeCreatedManagementCluster(cluster *v1.Cluster) (bool, error) {
	if cluster.Status.ClusterName == "" {
		return true, nil
	}
	mgmtCluster, err := h.mgmtClusterCache.Get(cluster.Status.ClusterName)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !util.CreatedBy(mgmtCluster, util.ProvisioningClusterGVK, cluster.Namespace, cluster.Name) || cluster.Namespace != mgmtCluster.Spec.FleetWorkspaceName {
		return true, nil
	}
	if mgmtCluster.DeletionTimestamp == nil {
		logrus.Infof("[provisioningcluster] deleting management cluster %s created by provisioning cluster %s/%s", mgmtCluster.Name, cluster.Namespace, cluster.Name)
		uid := mgmtCluster.UID
		err := h.mgmtClusters.Delete(mgmtCluster.Name, &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return false, err
		}
	}
	return false, nil
}

// onManagementClusterRemove removes the provisioning cluster a management cluster created, and lets the
// management cluster go once the provisioning cluster is gone.
func (h *handler) onManagementClusterRemove(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp == nil || !slices.Contains(cluster.Finalizers, removeProvisioningClusterFinalizer) {
		return cluster, nil
	}

	if gone, err := h.removeCreatedProvisioningCluster(cluster); err != nil || !gone {
		h.mgmtClusters.EnqueueAfter(cluster.Name, creatorRequeue)
		return cluster, err
	}

	cluster = cluster.DeepCopy()
	cluster.Finalizers = slices.DeleteFunc(cluster.Finalizers, func(f string) bool { return f == removeProvisioningClusterFinalizer })
	return h.mgmtClusters.Update(cluster)
}

// removeCreatedProvisioningCluster deletes the provisioning cluster cluster created, and reports whether it
// is gone. A provisioning cluster that cluster didn't create is left alone.
func (h *handler) removeCreatedProvisioningCluster(cluster *v3.Cluster) (bool, error) {
	if cluster.Spec.FleetWorkspaceName == "" {
		return true, nil
	}
	provCluster, err := h.clusterCache.Get(cluster.Spec.FleetWorkspaceName, cluster.Name)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !util.CreatedBy(provCluster, util.ManagementClusterGVK, "", cluster.Name) {
		return true, nil
	}
	if provCluster.DeletionTimestamp == nil {
		logrus.Infof("[provisioningcluster] deleting provisioning cluster %s/%s created by management cluster %s", provCluster.Namespace, provCluster.Name, cluster.Name)
		uid := provCluster.UID
		err := h.clusters.Delete(provCluster.Namespace, provCluster.Name, &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return false, err
		}
	}
	return false, nil
}
