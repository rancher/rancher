package authprovisioningv2

import (
	"reflect"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OnCluster creates the role required for users assigned through PRTBs to be able to see the
// provisioning cluster resource
func (h *handler) OnCluster(key string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

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
			return nil, err
		}

		if _, err := h.roleController.Create(role); err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		return cluster, nil
	}

	if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
		existingRole = existingRole.DeepCopy()
		existingRole.Rules = role.Rules
		_, err := h.roleController.Update(existingRole)
		return nil, err
	}
	return cluster, nil
}
