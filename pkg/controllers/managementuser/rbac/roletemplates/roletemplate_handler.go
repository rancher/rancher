package roletemplates

import (
	"errors"
	"reflect"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/slice"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: handle external rules

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
	promotedSuffix             = "promoted"
)

func newRoleTemplateHandler(uc *config.UserContext) *roleTemplateHandler {
	return &roleTemplateHandler{
		crClient: uc.Management.Wrangler.RBAC.ClusterRole(),
	}
}

type roleTemplateHandler struct {
	crClient crbacv1.ClusterRoleController
}

// OnChange ensures that the following Cluster Roles exist:
//  1. a ClusterRole with the same name as the RoleTemplate
//  2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates with the name "RoleTemplateName-aggregator"
//
// For RoleTemplates with the Context == "Project", we also ensure:
//  1. If the RoleTemplate has any rules for Global Resources, make a ClusterRole with those named "RoleTemplateName-promoted"
//  2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates' promoted Cluster Roles named "RoleTemplateName-promoted-aggregator"
func (rtl *roleTemplateHandler) OnChange(key string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {

	getFunc := func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
		return rtl.crClient.Get(cr.Name, metav1.GetOptions{})
	}

	// 1. Cluster role with rules
	cr := rtl.buildClusterRole(clusterRoleNameFor(rt), rt.Name, rt.Rules)
	if err := createOrUpdateResource(cr, rtl.crClient, getFunc, areClusterRolesSame); err != nil {
		return nil, err
	}

	// 2. Aggregating cluster role
	cr = rtl.buildAggregatingClusterRole(clusterRoleNameFor(rt), rt.Name, rt.RoleTemplateNames)
	if err := createOrUpdateResource(cr, rtl.crClient, getFunc, areAggregatingClusterRolesSame); err != nil {
		return nil, err
	}

	// Projects can have 2 extra cluster roles for global resources
	if rt.Context == projectContext {
		promotedRules := getPromotedRules(rt.Rules)

		// If there are no promoted rules and no inherited RoleTemplates, no need for additional cluster roles
		if len(promotedRules) == 0 && len(rt.RoleTemplateNames) == 0 {
			return rt, nil
		}

		if len(promotedRules) != 0 {
			// 3. Project global resources cluster role
			cr = rtl.buildClusterRole(promotedClusterRoleNameFor(rt), rt.Name, promotedRules)
			if err := createOrUpdateResource(cr, rtl.crClient, getFunc, areClusterRolesSame); err != nil {
				return nil, err
			}
		}

		// 4. Project global resources aggregating cluster role
		// It's possible for this role to have no rules if there are no promoted rules in any of the inherited RoleTemplates or in the above ClusterRole (3)
		// but without fetching all those RoleTemplates and looking through their rules, it's not possible to prevent this ahead of time as the Rules in
		// an aggregating cluster role only get populated at run time
		cr = rtl.buildAggregatingClusterRole(promotedClusterRoleNameFor(rt), rt.Name, rt.RoleTemplateNames)
		if err := createOrUpdateResource(cr, rtl.crClient, getFunc, areAggregatingClusterRolesSame); err != nil {
			return nil, err
		}
	}
	return rt, nil
}

// areClusterRolesSame returns true if the current ClusterRole has the same fields present in the desired ClusterRole.
// If not, it also updates the current ClusterRole fields to match the desired ClusterRole.
// The fields it checks are:
//
//   - Rules
//   - Cluster role owner annotation
//   - Aggregation label
func areClusterRolesSame(currentCR, wantedCR *rbacv1.ClusterRole) (bool, *rbacv1.ClusterRole) {
	same := true
	if !reflect.DeepEqual(currentCR.Rules, wantedCR.Rules) {
		same = false
		currentCR.AggregationRule = wantedCR.AggregationRule
	}
	if currentCR.Annotations[clusterRoleOwnerAnnotation] != wantedCR.Annotations[clusterRoleOwnerAnnotation] {
		same = false
		currentCR.Annotations[clusterRoleOwnerAnnotation] = wantedCR.Annotations[clusterRoleOwnerAnnotation]
	}
	if currentCR.Labels[aggregationLabel] != wantedCR.Labels[aggregationLabel] {
		same = false
		currentCR.Labels[aggregationLabel] = wantedCR.Labels[aggregationLabel]
	}
	return same, currentCR
}

// areAggregatingClusterRolesSame returns true if the current ClusterRole has the same fields present in the desired ClusterRole.
// If not, it also updates the current ClusterRole fields to match the desired ClusterRole.
// The fields it checks are:
//
//   - AggregationRule
//   - Cluster role owner annotation
//   - Aggregation label
func areAggregatingClusterRolesSame(currentCR, wantedCR *rbacv1.ClusterRole) (bool, *rbacv1.ClusterRole) {
	same := true
	if !reflect.DeepEqual(currentCR.AggregationRule, wantedCR.AggregationRule) {
		same = false
		currentCR.AggregationRule = wantedCR.AggregationRule
	}
	if currentCR.Annotations[clusterRoleOwnerAnnotation] != wantedCR.Annotations[clusterRoleOwnerAnnotation] {
		same = false
		currentCR.Annotations[clusterRoleOwnerAnnotation] = wantedCR.Annotations[clusterRoleOwnerAnnotation]
	}
	if currentCR.Labels[aggregationLabel] != wantedCR.Labels[aggregationLabel] {
		same = false
		currentCR.Labels[aggregationLabel] = wantedCR.Labels[aggregationLabel]
	}
	return same, currentCR
}

// Remove deletes all ClusterRoles created by the RoleTemplate
func (rtl *roleTemplateHandler) OnRemove(key string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	var returnedErrors error

	crName := clusterRoleNameFor(rt)
	err := rtl.crClient.Delete(crName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		returnedErrors = errors.Join(returnedErrors, err)
	}

	err = rtl.crClient.Delete(addAggregatorSuffix(crName), &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		returnedErrors = errors.Join(returnedErrors, err)
	}

	if rt.Context == projectContext {
		crName = promotedClusterRoleNameFor(rt)
		err = rtl.crClient.Delete(crName, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			returnedErrors = errors.Join(returnedErrors, err)
		}

		err = rtl.crClient.Delete(addAggregatorSuffix(crName), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			returnedErrors = errors.Join(returnedErrors, err)
		}
	}

	return nil, returnedErrors
}

// buildClusterRole returns a ClusterRole given a name, the name of the RoleTemplate that owns this ClusterRole and a set of rules
func (rtl *roleTemplateHandler) buildClusterRole(name, ownerName string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			// Label we use for aggregation
			Labels: map[string]string{
				aggregationLabel: name,
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwnerAnnotation: ownerName},
		},
		Rules: rules,
	}
}

// buildAggregatingClusterRole returns a ClusterRole with AggregationRules given a name, the name of the RoleTemplate that owns this ClusterRole
// and the names of the RoleTemplates to inherit
func (rtl *roleTemplateHandler) buildAggregatingClusterRole(name, ownerName string, roleTemplateNames []string) *rbacv1.ClusterRole {
	// aggregate our own cluster role
	roleTemplateLabels := []metav1.LabelSelector{{MatchLabels: map[string]string{aggregationLabel: name}}}
	// aggregate every inherited role template
	for _, roleTemplateName := range roleTemplateNames {
		labelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{aggregationLabel: roleTemplateName},
		}
		roleTemplateLabels = append(roleTemplateLabels, labelSelector)
	}

	crName := addAggregatorSuffix(name)
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			// Label so other cluster roles can aggregate this one
			Labels: map[string]string{
				aggregationLabel: name,
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwnerAnnotation: ownerName},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: roleTemplateLabels,
		},
	}
}

// TODO: handle external rules (ask Raul)
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

// clusterRoleNameFor returns the cluster role name for a given RoleTemplate
func clusterRoleNameFor(rt *v3.RoleTemplate) string {
	return rt.Name
}

// promotedClusterRoleNameFor returns the cluster role name of a promoted cluster role for a given RoleTemplate
func promotedClusterRoleNameFor(rt *v3.RoleTemplate) string {
	return name.SafeConcatName(rt.Name + promotedSuffix)
}
