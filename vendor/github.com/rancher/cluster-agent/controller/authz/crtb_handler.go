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
	if binding.Subject.Name == "" {
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

	for _, role := range roles {
		if err := c.ensureClusterBinding(role.Name, binding); err != nil {
			return errors.Wrapf(err, "couldn't ensure cluster binding %v %v", role.Name, binding.Subject.Name)
		}
	}

	return nil
}

func (c *crtbLifecycle) ensureClusterBinding(roleName string, binding *v3.ClusterRoleTemplateBinding) error {
	bindingCli := c.m.workload.K8sClient.RbacV1().ClusterRoleBindings()
	bindingName, objectMeta, subjects, roleRef := bindingParts(roleName, string(binding.UID), binding.Subject)
	if c, _ := c.m.crbLister.Get("", bindingName); c != nil {
		return nil
	}

	_, err := bindingCli.Create(&rbacv1.ClusterRoleBinding{
		ObjectMeta: objectMeta,
		Subjects:   subjects,
		RoleRef:    roleRef,
	})

	return err
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
