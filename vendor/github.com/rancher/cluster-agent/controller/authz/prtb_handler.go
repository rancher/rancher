package authz

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (r *roleHandler) syncPRTB(key string, binding *v3.ProjectRoleTemplateBinding) error {
	if binding == nil {
		return nil
	}

	if binding.DeletionTimestamp != nil {
		return r.ensurePRTBDelete(key, binding)
	}

	return r.ensurePRTB(key, binding)
}

func (r *roleHandler) ensurePRTB(key string, binding *v3.ProjectRoleTemplateBinding) error {
	binding = binding.DeepCopy()
	added := r.addFinalizer(binding)
	if added {
		if _, err := r.workload.Management.Management.ProjectRoleTemplateBindings(binding.Namespace).Update(binding); err != nil {
			return errors.Wrapf(err, "couldn't set finalizer on %v", key)
		}
	}

	if binding.RoleTemplateName == "" {
		logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		return nil
	}
	if binding.Subject.Name == "" {
		logrus.Warnf("Binding %v has no subject. Skipping", binding.Name)
		return nil
	}

	rt, err := r.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return errors.Wrapf(err, "couldn't get role template %v", binding.RoleTemplateName)
	}

	// Get namespaces belonging to project
	namespaces, err := r.nsIndexer.ByIndex(nsIndex, binding.ProjectName)
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with project ID %v", binding.ProjectName)
	}
	if len(namespaces) == 0 {
		return nil
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := r.gatherRoles(rt, roles); err != nil {
		return err
	}

	if err := r.ensureRoles(roles); err != nil {
		return errors.Wrap(err, "couldn't ensure roles")
	}

	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		for _, role := range roles {
			if err := r.ensureBinding(ns.Name, role.Name, binding); err != nil {
				return errors.Wrapf(err, "couldn't ensure binding %v %v in %v", role.Name, binding.Subject.Name, ns.Name)
			}
		}
	}

	return nil
}

func (r *roleHandler) ensureBinding(ns, roleName string, binding *v3.ProjectRoleTemplateBinding) error {
	bindingCli := r.workload.K8sClient.RbacV1().RoleBindings(ns)
	bindingName, objectMeta, subjects, roleRef := bindingParts(roleName, string(binding.UID), binding.Subject)
	if b, _ := r.rbLister.Get(ns, bindingName); b != nil {
		return nil
	}
	_, err := bindingCli.Create(&rbacv1.RoleBinding{
		ObjectMeta: objectMeta,
		Subjects:   subjects,
		RoleRef:    roleRef,
	})
	return err
}

func (r *roleHandler) ensurePRTBDelete(key string, binding *v3.ProjectRoleTemplateBinding) error {
	if len(binding.ObjectMeta.Finalizers) <= 0 || binding.ObjectMeta.Finalizers[0] != r.finalizerName {
		return nil
	}

	binding = binding.DeepCopy()

	// Get namespaces belonging to project
	namespaces, err := r.nsIndexer.ByIndex(nsIndex, binding.ProjectName)
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with project ID %v", binding.ProjectName)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: string(binding.UID)})
	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		bindingCli := r.workload.K8sClient.RbacV1().RoleBindings(ns.Name)
		rbs, err := r.rbLister.List(ns.Name, set.AsSelector())
		if err != nil {
			return errors.Wrapf(err, "couldn't list rolebindings with selector %s", set.AsSelector())
		}

		for _, rb := range rbs {
			if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return errors.Wrapf(err, "error deleting rolebinding %v", rb.Name)
				}
			}
		}
	}

	if r.removeFinalizer(binding) {
		_, err := r.workload.Management.Management.ProjectRoleTemplateBindings(binding.Namespace).Update(binding)
		return err
	}
	return nil
}
