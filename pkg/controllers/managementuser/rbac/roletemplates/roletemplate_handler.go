package roletemplates

import (
	"errors"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/slice"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var promotedRulesForProjects = map[string]string{
	"navlinks":          "ui.cattle.io",
	"nodes":             "",
	"persistentvolumes": "",
	"storageclasses":    "storage.k8s.io",
	"apiservices":       "apiregistration.k8s.io",
	"clusterrepos":      "catalog.cattle.io",
	"clusters":          "management.cattle.io",
}

const (
	clusterRoleOwnerAnnotation = "authz.cluster.cattle.io/clusterrole-owner"
	aggregationLabel           = "management.cattle.io/aggregates"
	projectContext             = "project"
)

func newRoleTemplateHandler(uc *config.UserContext) *roleTemplateHandler {
	return &roleTemplateHandler{
		crController: uc.RBACw.ClusterRole(),
	}
}

type roleTemplateHandler struct {
	crController crbacv1.ClusterRoleController
}

// OnChange ensures that the following Cluster Roles exist:
//  1. a ClusterRole with the same name as the RoleTemplate
//  2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates with the name "RoleTemplateName-aggregator"
//
// For RoleTemplates with the Context == "Project", we also ensure:
//  1. If the RoleTemplate has any rules for Global Resources, make a ClusterRole with those named "RoleTemplateName-promoted"
//  2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates' promoted Cluster Roles named "RoleTemplateName-promoted-aggregator"
func (rth *roleTemplateHandler) OnChange(_ string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	if rt == nil || rt.DeletionTimestamp != nil {
		return nil, nil
	}

	rules, err := rth.collectRules(rt)
	if err != nil {
		return nil, err
	}
	clusterRoles := clusterRolesForRoleTemplate(rt, rules)
	for _, cr := range clusterRoles {
		if err := rbac.CreateOrUpdateResource(cr, rth.crController, rbac.AreClusterRolesSame); err != nil {
			return nil, err
		}
	}
	return rt, nil
}

// clusterRolesForRoleTemplate builds and returns all needed Cluster Roles for the RoleTemplate using the given rules.
func clusterRolesForRoleTemplate(rt *v3.RoleTemplate, rules []rbacv1.PolicyRule) []*rbacv1.ClusterRole {
	res := []*rbacv1.ClusterRole{
		// 1. Cluster role with rules
		rbac.BuildClusterRole(rbac.ClusterRoleNameFor(rt.Name), rt.Name, rules),
		// 2. Aggregating cluster role
		rbac.BuildAggregatingClusterRole(rt, rbac.ClusterRoleNameFor),
	}
	// Projects can have 2 extra cluster roles for global resources
	if rt.Context == projectContext {
		promotedRules := getPromotedRules(rules)

		// If there are no promoted rules and no inherited RoleTemplates, no need for additional cluster roles
		if len(promotedRules) == 0 && len(rt.RoleTemplateNames) == 0 {
			return res
		}

		if len(promotedRules) != 0 {
			// 3. Project global resources cluster role
			res = append(res, rbac.BuildClusterRole(rbac.PromotedClusterRoleNameFor(rt.Name), rt.Name, promotedRules))
		}

		// 4. Project global resources aggregating cluster role
		// It's possible for this role to have no rules if there are no promoted rules in any of the inherited RoleTemplates or in the above ClusterRole (3)
		// but without fetching all those RoleTemplates and looking through their rules, it's not possible to prevent this ahead of time as the Rules in
		// an aggregating cluster role only get populated at run time
		res = append(res, rbac.BuildAggregatingClusterRole(rt, rbac.PromotedClusterRoleNameFor))
	}
	return res
}

// collectRules returns the rules for a RoleTemplate. If the referenced role is external, it gets the referenced cluster role's rules.
func (rth *roleTemplateHandler) collectRules(rt *v3.RoleTemplate) ([]rbacv1.PolicyRule, error) {
	if rt.External {
		if rt.ExternalRules != nil {
			return rt.ExternalRules, nil
		} else {
			externalRole, err := rth.crController.Get(rt.Name, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			}
			if externalRole != nil {
				return externalRole.Rules, nil
			}
		}
	} else {
		return rt.Rules, nil
	}
	return nil, nil
}

// OnRemove deletes all ClusterRoles created by the RoleTemplate
func (rth *roleTemplateHandler) OnRemove(_ string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	var returnedErrors error

	crName := rbac.ClusterRoleNameFor(rt.Name)
	acrName := rbac.AggregatedClusterRoleNameFor(crName)
	returnedErrors = errors.Join(
		rbac.DeleteResource(crName, rth.crController),
		rbac.DeleteResource(acrName, rth.crController),
	)

	if rt.Context == projectContext {
		promotedCRName := rbac.PromotedClusterRoleNameFor(crName)
		promotedACRName := rbac.AggregatedClusterRoleNameFor(promotedCRName)
		returnedErrors = errors.Join(returnedErrors,
			rbac.DeleteResource(promotedCRName, rth.crController),
			rbac.DeleteResource(promotedACRName, rth.crController),
		)
	}

	return nil, returnedErrors
}

// getPromotedRules filters a list of PolicRules for promoted rules for projects and returns them as a list
func getPromotedRules(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	var promotedRules []rbacv1.PolicyRule
	for _, r := range rules {
		for resource, apigroup := range promotedRulesForProjects {
			if slice.ContainsString(r.Resources, resource) || slice.ContainsString(r.Resources, rbacv1.ResourceAll) {
				if slice.ContainsString(r.APIGroups, apigroup) || slice.ContainsString(r.APIGroups, rbacv1.APIGroupAll) {
					// the only cluster that can be provided is the local cluster
					if resource == "clusters" {
						r.ResourceNames = []string{"local"}
					}
					promotedRules = append(promotedRules, r)
				}
			}
		}
	}
	return promotedRules
}
