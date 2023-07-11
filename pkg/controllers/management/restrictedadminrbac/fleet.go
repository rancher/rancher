package restrictedadminrbac

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/pkg/relatedresource"
	k8srbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *rbaccontroller) enqueueGrb(namespace, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if fw, ok := obj.(*v3.FleetWorkspace); !ok || fw.Name == fleetconst.ClustersLocalNamespace {
		return nil, nil
	}

	return r.getRestrictedAdminGRBs()
}

func (r *rbaccontroller) ensureRestricedAdminForFleet(key string, obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.GlobalRoleName != rbac.GlobalRestrictedAdmin {
		return obj, nil
	}

	fleetworkspaces, err := r.fleetworkspaces.Controller().Lister().List("", labels.Everything())
	if err != nil {
		return obj, nil
	}

	var finalError error
	for _, fw := range fleetworkspaces {
		if fw.Name == fleetconst.ClustersLocalNamespace {
			continue
		}
		if err := r.ensureRolebinding(fw.Name, rbac.GetGRBSubject(obj), obj); err != nil {
			finalError = multierror.Append(finalError, err)
		}
	}
	return obj, finalError
}

func (r *rbaccontroller) ensureRolebinding(namespace string, subject k8srbac.Subject, grb *v3.GlobalRoleBinding) error {
	rbName := fmt.Sprintf("%s-fleetworkspace-%s", grb.Name, rbac.RestrictedAdminClusterRoleBinding)
	rb := &k8srbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: namespace,
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
			Name: "fleetworkspace-admin",
			Kind: "ClusterRole",
		},
		Subjects: []k8srbac.Subject{
			subject,
		},
	}
	_, err := r.rbLister.Get(namespace, rbName)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	} else if k8serrors.IsNotFound(err) {
		_, err = r.roleBindings.Create(rb)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	} else {
		_, err := r.roleBindings.Update(rb)
		if err != nil {
			return err
		}
	}
	return nil
}
