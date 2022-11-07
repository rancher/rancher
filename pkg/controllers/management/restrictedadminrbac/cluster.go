package restrictedadminrbac

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	clusterv2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
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
	var grbList []*v3.GlobalRoleBinding
	for _, x := range grbs {
		grb, _ := x.(*v3.GlobalRoleBinding)
		grbList = append(grbList, grb)
		subject := rbac.GetGRBSubject(grb)
		subjects = append(subjects, subject)
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
			Subjects: []k8srbac.Subject{subject},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			returnErr = multierror.Append(returnErr, err)
		}
	}

	if returnErr != nil {
		return nil, returnErr
	}

	err = r.createCRAndCRBForRestrictedAdminClusterAccess(cluster, subjects)
	if err != nil {
		return nil, err
	}

	return nil, r.createRBForRestrictedAdminProvisioningClusterAccess(cluster, grbList)
}

/*
	createCRAndCRBForRestrictedAdminClusterAccess creates a CR with the resourceName field containing current cluster's ID. It also creates

a CRB for binding this CR to all the restricted admins. This way all restricted admins become owners of the cluster
*/
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

/*
	createRBForRestrictedAdminProvisioningClusterAccess creates for all restrictedAdmin users, a RB to the cluster-admin Role

created with the resourceName field containing the provisioning cluster's ID corresponding to the v3.cluster,
This way all restricted admins become owners of the provisioning cluster
*/
func (r *rbaccontroller) createRBForRestrictedAdminProvisioningClusterAccess(cluster *v3.Cluster, grbList []*v3.GlobalRoleBinding) error {
	var returnErr error
	clusterName := cluster.Name

	pClusters, err := r.provClusters.GetByIndex(clusterv2.ByCluster, clusterName)
	if err != nil {
		return err
	}

	if len(pClusters) == 0 {
		// When no provisioning cluster is found, enqueue this v3.cluster to wait for
		// the provisioning cluster to be created. If we don't try again
		// these rbac permissions for the restrictedAdmin users won't be created until an
		// update to the v3.cluster happens again.
		logrus.Debugf("No provisioning cluster found for cluster %v in rbac handler for restrictedAdmin, enqueuing", clusterName)
		r.clusters.Controller().EnqueueAfter(cluster.Namespace, clusterName, 10*time.Second)
		return nil
	}

	provCluster := pClusters[0]
	for _, grb := range grbList {
		// The roleBinding name format: r-cluster-<cluster name>-admin-<subject name>
		// Example: r-cluster-cluster1-admin-u-xyz
		subject := rbac.GetGRBSubject(grb)
		rbName := name.SafeConcatName(rbac.ProvisioningClusterAdminName(provCluster), rbac.GetGRBTargetKey(grb))
		existingRb, err := r.rbLister.Get(provCluster.Namespace, rbName)
		if err != nil && !k8serrors.IsNotFound(err) {
			returnErr = multierror.Append(returnErr, err)
		}
		if existingRb != nil {
			continue
		}
		_, err = r.roleBindings.Create(&k8srbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rbName,
				Namespace: provCluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: grb.APIVersion,
						Kind:       grb.Kind,
						Name:       grb.Name,
						UID:        grb.UID,
					},
				},
			},
			RoleRef: k8srbac.RoleRef{
				APIGroup: k8srbac.GroupName,
				Kind:     "Role",
				Name:     rbac.ProvisioningClusterAdminName(provCluster),
			},
			Subjects: []k8srbac.Subject{subject},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			returnErr = multierror.Append(returnErr, err)
		}
	}
	return returnErr
}
