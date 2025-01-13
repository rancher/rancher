package roletemplates

import (
	"errors"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/wrangler"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/slice"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	clusterManagementPlaneResources = map[string]string{
		"clusterscans":                "management.cattle.io",
		"clusterregistrationtokens":   "management.cattle.io",
		"clusterroletemplatebindings": "management.cattle.io",
		"etcdbackups":                 "management.cattle.io",
		"nodes":                       "management.cattle.io",
		"nodepools":                   "management.cattle.io",
		"projects":                    "management.cattle.io",
		"etcdsnapshots":               "rke.cattle.io",
	}
	projectManagementPlaneResources = map[string]string{
		"apps":                        "project.cattle.io",
		"sourcecodeproviderconfigs":   "project.cattle.io",
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

// OnChange creates all management plane cluster roles that will be needed. If there are no management plane rules in the role template, no cluster roles will be created.
func (r *roleTemplateHandler) OnChange(_ string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	if rt == nil || rt.DeletionTimestamp != nil {
		return nil, nil
	}

	rules, err := r.getAllRules(rt)
	if err != nil {
		return nil, err
	}

	var clusterScopedPrivileges, projectScopedPrivileges []v1.PolicyRule
	if rt.Context == "project" {
		projectScopedPrivileges = getManagementPlaneRules(rules, projectManagementPlaneResources)
	} else if rt.Context == "cluster" {
		clusterScopedPrivileges = getManagementPlaneRules(rules, clusterManagementPlaneResources)
		projectScopedPrivileges = getManagementPlaneRules(rules, projectManagementPlaneResources)
	}

	var clusterRoles []*v1.ClusterRole
	rtName := rbac.ClusterRoleNameFor(rt.Name)

	if len(clusterScopedPrivileges) != 0 {
		clusterRoles = append(clusterRoles,
			rbac.BuildClusterRole(rbac.ClusterManagementPlaneClusterRoleNameFor(rtName), rtName, clusterScopedPrivileges),
			rbac.BuildAggregatingClusterRole(rt, rbac.ClusterManagementPlaneClusterRoleNameFor))
	}

	if len(projectScopedPrivileges) != 0 {
		clusterRoles = append(clusterRoles,
			rbac.BuildClusterRole(rbac.ProjectManagementPlaneClusterRoleNameFor(rtName), rtName, projectScopedPrivileges),
			rbac.BuildAggregatingClusterRole(rt, rbac.ProjectManagementPlaneClusterRoleNameFor))
	}

	for _, cr := range clusterRoles {
		if err := rbac.CreateOrUpdateResource(cr, r.crClient, rbac.AreClusterRolesSame); err != nil {
			return nil, err
		}
	}

	return rt, nil
}

// OnRemove deletes all Cluster Roles in the local cluster created by the RoleTemplate
func (r *roleTemplateHandler) OnRemove(_ string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	rtName := rbac.ClusterRoleNameFor(rt.Name)
	clusterScopedCRName := rbac.ClusterManagementPlaneClusterRoleNameFor(rtName)
	clusterScopedACRName := rbac.AggregatedClusterRoleNameFor(clusterScopedCRName)
	projectScopedCRName := rbac.ProjectManagementPlaneClusterRoleNameFor(rtName)
	projectScopedACRName := rbac.AggregatedClusterRoleNameFor(projectScopedCRName)

	return nil, errors.Join(
		rbac.DeleteResource(clusterScopedCRName, r.crClient),
		rbac.DeleteResource(clusterScopedACRName, r.crClient),
		rbac.DeleteResource(projectScopedCRName, r.crClient),
		rbac.DeleteResource(projectScopedACRName, r.crClient),
	)
}

// getAllRules collects all policy rules that the RoleTemplate applies. Specifically it searches for external rules.
func (r *roleTemplateHandler) getAllRules(rt *v3.RoleTemplate) ([]v1.PolicyRule, error) {
	if !rt.External {
		return rt.Rules, nil
	}
	if rt.ExternalRules != nil {
		return rt.ExternalRules, nil
	}

	externalRole, err := r.crClient.Get(rt.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Don't error if it doesn't exist
			return nil, nil
		}
		return nil, err
	}

	return externalRole.Rules, nil
}

// getManagementPlaneRules filters a set of rules based on the map passed in. Used to provide special resources that have cluster/project scope.
func getManagementPlaneRules(rules []v1.PolicyRule, managementResources map[string]string) []v1.PolicyRule {
	managementRules := []v1.PolicyRule{}
	for _, rule := range rules {
		for resource, apiGroup := range managementResources {
			if ruleContainsManagementPlaneRule(resource, apiGroup, rule) {
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

// ruleContainsManagementPlaneRule takes a rule and checks if it has the resource and apigroup from the management plane rules.
// If there are ResourceNames specified in the rule, it only applies to specific resources and doesn't count as a management plane rule.
func ruleContainsManagementPlaneRule(resource, apiGroup string, rule v1.PolicyRule) bool {
	return len(rule.ResourceNames) == 0 &&
		(slice.ContainsString(rule.Resources, resource) || slice.ContainsString(rule.Resources, v1.ResourceAll)) &&
		(slice.ContainsString(rule.APIGroups, apiGroup) || slice.ContainsString(rule.APIGroups, v1.APIGroupAll))
}
