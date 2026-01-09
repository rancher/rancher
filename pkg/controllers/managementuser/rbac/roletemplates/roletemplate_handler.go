package roletemplates

import (
	"fmt"
	"slices"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	projectContext = "project"
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
//  1. A ClusterRole with the same name as the RoleTemplate (unless RoleTemplate is External)
//  2. An Aggregating ClusterRole that aggregates all inherited RoleTemplates with the name "RoleTemplateName-aggregator"
//
// For RoleTemplates with the Context == "Project", the additional cluster roles are created:
//  1. If the RoleTemplate has any rules for Global Resources, make a ClusterRole with those named "RoleTemplateName-promoted"
//  2. An Aggregating ClusterRole that aggregates all inherited RoleTemplates' promoted Cluster Roles named "RoleTemplateName-promoted-aggregator"
func (rth *roleTemplateHandler) OnChange(_ string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	if rt == nil || rt.DeletionTimestamp != nil || !features.AggregatedRoleTemplates.Enabled() {
		return nil, nil
	}

	clusterRoles, err := rth.clusterRolesForRoleTemplate(rt)
	if err != nil {
		return nil, err
	}
	for _, cr := range clusterRoles {
		if err := rbac.CreateOrUpdateResource(cr, rth.crController, rbac.AreClusterRolesSame); err != nil {
			return nil, err
		}
	}

	// add aggregation label to external cluster role
	if err := rth.addLabelToExternalRole(rt); err != nil {
		return nil, err
	}

	return rt, nil
}

// clusterRolesForRoleTemplate builds and returns all needed Cluster Roles for the RoleTemplate using the given rules.
func (rth *roleTemplateHandler) clusterRolesForRoleTemplate(rt *v3.RoleTemplate) ([]*rbacv1.ClusterRole, error) {
	res := []*rbacv1.ClusterRole{}
	// We extract the promoted rules from the RoleTemplate, so we don't want to modify the original object
	rtCopy := rt.DeepCopy()

	if rtCopy.Context == projectContext {
		// ClusterRoles for promoted rules
		var promotedClusterRoles []*rbacv1.ClusterRole
		var err error
		promotedClusterRoles, err = rth.buildPromotedClusterRoles(rt)
		if err != nil {
			return nil, err
		}

		res = append(res, promotedClusterRoles...)
	}

	// If the RoleTemplate refers to an external cluster role, don't modify/create it. Instead we will aggregate it.
	if !rtCopy.External {
		res = append(res, rbac.BuildClusterRole(rbac.ClusterRoleNameFor(rtCopy.Name), rtCopy.Name, rtCopy.Rules))
	}
	res = append(res, rbac.BuildAggregatingClusterRole(rtCopy, rbac.ClusterRoleNameFor))

	return res, nil
}

// buildPromotedClusterRoles looks for promoted rules in a project role template and creates required promoted cluster roles.
// It also returns the role template rules with the promoted rules removed.
func (rth *roleTemplateHandler) buildPromotedClusterRoles(rt *v3.RoleTemplate) ([]*rbacv1.ClusterRole, error) {
	clusterRoles := []*rbacv1.ClusterRole{}

	promotedRules := ExtractPromotedRules(rt.Rules)

	inheritedPromotedRules, err := rth.areThereInheritedPromotedRules(rt.RoleTemplateNames)
	if err != nil {
		return nil, err
	}

	// If there are no promoted rules and no inherited RoleTemplates with promoted rules, no need for additional cluster roles
	if len(promotedRules) == 0 && !inheritedPromotedRules {
		return clusterRoles, nil
	}

	if len(promotedRules) != 0 {
		// Create a promoted cluster role
		clusterRoles = append(clusterRoles, rbac.BuildClusterRole(rbac.PromotedClusterRoleNameFor(rt.Name), rt.Name, promotedRules))
	}

	// If there are promoted rules or inherited promoted rules, an aggregating cluster role will be what PRTBs bind to.
	clusterRoles = append(clusterRoles, rbac.BuildAggregatingClusterRole(rt, rbac.PromotedClusterRoleNameFor))

	return clusterRoles, nil
}

// areThereInheritedPromotedRules checks if any of the inherited RoleTemplates contain promoted rules. If none do, return false.
func (rth *roleTemplateHandler) areThereInheritedPromotedRules(inheritedRoleTemplates []string) (bool, error) {
	for _, rt := range inheritedRoleTemplates {
		_, err := rth.crController.Get(rbac.AggregatedClusterRoleNameFor(rbac.PromotedClusterRoleNameFor(rt)), metav1.GetOptions{})
		if err == nil {
			return true, nil
		} else if !apierrors.IsNotFound(err) {
			return false, err
		}
	}
	return false, nil
}

var promotedRulesForProjects = map[string]string{
	"navlinks":          "ui.cattle.io",
	"nodes":             "",
	"persistentvolumes": "",
	"storageclasses":    "storage.k8s.io",
	"apiservices":       "apiregistration.k8s.io",
	"clusterrepos":      "catalog.cattle.io",
	"clusters":          "management.cattle.io",
}

// ExtractPromotedRules filters a list of PolicyRules for promoted rules for projects and returns the list of promoted rules.
func ExtractPromotedRules(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	promotedRules := []rbacv1.PolicyRule{}
	for _, rule := range rules {
		for resource, apigroup := range promotedRulesForProjects {
			// Check if the rule contains the resource and apigroup of a global resource
			resourceMatch := slices.Contains(rule.Resources, resource) || slices.Contains(rule.Resources, rbacv1.ResourceAll)
			apiGroupMatch := slices.Contains(rule.APIGroups, apigroup) || slices.Contains(rule.APIGroups, rbacv1.APIGroupAll)

			if resourceMatch && apiGroupMatch {
				// Create our promoted rule for the specific global resource
				promotedRule := rbacv1.PolicyRule{
					Resources:     []string{resource},
					APIGroups:     []string{apigroup},
					Verbs:         rule.Verbs,
					ResourceNames: rule.ResourceNames,
				}

				// the only cluster that can be provided is the local cluster
				if resource == "clusters" {
					// we only care about the ResourceName "local"
					if len(rule.ResourceNames) != 0 && !slices.Contains(rule.ResourceNames, "local") {
						continue
					}
					promotedRule.ResourceNames = []string{"local"}
				}
				promotedRules = append(promotedRules, promotedRule)
				continue
			}
		}
	}
	return promotedRules
}

// addLabelToExternalRole ensures the external role has the right aggregation label.
// It is a no-op if the RoleTemplate does not have an external role.
func (rth *roleTemplateHandler) addLabelToExternalRole(rt *v3.RoleTemplate) error {
	if !rt.External {
		return nil
	}

	externalRole, err := rth.crController.Get(rt.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if externalRole.Labels == nil {
		externalRole.Labels = map[string]string{}
	}

	if val, ok := externalRole.Labels[rbac.AggregationLabel]; !ok || val != rbac.ClusterRoleNameFor(rt.Name) {
		externalRole.Labels[rbac.AggregationLabel] = rbac.ClusterRoleNameFor(rt.Name)
		if _, err := rth.crController.Update(externalRole); err != nil {
			return fmt.Errorf("failed to update external cluster role %s: %w", externalRole.Name, err)
		}
	}

	return nil
}
