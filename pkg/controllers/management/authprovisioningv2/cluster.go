package authprovisioningv2

import (
	"fmt"
	"reflect"

	"github.com/rancher/kubernetes-provider-detector/providers"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	"github.com/rancher/rancher/pkg/rbac"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// OnCluster creates the roles required for users to be able to see/manage the
// provisioning cluster resource
func (h *handler) OnCluster(key string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	if cluster.DeletionTimestamp != nil {
		return cluster, h.cleanClusterAdminRoleBindings(cluster)
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

		// This is needed for creating RoleBindings when moving rke clusters to a different workspace.
		// This is only needed for rke because Role and RoleBindings are moved to the new workspace. In other k8s distros they stay in the fleet-default ns.
		if err = h.enqueueCRTBsForRKEClusters(cluster); err != nil {
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

func (h *handler) enqueueCRTBsForRKEClusters(cluster *v1.Cluster) error {
	if cluster.Labels[kubernetesprovider.ProviderKey] == providers.RKE {
		crtbs, err := h.clusterRoleTemplateBindings.List(cluster.Name, labels.Everything())
		if err != nil {
			return err
		}
		for _, crtb := range crtbs {
			h.clusterRoleTemplateBindingController.Enqueue(crtb.Namespace, crtb.Name)
		}
	}

	return nil
}
