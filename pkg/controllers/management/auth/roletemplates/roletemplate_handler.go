package roletemplates

import (
	"errors"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/wrangler"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/slice"
	v1 "k8s.io/api/rbac/v1"
)

var (
	commonClusterAndProjectMgmtPlaneResources = map[string]bool{
		"catalogtemplates":        true,
		"catalogtemplateversions": true,
	}
	clusterManagementPlaneResources = map[string]string{
		"clusterscans":                "management.cattle.io",
		"catalogtemplates":            "management.cattle.io",
		"catalogtemplateversions":     "management.cattle.io",
		"clusteralertrules":           "management.cattle.io",
		"clusteralertgroups":          "management.cattle.io",
		"clustercatalogs":             "management.cattle.io",
		"clusterloggings":             "management.cattle.io",
		"clustermonitorgraphs":        "management.cattle.io",
		"clusterregistrationtokens":   "management.cattle.io",
		"clusterroletemplatebindings": "management.cattle.io",
		"etcdbackups":                 "management.cattle.io",
		"nodes":                       "management.cattle.io",
		"nodepools":                   "management.cattle.io",
		"notifiers":                   "management.cattle.io",
		"projects":                    "management.cattle.io",
		"etcdsnapshots":               "rke.cattle.io",
	}
	prtbClusterManagementPlaneResources = map[string]string{
		"notifiers":               "management.cattle.io",
		"clustercatalogs":         "management.cattle.io",
		"catalogtemplates":        "management.cattle.io",
		"catalogtemplateversions": "management.cattle.io",
	}
	projectManagementPlaneResources = map[string]string{
		"apps":                        "project.cattle.io",
		"apprevisions":                "project.cattle.io",
		"catalogtemplates":            "management.cattle.io",
		"catalogtemplateversions":     "management.cattle.io",
		"sourcecodeproviderconfigs":   "project.cattle.io",
		"projectloggings":             "management.cattle.io",
		"projectalertrules":           "management.cattle.io",
		"projectalertgroups":          "management.cattle.io",
		"projectcatalogs":             "management.cattle.io",
		"projectmonitorgraphs":        "management.cattle.io",
		"projectroletemplatebindings": "management.cattle.io",
		"secrets":                     "",
	}
)

type roleTemplateHandler struct {
	crClient crbacv1.ClusterRoleController
}

func newRoleTemplateHandler(w *wrangler.Context) *roleTemplateHandler {
	return &roleTemplateHandler{
		crClient: w.RBAC.ClusterRole(),
	}
}

// OnChange creates 4 objects. A cluster role to give access to project or cluster scoped objects and an accompanying aggregating cluster role
// It also creates a cluster role for management plane privileges and an accompanying aggregating cluster role
func (r *roleTemplateHandler) OnChange(key string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	if rt == nil || rt.DeletionTimestamp != nil {
		return nil, nil
	}

	// TODO Need more investigation to make sure these rules are correct
	// I might be able to skip some creation if there are no special rules and no inherited RTs
	var managementPlaneRules, scopedPrivileges []v1.PolicyRule
	if rt.Context == "project" {
		managementPlaneRules = getManagementPlaneRules(rt, prtbClusterManagementPlaneResources)
		scopedPrivileges = getMatchingRules(rt, projectManagementPlaneResources)
	} else if rt.Context == "cluster" {
		managementPlaneRules = getManagementPlaneRules(rt, clusterManagementPlaneResources)
		scopedPrivileges = getMatchingRules(rt, projectManagementPlaneResources)
	}

	// TODO pick a better naming convention
	cr := rbac.BuildClusterRole(rt.Name, rt.Name, managementPlaneRules)
	if err := rbac.CreateOrUpdateResource(cr, r.crClient, rbac.AreClusterRolesSame); err != nil {
		return nil, err
	}
	acr := rbac.BuildAggregatingClusterRole(rt, rbac.ClusterManagementPlaneClusterRoleNameFor)
	if err := rbac.CreateOrUpdateResource(acr, r.crClient, rbac.AreAggregatingClusterRolesSame); err != nil {
		return nil, err
	}

	// TODO pick a better naming convention
	cr = rbac.BuildClusterRole(rt.Name, rt.Name, scopedPrivileges)
	if err := rbac.CreateOrUpdateResource(cr, r.crClient, rbac.AreClusterRolesSame); err != nil {
		return nil, err
	}
	acr = rbac.BuildAggregatingClusterRole(rt, rbac.ClusterManagementPlaneClusterRoleNameFor)
	if err := rbac.CreateOrUpdateResource(acr, r.crClient, rbac.AreAggregatingClusterRolesSame); err != nil {
		return nil, err
	}

	return rt, nil
}

func (r *roleTemplateHandler) OnRemove(key string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	// TODO pick better names
	managementPlaneCRName := rt.Name
	managementPlaneACRName := rbac.AggregatedClusterRoleNameFor(managementPlaneCRName)
	scopedPrivilegesCRName := rt.Name
	scopedPrivilegesACRName := rbac.AggregatedClusterRoleNameFor(scopedPrivilegesCRName)

	return nil, errors.Join(
		rbac.DeleteResource(managementPlaneCRName, r.crClient),
		rbac.DeleteResource(managementPlaneACRName, r.crClient),
		rbac.DeleteResource(scopedPrivilegesCRName, r.crClient),
		rbac.DeleteResource(scopedPrivilegesACRName, r.crClient),
	)
}

// TODO getMatchingRules and getManagementPlaneRules try to do the same thing. I need to figure this out more thoroughly
func getMatchingRules(rt *v3.RoleTemplate, resources map[string]string) []v1.PolicyRule {
	var matchingRules []v1.PolicyRule

	for resource, apigroup := range resources {
		// Adding this check, because we want cluster-owners to have access to catalogtemplates/versions of all projects, but no other cluster roles
		// need to access catalogtemplates of projects they do not belong to
		if !rt.Administrative && rt.Context == "cluster" && commonClusterAndProjectMgmtPlaneResources[resource] {
			continue
		}
		for _, rule := range rt.Rules {
			if checkResource(resource, rule) && checkGroup(apigroup, rule) {
				matchingRules = append(matchingRules, rule)
			}
		}
	}
	return matchingRules
}

func checkResource(resource string, rule v1.PolicyRule) bool {
	return slice.ContainsString(rule.Resources, resource) || slice.ContainsString(rule.Resources, "*") && len(rule.ResourceNames) == 0
}

func checkGroup(apiGroup string, rule v1.PolicyRule) bool {
	return slice.ContainsString(rule.APIGroups, apiGroup) || apiGroup == "*"
}

func getManagementPlaneRules(rt *v3.RoleTemplate, managementResources map[string]string) []v1.PolicyRule {
	// TODO handle external rules
	rules := rt.Rules

	var managementRules []v1.PolicyRule
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			apiGroup, ok := managementResources[resource]
			if !ok {
				continue
			}
			if slice.ContainsString(rule.APIGroups, apiGroup) {
				managementRules = append(managementRules, v1.PolicyRule{
					Resources: []string{resource},
					APIGroups: []string{apiGroup},
					Verbs:     rule.Verbs,
				})
			}
		}
	}
	return managementRules
}
