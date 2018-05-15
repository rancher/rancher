package rbac

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
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

	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
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

	if err := c.m.ensureClusterBindings(roles, binding); err != nil {
		return errors.Wrapf(err, "couldn't ensure cluster bindings %v", binding)
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
