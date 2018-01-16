package authz

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *roleHandler) syncRT(key string, template *v3.RoleTemplate) error {
	if template == nil {
		return nil
	}

	if template.DeletionTimestamp != nil {
		return r.ensureRTDelete(key, template)
	}

	return r.ensureRT(key, template)
}

func (r *roleHandler) ensureRT(key string, template *v3.RoleTemplate) error {
	template = template.DeepCopy()
	if r.addFinalizer(template) {
		if _, err := r.workload.Management.Management.RoleTemplates("").Update(template); err != nil {
			return errors.Wrapf(err, "couldn't set finalizer on %v", key)
		}
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := r.gatherRoles(template, roles); err != nil {
		return err
	}

	if err := r.ensureRoles(roles); err != nil {
		return errors.Wrapf(err, "couldn't ensure roles")
	}

	return nil
}

func (r *roleHandler) ensureRTDelete(key string, template *v3.RoleTemplate) error {
	if len(template.ObjectMeta.Finalizers) <= 0 || template.ObjectMeta.Finalizers[0] != r.finalizerName {
		return nil
	}

	template = template.DeepCopy()

	roles := map[string]*v3.RoleTemplate{}
	if err := r.gatherRoles(template, roles); err != nil {
		return err
	}

	roleCli := r.workload.K8sClient.RbacV1().ClusterRoles()
	for _, role := range roles {
		if err := roleCli.Delete(role.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "error deleting clusterrole %v", role.Name)
			}
		}
	}

	if r.removeFinalizer(template) {
		_, err := r.workload.Management.Management.RoleTemplates("").Update(template)
		return err
	}

	return nil
}
