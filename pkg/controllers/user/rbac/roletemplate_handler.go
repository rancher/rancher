package rbac

import (
	"github.com/pkg/errors"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func newRTLifecycle(m *manager) *rtLifecycle {
	return &rtLifecycle{m: m}
}

// rtLifecycle is responsible for ensuring that roleTemplates and their corresponding clusterRoles stay in sync.
// This means that if a roleTemplate's rules change, this handler will ensure the corresponding clusterRole's rules are changed
// and if a roleTemplate is removed, the corresponding clusterRole is removed.
// This handler does not create new clusterRoles. They are created on the fly when a ProjectRoleTemplateBinding or
// ClusterRoleTemplateBinding references the roleTemplates. This handler only ensures they remain in-sync after being created
type rtLifecycle struct {
	m *manager
}

func (c *rtLifecycle) Create(obj *v3.RoleTemplate) (runtime.Object, error) {
	return obj, nil
}

func (c *rtLifecycle) Updated(obj *v3.RoleTemplate) (runtime.Object, error) {
	// checky if there are any PRTBs/CRTBs referencing this RoleTemplate for this cluster
	prtbs, err := c.m.prtbIndexer.ByIndex(rtbByClusterAndRoleTemplateIndex, c.m.workload.ClusterName+"-"+obj.Name)
	if err != nil {
		return obj, err
	}
	hasPRTBs := len(prtbs) > 0
	hasRTBs := hasPRTBs
	var crtbs []interface{}
	if !hasRTBs {
		crtbs, err = c.m.crtbIndexer.ByIndex(rtbByClusterAndRoleTemplateIndex, c.m.workload.ClusterName+"-"+obj.Name)
		if err != nil {
			return obj, err
		}
		hasRTBs = len(crtbs) > 0
	}

	// No RTBs referencing this RoleTemplate in this cluster, do not attempt to sync
	if !hasRTBs {
		return nil, nil
	}

	err = c.syncRT(obj, hasPRTBs, prtbs, crtbs)
	return nil, err
}

func (c *rtLifecycle) Remove(obj *v3.RoleTemplate) (runtime.Object, error) {
	err := c.ensureRTDelete(obj)
	return obj, err
}

func (c *rtLifecycle) syncRT(template *v3.RoleTemplate, usedInProjects bool, prtbs []interface{}, crtbs []interface{}) error {
	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(template, roles); err != nil {
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		return errors.Wrapf(err, "couldn't ensure roles")
	}

	rolesToKeep := make(map[string]bool)
	if usedInProjects {
		for _, rt := range roles {
			for resource := range globalResourcesNeededInProjects {
				verbs, err := c.m.checkForGlobalResourceRules(rt, resource)
				if err != nil {
					return err
				}
				if len(verbs) > 0 {
					roleName, err := c.m.reconcileRoleForProjectAccessToGlobalResource(resource, rt, verbs)
					if err != nil {
						return errors.Wrapf(err, "couldn't reconcile role for project access to global resources")
					}
					rolesToKeep[roleName] = true
				}
			}
		}
	}

	for _, obj := range prtbs {
		prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			continue
		}

		crbsToKeep, err := c.m.reconcileProjectAccessToGlobalResources(prtb, roles)
		if err != nil {
			return err
		}

		rtbUID := string(prtb.UID)
		set := labels.Set(map[string]string{rtbUID: owner})
		existingCrbs, err := c.m.crbLister.List("", set.AsSelector())
		if err != nil {
			return err
		}

		for _, crb := range existingCrbs {
			if !crbsToKeep[crb.Name] {
				c.m.clusterRoleBindings.Delete(crb.Name, &metav1.DeleteOptions{})
			}
		}

		// Get namespaces belonging to project to update the rolebinding in the namespaces of this project for the user
		namespaces, err := c.m.nsIndexer.ByIndex(nsByProjectIndex, prtb.ProjectName)
		if err != nil {
			return errors.Wrapf(err, "couldn't list namespaces with project ID %v", prtb.ProjectName)
		}

		for _, n := range namespaces {
			ns := n.(*v1.Namespace)
			if err := c.m.ensureProjectRoleBindings(ns.Name, roles, prtb); err != nil {
				return errors.Wrapf(err, "couldn't ensure binding %v in %v", prtb.Name, ns.Name)
			}
		}
	}

	for _, obj := range crtbs {
		crtb, ok := obj.(*v3.ClusterRoleTemplateBinding)
		if !ok {
			continue
		}
		if err := c.m.ensureClusterBindings(roles, crtb); err != nil {
			return err
		}
	}
	return nil
}

func (c *rtLifecycle) ensureRTDelete(template *v3.RoleTemplate) error {
	roleCli := c.m.workload.RBAC.ClusterRoles("")
	if err := roleCli.Delete(template.Name, &metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "error deleting clusterrole %v", template.Name)
		}
	}

	return nil
}
