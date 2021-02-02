package restrictedadminrbac

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	k8srbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *rbaccontroller) clusterRBACSync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	// restricted-admin should not be granted admin access to the local cluster, it only needs access to downstream clusters
	if cluster.Name == "local" {
		return nil, nil
	}

	grbs, err := r.grbIndexer.ByIndex(grbByRoleIndex, rbac.GlobalRestrictedAdmin)
	if err != nil {
		return nil, err
	}

	var subjects []k8srbac.Subject
	var returnErr error
	for _, x := range grbs {
		grb, _ := x.(*v3.GlobalRoleBinding)
		restrictedAdminUserName := grb.UserName
		subjects = append(subjects, k8srbac.Subject{
			Kind: "User",
			Name: restrictedAdminUserName,
		})
		rbName := fmt.Sprintf("%s-%s", grb.Name, rbac.RestrictedAdminClusterRoleBinding)
		rb, err := r.rbLister.Get(cluster.Name, rbName)
		if err != nil && !k8serrors.IsNotFound(err) {
			returnErr = multierror.Append(returnErr, err)
			continue
		}
		if rb != nil {
			continue
		}
		_, err = r.roleBindings.Create(&k8srbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rbName,
				Namespace: cluster.Name,
				Labels:    map[string]string{rbac.RestrictedAdminClusterRoleBinding: "true"},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: grb.TypeMeta.APIVersion,
						Kind:       grb.TypeMeta.Kind,
						UID:        grb.UID,
						Name:       grb.Name,
					},
				},
			},
			RoleRef: k8srbac.RoleRef{
				Name: rbac.ClusterCRDsClusterRole,
				Kind: "ClusterRole",
			},
			Subjects: []k8srbac.Subject{
				{
					Kind: "User",
					Name: restrictedAdminUserName,
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			returnErr = multierror.Append(returnErr, err)
		}
	}

	if returnErr != nil {
		return nil, returnErr
	}

	return nil, r.createCRAndCRBForRestrictedAdminClusterAccess(cluster, subjects)
}

/* createCRAndCRBForRestrictedAdminClusterAccess creates a CR with the resourceName field containing current cluster's ID. It also creates
a CRB for binding this CR to all the restricted admins. This way all restricted admins become owners of the cluster*/
func (r *rbaccontroller) createCRAndCRBForRestrictedAdminClusterAccess(cluster *v3.Cluster, subjects []k8srbac.Subject) error {
	var returnErr error

	crName := fmt.Sprintf("%s-%s", cluster.Name, rbac.RestrictedAdminCRForClusters)
	_, err := r.crLister.Get("", crName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		cr := k8srbac.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   crName,
				Labels: map[string]string{rbac.RestrictedAdminCRForClusters: cluster.Name},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: cluster.TypeMeta.APIVersion,
						Kind:       cluster.TypeMeta.Kind,
						UID:        cluster.UID,
						Name:       cluster.Name,
					},
				},
			},
			Rules: []k8srbac.PolicyRule{
				{
					APIGroups:     []string{"management.cattle.io"},
					Resources:     []string{"clusters"},
					ResourceNames: []string{cluster.Name},
					Verbs:         []string{"*"},
				},
			},
		}
		_, err := r.clusterRoles.Create(&cr)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}

		crbNamePrefix := fmt.Sprintf("%s-%s", cluster.Name, rbac.RestrictedAdminCRBForClusters)
		for _, subject := range subjects {
			crbName := fmt.Sprintf("%s-%s", crbNamePrefix, subject.Name)
			existingCrb, err := r.crbLister.Get("", crbName)
			if err != nil && !k8serrors.IsNotFound(err) {
				returnErr = multierror.Append(returnErr, err)
			}
			if existingCrb != nil {
				continue
			}
			crb := k8srbac.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   crbName,
					Labels: map[string]string{rbac.RestrictedAdminCRBForClusters: cluster.Name},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cluster.TypeMeta.APIVersion,
							Kind:       cluster.TypeMeta.Kind,
							UID:        cluster.UID,
							Name:       cluster.Name,
						},
					},
				},
				RoleRef: k8srbac.RoleRef{
					Kind: "ClusterRole",
					Name: crName,
				},
				Subjects: []k8srbac.Subject{subject},
			}

			_, err = r.clusterRoleBindings.Create(&crb)
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				returnErr = multierror.Append(returnErr, err)
			}
		}
	}
	return returnErr
}
