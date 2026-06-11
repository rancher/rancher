package authprovisioningv2

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/capr/dynamicschema"
	"k8s.io/apimachinery/pkg/labels"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const capiResourcesCleanupFinalizer = "auth.cattle.io/capi-resources-cleanup"

// OnCluster creates the roles required for users to be able to see/manage the
// provisioning cluster resource. It also manages a finalizer to ensure that
// cluster-indexed resources (e.g. AWSMachineTemplate) can be cleaned up by users
// before the scoped RBAC roles are garbage-collected along with the cluster.
func (h *handler) OnCluster(key string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	// Ensure the finalizer is present on all provisioning clusters
	if !slices.Contains(cluster.Finalizers, capiResourcesCleanupFinalizer) {
		clusterCopy := cluster.DeepCopy()
		clusterCopy.Finalizers = append(clusterCopy.Finalizers, capiResourcesCleanupFinalizer)
		return h.clusterController.Update(clusterCopy)
	}

	if cluster.DeletionTimestamp != nil {
		// Cluster is being deleted — check if any cluster-indexed resources still exist
		hasResources, err := h.clusterIndexedResourcesExist(cluster)
		if err != nil {
			return cluster, err
		}
		if hasResources {
			// Re-enqueue to keep checking; this keeps the crt-* Role alive
			// via the finalizer blocking GC on the cluster object
			h.clusterController.EnqueueAfter(cluster.Namespace, cluster.Name, reenqueueTime)
			return cluster, nil
		}

		// No cluster-indexed resources remain — safe to proceed with deletion
		if err := h.cleanClusterAdminRoleBindings(cluster); err != nil {
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

// clusterIndexedResourcesExist returns true if any cluster-indexed resources
// (across all registered GVKs) still exist for the given cluster.
func (h *handler) clusterIndexedResourcesExist(cluster *v1.Cluster) (bool, error) {
	for _, candidate := range h.candidateTypes() {
		// Skip the provisioning cluster GVK itself — the cluster being deleted
		// is still in the index (blocked by our finalizer), which would cause
		// a self-referential deadlock preventing the finalizer from ever being removed.
		if candidate.GVK == h.provisioningClusterGVK {
			continue
		}
		// Skip the rke machine config whose owner is the provisioning cluster
		if candidate.GVK.Group == dynamicschema.MachineConfigAPIGroup {
			continue
		}
		names, err := getResourceNames(h.dynamic, candidate, cluster)
		if err != nil {
			return false, err
		}
		if len(names) > 0 {
			return true, nil
		}
	}
	return false, nil
}
