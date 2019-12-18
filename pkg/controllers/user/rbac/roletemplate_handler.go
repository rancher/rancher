package rbac

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "k8s.io/api/rbac/v1"
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
		rtbUID := string(prtb.UID)
		set := labels.Set(map[string]string{rtbUID: owner})
		crbs, err := c.m.crbLister.List("", set.AsSelector())
		if err != nil {
			return err
		}
		existingCRBs := make(map[string]bool)
		for _, crb := range crbs {
			existingCRBs[crb.RoleRef.Name] = true
		}
		if parts := strings.SplitN(prtb.ProjectName, ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
			projectName := parts[1]
			var roleVerb, roleSuffix string
			for _, rtName := range template.RoleTemplateNames {
				var createNS bool
				rt, err := c.m.rtLister.Get("", rtName)
				if err != nil {
					return err
				}
				for _, rule := range rt.Rules {
					if slice.ContainsString(rule.Resources, "namespaces") && len(rule.ResourceNames) == 0 {
						if slice.ContainsString(rule.Verbs, "*") || slice.ContainsString(rule.Verbs, "create") {
							roleVerb = "*"
							createNS = true
							break
						}
					}
				}
				if rt.Rules == nil {
					// Kubernetes admin/edit roles
					if rt.External && rt.Context == "project" && (rt.Name == "admin" || rt.Name == "edit") {
						roleVerb = "*"
						createNS = true
					}
				}

				if roleVerb == "" {
					roleVerb = "get"
				}
				roleSuffix = projectNSVerbToSuffix[roleVerb]
				role := fmt.Sprintf(projectNSGetClusterRoleNameFmt, projectName, roleSuffix)
				rolesToKeep[role] = true
				if createNS {
					rolesToKeep["create-ns"] = true
				}
				for resource := range globalResourcesNeededInProjects {
					verbs, err := c.m.checkForGlobalResourceRules(rt, resource)
					if err != nil {
						return err
					}
					if len(verbs) > 0 {
						roleName, err := c.m.reconcileRoleForProjectAccessToGlobalResource(resource, rt, verbs)
						if err != nil {
							return err
						}
						rolesToKeep[roleName] = true
					}
				}
			}
			for _, crb := range crbs {
				if !rolesToKeep[crb.RoleRef.Name] {
					if err := c.m.clusterRoleBindings.Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
						return err
					}
				} else {
					delete(rolesToKeep, crb.RoleRef.Name)
				}
			}
			for role := range rolesToKeep {
				if existingCRBs[role] {
					continue
				}
				subject, err := pkgrbac.BuildSubjectFromRTB(prtb)
				if err != nil {
					return err
				}
				_, err = c.m.clusterRoleBindings.Create(&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "clusterrolebinding-",
						Labels: map[string]string{
							rtbUID: owner,
						},
					},
					Subjects: []rbacv1.Subject{subject},
					RoleRef: rbacv1.RoleRef{
						Kind: "ClusterRole",
						Name: role,
					},
				})
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			}
		}
	}

	for _, rtName := range template.RoleTemplateNames {
		rt, err := c.m.rtLister.Get("", rtName)
		if err != nil {
			return err
		}
		roles[rt.Name] = rt
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
