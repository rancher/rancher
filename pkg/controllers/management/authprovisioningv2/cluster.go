package authprovisioningv2

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/rbac"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	capiResourcesCleanupFinalizer = "auth.cattle.io/capi-resources-cleanup"

	// removedWaitingReason is the Removed condition reason used while a removal is waiting on
	// dependents. It matches the reason capr.DoRemoveAndUpdateStatus uses so that the two
	// writers of this condition read as one.
	removedWaitingReason = "Waiting"

	// removedWaitingMessagePrefix prefixes the Removed condition message this handler writes. It
	// is how clearRemovedWaiting recognises its own message and avoids clobbering a Removed
	// condition that the provisioning cluster remove handler owns.
	removedWaitingMessagePrefix = "waiting for cluster-indexed resources to be deleted"

	// maxBlockingResourcesInMessage caps how many resource references are named in the Removed
	// condition message, so that a cluster with many leftovers doesn't produce an unbounded status.
	maxBlockingResourcesInMessage = 10
)

// OnCluster creates the roles required for users to be able to see/manage the
// provisioning cluster resource. It also manages a finalizer to ensure that
// cluster-indexed resources (e.g. AWSMachineTemplate) can be cleaned up by users
// before the scoped RBAC roles are garbage-collected along with the cluster.
func (h *handler) OnCluster(key string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	// Ensure the finalizer is present on all non-deleting provisioning clusters
	if cluster.DeletionTimestamp == nil && !slices.Contains(cluster.Finalizers, capiResourcesCleanupFinalizer) {
		clusterCopy := cluster.DeepCopy()
		clusterCopy.Finalizers = append(clusterCopy.Finalizers, capiResourcesCleanupFinalizer)
		return h.clusterController.Update(clusterCopy)
	}

	if cluster.DeletionTimestamp != nil {
		// Cluster is being deleted — check if any cluster-indexed resources still exist
		blocking, err := h.blockingClusterIndexedResources(cluster)
		if err != nil {
			return cluster, err
		}
		if len(blocking) > 0 {
			// Record what is holding the deletion up so it can be diagnosed from the
			// cluster object, then re-enqueue to keep checking; this keeps the crt-*
			// Role alive via the finalizer blocking GC on the cluster object
			cluster, err = h.setRemovedWaiting(cluster, blocking)
			if err != nil {
				return cluster, err
			}
			h.clusterController.EnqueueAfter(cluster.Namespace, cluster.Name, reenqueueTime)
			return cluster, nil
		}

		// No cluster-indexed resources remain — safe to proceed with deletion
		if err := h.cleanClusterAdminRoleBindings(cluster); err != nil {
			return cluster, err
		}

		if cluster, err = h.clearRemovedWaiting(cluster); err != nil {
			return cluster, err
		}

		// Remove the finalizer to unblock GC
		if slices.Contains(cluster.Finalizers, capiResourcesCleanupFinalizer) {
			clusterCopy := cluster.DeepCopy()
			clusterCopy.Finalizers = slices.DeleteFunc(clusterCopy.Finalizers,
				func(f string) bool { return f == capiResourcesCleanupFinalizer })
			return h.clusterController.Update(clusterCopy)
		}
	}

	return cluster, h.createClusterViewRole(cluster)
}

func (h *handler) createClusterViewRole(cluster *v1.Cluster) error {
	roleName := clusterViewName(cluster)
	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{cluster.GroupVersionKind().Group},
				Resources:     []string{"clusters"},
				ResourceNames: []string{cluster.Name},
				Verbs:         []string{"get"},
			},
		},
	}

	existingRole, err := h.roleCache.Get(cluster.Namespace, roleName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}

		if _, err := h.roleController.Create(role); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}

		// This is needed for creating RoleBindings when moving clusters to a different workspace.
		if err = h.enqueueRoleTemplateBindings(cluster); err != nil {
			return err
		}
		return nil
	}

	if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
		existingRole = existingRole.DeepCopy()
		existingRole.Rules = role.Rules
		_, err := h.roleController.Update(existingRole)
		return err
	}

	return nil
}

func (h *handler) cleanClusterAdminRoleBindings(cluster *v1.Cluster) error {
	// Collect all the errors to delete as many rolebindings as possible
	var allErrors []error

	roleName := rbac.ProvisioningClusterAdminName(cluster)
	rbList, err := h.roleBindingController.List(cluster.Namespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, roleBinding := range rbList.Items {
		if roleBinding.RoleRef.Kind == "Role" && roleBinding.RoleRef.Name == roleName {
			err = h.roleBindingController.Delete(roleBinding.Namespace, roleBinding.Name, &metav1.DeleteOptions{})
			if err != nil {
				// Continue if this RoleBinding doesn't exist
				if !k8serrors.IsNotFound(err) {
					allErrors = append(allErrors, err)
				}
				continue
			}
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("errors deleting cluster admin role binding: %v", allErrors)
	}
	return nil
}

func (h *handler) enqueueRoleTemplateBindings(cluster *v1.Cluster) error {
	crtbs, err := h.clusterRoleTemplateBindings.List(cluster.Name, labels.Everything())
	if err != nil {
		return err
	}
	for _, crtb := range crtbs {
		h.clusterRoleTemplateBindingController.Enqueue(crtb.Namespace, crtb.Name)
	}

	prtbs, err := h.projectRoleTemplateBindings.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, prtb := range prtbs {
		clusterName := strings.Split(prtb.ProjectName, ":")[0]
		if clusterName == cluster.Name {
			h.projectRoleTemplateBindingController.Enqueue(prtb.Namespace, prtb.Name)
		}
	}

	return nil
}

// blockingClusterIndexedResources returns a sorted list of references to the cluster-indexed
// resources (across all registered GVKs) which still exist for the given cluster and which the
// cluster does not own.
//
// Resources owned by the cluster are excluded because Kubernetes garbage collection deletes them
// as soon as the cluster object itself is removed, which cannot happen while this handler's
// finalizer is in place. Waiting on them would deadlock: the finalizer blocks the GC that would
// satisfy the finalizer. S3 ETCDSnapshots and rke machine configs are both owned by the
// provisioning cluster and hit exactly this case.
//
// Ownership is matched on UID rather than name so that a cluster which was deleted and recreated
// under the same name does not adopt the previous cluster's leftovers.
func (h *handler) blockingClusterIndexedResources(cluster *v1.Cluster) ([]string, error) {
	var blocking []string
	for _, candidate := range h.candidateTypes() {
		// Skip the provisioning cluster GVK itself — the cluster being deleted
		// is still in the index (blocked by our finalizer), which would cause
		// a self-referential deadlock preventing the finalizer from ever being removed.
		if candidate.GVK == h.provisioningClusterGVK {
			continue
		}
		objs, err := h.indexGetter.GetByIndex(candidate.GVK, clusterIndexed, clusterIndexKey(cluster))
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			objMeta, err := meta.Accessor(obj)
			if err != nil {
				return nil, err
			}
			if ownedBy(objMeta, cluster.UID) {
				continue
			}
			blocking = append(blocking, fmt.Sprintf("%s %s/%s", candidate.GVK.Kind, objMeta.GetNamespace(), objMeta.GetName()))
		}
	}
	sort.Strings(blocking)
	return blocking, nil
}

// ownedBy reports whether obj has an owner reference to the object with the given UID.
func ownedBy(obj metav1.Object, ownerUID types.UID) bool {
	return slices.ContainsFunc(obj.GetOwnerReferences(), func(ref metav1.OwnerReference) bool {
		return ref.UID == ownerUID
	})
}

// setRemovedWaiting records on the Removed condition which resources are blocking the cluster's
// deletion, in the same Unknown/Waiting shape capr.DoRemoveAndUpdateStatus uses when a removal is
// waiting on dependents. It is a no-op if the condition already says the same thing, so that the
// deletion re-enqueue loop does not write to the API server every reenqueueTime.
func (h *handler) setRemovedWaiting(cluster *v1.Cluster, blocking []string) (*v1.Cluster, error) {
	message := blockingResourcesMessage(blocking)
	if capr.Removed.IsUnknown(cluster) &&
		capr.Removed.GetReason(cluster) == removedWaitingReason &&
		capr.Removed.GetMessage(cluster) == message {
		return cluster, nil
	}

	clusterCopy := cluster.DeepCopy()
	capr.Removed.SetStatus(clusterCopy, "Unknown")
	capr.Removed.Reason(clusterCopy, removedWaitingReason)
	capr.Removed.Message(clusterCopy, message)

	updated, err := h.clusterController.UpdateStatus(clusterCopy)
	if err != nil {
		return cluster, err
	}
	return updated, nil
}

// clearRemovedWaiting reverts a Removed condition this handler parked on Unknown, so a cluster
// which lingers on some other controller's finalizer doesn't keep reporting a blockage that no
// longer exists. It only acts on a message it wrote itself, leaving the provisioning cluster
// remove handler's own Removed bookkeeping alone.
func (h *handler) clearRemovedWaiting(cluster *v1.Cluster) (*v1.Cluster, error) {
	if !strings.HasPrefix(capr.Removed.GetMessage(cluster), removedWaitingMessagePrefix) {
		return cluster, nil
	}

	clusterCopy := cluster.DeepCopy()
	capr.Removed.SetStatusBool(clusterCopy, true)
	capr.Removed.Reason(clusterCopy, "")
	capr.Removed.Message(clusterCopy, "")

	updated, err := h.clusterController.UpdateStatus(clusterCopy)
	if err != nil {
		return cluster, err
	}
	return updated, nil
}

func blockingResourcesMessage(blocking []string) string {
	shown, extra := blocking, ""
	if len(shown) > maxBlockingResourcesInMessage {
		shown = shown[:maxBlockingResourcesInMessage]
		extra = fmt.Sprintf(" and %d more", len(blocking)-maxBlockingResourcesInMessage)
	}
	return fmt.Sprintf("%s: %s%s", removedWaitingMessagePrefix, strings.Join(shown, ", "), extra)
}
