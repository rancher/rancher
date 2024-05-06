package restrictedadminrbac

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

// clusterOwnerSync ensures that the given enqueued GRB has a cluster-owner CRTB in all downstream clusters iff the role is GlobalRestrictedAdmin.
func (r *rbaccontroller) clusterOwnerSync(_ string, grb *v3.GlobalRoleBinding) (runtime.Object, error) {
	if grb == nil || grb.DeletionTimestamp != nil || grb.GlobalRoleName != rbac.GlobalRestrictedAdmin {
		return nil, nil
	}
	clusters, err := r.clusterCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	var retError error
	for _, cluster := range clusters {
		if cluster.Name == "local" {
			// do not sync to the local cluster
			continue
		}
		crtbName := name.SafeConcatName(rbac.GetGRBTargetKey(grb), "restricted-admin", "cluster-owner")
		_, err := r.crtbCache.Get(cluster.Name, crtbName)
		if err != nil && !apierrors.IsNotFound(err) {
			retError = multierror.Append(retError, fmt.Errorf("failed to get CRTB '%s' from cache: %w", crtbName, err))
			continue
		}
		if err == nil {
			// CRTB was already created.
			// we do not need to check for equivalence between the current CRTB and the desired CRTB
			// this is because the fields we care about can not be modified
			continue
		}

		// add the restricted admin user as a member of the downstream cluster
		// by creating a CRTB in the local custer in the namespace named after the downstream cluster.
		crtb := v3.ClusterRoleTemplateBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      crtbName,
				Namespace: cluster.Name,
				Labels:    map[string]string{sourceKey: grbHandlerName},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: grb.APIVersion,
						Kind:       grb.Kind,
						Name:       grb.Name,
						UID:        grb.UID,
					},
				},
			},
			ClusterName:      cluster.Name,
			RoleTemplateName: "cluster-owner",
		}

		// CRTBs must contain either user or group information but not both.
		// we will attempt to first use the userName then if not assign the groupName.
		if grb.UserName != "" {
			crtb.UserName = grb.UserName
		} else {
			crtb.GroupPrincipalName = grb.GroupPrincipalName
		}

		_, err = r.crtbCtrl.Create(&crtb)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			retError = multierror.Append(retError, fmt.Errorf("failed to create a CRTB '%s': %w", crtbName, err))
			continue
		}
	}
	return nil, retError
}

// enqueueGrbOnCRTB returns a resolver which provides the key to the GRBs that owns a given CRTB if one exist.
func (r *rbaccontroller) enqueueGrbOnCRTB(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	crtb, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok || crtb == nil {
		return nil, nil
	}

	var grbOwner []*metav1.OwnerReference
	for i := range crtb.OwnerReferences {
		ref := &crtb.OwnerReferences[i]
		if ref.Kind == "GlobalRoleBinding" && ref.APIVersion == v3.SchemeGroupVersion.String() {
			grbOwner = append(grbOwner, ref)
		}
	}

	if grbOwner == nil {
		// there are no owner references to GlobalRoleBindings
		return nil, nil
	}

	keys := make([]relatedresource.Key, 0, len(grbOwner))
	for _, owner := range grbOwner {
		grb, err := r.grbCache.Get(owner.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get owner reference '%s' from cache: %w", owner.Name, err)
		}
		keys = append(keys, relatedresource.Key{Name: grb.Name})
	}

	return keys, nil
}

// enqueueGrbOnCluster returns a resolver which provides the keys to all restrictedAdminGRBs.
func (r *rbaccontroller) enqueueGrbOnCluster(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	cluster, ok := obj.(*v3.Cluster)
	if !ok || cluster == nil || cluster.DeletionTimestamp != nil || cluster.Name == "local" {
		return nil, nil
	}
	return r.getRestrictedAdminGRBs()
}
