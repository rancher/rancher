package restrictedadminrbac

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
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
	name := fmt.Sprintf("%s-fleetworkspace-%s", grb.Name, rbac.RestrictedAdminClusterRoleBinding)
	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion: grb.TypeMeta.APIVersion,
			Kind:       grb.TypeMeta.Kind,
			UID:        grb.UID,
			Name:       grb.Name,
		},
	}
	roleRef := k8srbac.RoleRef{
		Name: "fleetworkspace-admin",
		Kind: "ClusterRole",
	}
	subjects := []k8srbac.Subject{
		subject,
	}

	rb, err := r.rbLister.Get(namespace, name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			// list call failed for unknown reason, give up
			return fmt.Errorf("unable to list backing role %s/%s: %w", namespace, name, err)
		}

		// role binding not found, create it
		_, err = r.roleBindings.Create(&k8srbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       namespace,
				Labels:          map[string]string{rbac.RestrictedAdminClusterRoleBinding: "true"},
				OwnerReferences: ownerRefs,
			},
			RoleRef:  roleRef,
			Subjects: subjects,
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create backing role %s/%s: %w", namespace, name, err)
		}
		return nil
	}

	// role binding found, possibly in dirty state. Make sure relevant fields are set right
	label, ok := rb.Labels[rbac.RestrictedAdminClusterRoleBinding]
	if !ok || label != "true" ||
		!reflect.DeepEqual(rb.OwnerReferences, ownerRefs) ||
		!reflect.DeepEqual(rb.RoleRef, roleRef) ||
		!reflect.DeepEqual(rb.Subjects, subjects) {

		rb.Labels[rbac.RestrictedAdminClusterRoleBinding] = "true"
		rb.OwnerReferences = ownerRefs
		rb.RoleRef = roleRef
		rb.Subjects = subjects

		_, err = r.roleBindings.Update(rb)
		return err
	}
	return nil
}
