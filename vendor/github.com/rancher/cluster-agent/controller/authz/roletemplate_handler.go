package authz

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newRTLifecycle(m *manager) *rtLifecycle {
	return &rtLifecycle{m: m}
}

type rtLifecycle struct {
	m *manager
}

func (c *rtLifecycle) Create(obj *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	err := c.syncRT(obj)
	return obj, err
}

func (c *rtLifecycle) Updated(obj *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	err := c.syncRT(obj)
	return obj, err
}

func (c *rtLifecycle) Remove(obj *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	err := c.ensureRTDelete(obj)
	return obj, err
}

func (c *rtLifecycle) syncRT(template *v3.RoleTemplate) error {
	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(template, roles); err != nil {
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		return errors.Wrapf(err, "couldn't ensure roles")
	}

	return nil
}

func (c *rtLifecycle) ensureRTDelete(template *v3.RoleTemplate) error {
	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(template, roles); err != nil {
		return err
	}

	roleCli := c.m.workload.K8sClient.RbacV1().ClusterRoles()
	for _, role := range roles {
		if err := roleCli.Delete(role.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "error deleting clusterrole %v", role.Name)
			}
		}
	}

	return nil
}
