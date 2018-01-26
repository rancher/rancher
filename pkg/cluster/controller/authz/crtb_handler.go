package authz

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func newCRTBLifecycle(m *manager) *crtbLifecycle {
	return &crtbLifecycle{m: m}
}

type crtbLifecycle struct {
	m *manager
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	err := c.syncCRTB(obj)
	return obj, err
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	err := c.syncCRTB(obj)
	return obj, err
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	err := c.ensureCRTBDelete(obj)
	return obj, err
}

func (c *crtbLifecycle) syncCRTB(binding *v3.ClusterRoleTemplateBinding) error {
	if binding.RoleTemplateName == "" {
		logrus.Warnf("ClusterRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		return nil
	}
	if binding.UserName == "" {
		logrus.Warnf("Binding %v has no subject. Skipping", binding.Name)
		return nil
	}

	rt, err := c.m.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return errors.Wrapf(err, "couldn't get role template %v", binding.RoleTemplateName)
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(rt, roles); err != nil {
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		return errors.Wrap(err, "couldn't ensure roles")
	}

	if err := c.ensureClusterBinding(roles, binding); err != nil {
		return errors.Wrapf(err, "couldn't ensure cluster bindings for %v", binding.UserName)
	}

	return nil
}

func (c *crtbLifecycle) ensureClusterBinding(roles map[string]*v3.RoleTemplate, binding *v3.ClusterRoleTemplateBinding) error {
	roleBindings := c.m.workload.K8sClient.RbacV1().ClusterRoleBindings()

	set := labels.Set(map[string]string{rtbOwnerLabel: string(binding.UID)})
	desiredRBs := map[string]*rbacv1.ClusterRoleBinding{}
	subject := buildSubjectFromCRTB(binding)
	for roleName := range roles {
		bindingName, objectMeta, subjects, roleRef := bindingParts(roleName, string(binding.UID), subject)
		desiredRBs[bindingName] = &rbacv1.ClusterRoleBinding{
			ObjectMeta: objectMeta,
			Subjects:   subjects,
			RoleRef:    roleRef,
		}
	}

	currentRBs, err := c.m.crbLister.List("", set.AsSelector())
	if err != nil {
		return err
	}
	rbsToDelete := map[string]bool{}
	processed := map[string]bool{}
	for _, rb := range currentRBs {
		// protect against an rb being in the list more than once (shouldn't happen, but just to be safe)
		if ok := processed[rb.Name]; ok {
			continue
		}
		processed[rb.Name] = true

		if _, ok := desiredRBs[rb.Name]; ok {
			delete(desiredRBs, rb.Name)
		} else {
			rbsToDelete[rb.Name] = true
		}
	}

	for _, rb := range desiredRBs {
		_, err := roleBindings.Create(rb)
		if err != nil {
			return err
		}
	}

	for name := range rbsToDelete {
		if err := roleBindings.Delete(name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (c *crtbLifecycle) ensureCRTBDelete(binding *v3.ClusterRoleTemplateBinding) error {
	set := labels.Set(map[string]string{rtbOwnerLabel: string(binding.UID)})
	bindingCli := c.m.workload.K8sClient.RbacV1().ClusterRoleBindings()
	rbs, err := c.m.crbLister.List("", set.AsSelector())
	if err != nil {
		return errors.Wrapf(err, "couldn't list clusterrolebindings with selector %s", set.AsSelector())
	}

	for _, rb := range rbs {
		if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "error deleting clusterrolebinding %v", rb.Name)
			}
		}
	}

	return nil
}
