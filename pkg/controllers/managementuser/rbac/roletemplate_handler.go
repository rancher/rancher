package rbac

import (
	"github.com/pkg/errors"
	wranglerv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func newRTLifecycle(m *manager) v3.RoleTemplateHandlerFunc {
	rtSync := &rtSync{m: m}
	return rtSync.sync
}

// rtSync is responsible for ensuring that roleTemplates and their corresponding clusterRoles stay in sync.
// This means that if a roleTemplate's rules change, this handler will ensure the corresponding clusterRole's rules are changed
// If a roleTemplate is removed the management lifecycle will remove the clusterRole from user clusters
// This handler does not create new clusterRoles. They are created on the fly when a ProjectRoleTemplateBinding or
// ClusterRoleTemplateBinding references the roleTemplates. This handler only ensures they remain in-sync after being created
type rtSync struct {
	m *manager
}

func newRTEnqueueFunc(rtIndxer cache.Indexer) relatedresource.Resolver {
	return (&rtEnqueue{rtIndexer: rtIndxer}).rtRelatedResources
}

// rtEnqueue is responsible for returning RoleTemplates names that inherit from a changed roletemplate and should be
// enqueued as a result. This is to ensure those beneficiary roletemplates make the necessary syncs for their own
// corresponding clusterRoles.
type rtEnqueue struct {
	rtIndexer cache.Indexer
}

func (c *rtSync) sync(key string, obj *v3.RoleTemplate) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	// check if there are any PRTBs/CRTBs referencing this RoleTemplate for this cluster
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
		return obj, nil
	}

	err = c.syncRT(obj, hasPRTBs, prtbs, crtbs)
	return obj, err
}

func (c *rtSync) syncRT(template *v3.RoleTemplate, usedInProjects bool, prtbs []interface{}, crtbs []interface{}) error {
	roleTemplates := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(template, roleTemplates, 0); err != nil {
		return err
	}

	if err := c.m.ensureRoles(roleTemplates); err != nil {
		return errors.Wrapf(err, "couldn't ensure roles")
	}

	if usedInProjects {
		for _, rt := range roleTemplates {
			for resource, baseRule := range globalResourceRulesNeededInProjects {
				verbs, err := c.m.checkForGlobalResourceRules(rt, resource, baseRule)
				if err != nil {
					return err
				}

				roleName, err := c.m.reconcileRoleForProjectAccessToGlobalResource(resource, rt.Name, verbs, baseRule)
				if err != nil {
					return errors.Wrapf(err, "couldn't reconcile role '%s' for project access to global resources", roleName)
				}
			}
		}
	}

	for _, obj := range prtbs {
		prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			continue
		}

		roles, err := c.m.ensureGlobalResourcesRolesForPRTB(parseProjectName(prtb.ProjectName), roleTemplates)
		if err != nil {
			return err
		}
		crbsToKeep, err := c.m.reconcileProjectAccessToGlobalResources(prtb, roles)
		if err != nil {
			return err
		}

		rtbNsAndName := rbac.GetRTBLabel(prtb.ObjectMeta)
		set := labels.Set(map[string]string{rtbNsAndName: owner})
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
			if !ns.DeletionTimestamp.IsZero() {
				continue
			}
			if err := c.m.ensureProjectRoleBindings(ns.Name, roleTemplates, prtb); err != nil {
				return errors.Wrapf(err, "couldn't ensure binding %v in %v", prtb.Name, ns.Name)
			}
		}
	}

	for _, obj := range crtbs {
		crtb, ok := obj.(*v3.ClusterRoleTemplateBinding)
		if !ok {
			continue
		}
		if err := c.m.ensureClusterBindings(roleTemplates, crtb); err != nil {
			return err
		}
	}
	return nil
}

func (r *rtEnqueue) rtRelatedResources(_, name string, _ runtime.Object) ([]relatedresource.Key, error) {
	beneficiaryRTs, err := r.rtIndexer.ByIndex(rtByInheritedRTsIndex, name)
	if err != nil {
		return nil, err
	}
	result := make([]relatedresource.Key, len(beneficiaryRTs))
	for i, rt := range beneficiaryRTs {
		result[i] = relatedresource.Key{Name: rt.(*wranglerv3.RoleTemplate).Name}
	}
	return result, nil
}
