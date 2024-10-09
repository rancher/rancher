package roletemplates

import (
	"errors"
	"reflect"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/slice"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO: handle external rules
// TODO: Use a Wrangler client instead of Norman

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
	aggregationLabel           = "management.cattle.io/aggregates-to"
	projectContext             = "project"
	aggregatorSuffix           = "aggregator"
	promotedSuffix             = "promoted"
)

func newRoleTemplateLifecycle(uc *config.UserContext) *roleTemplateLifecycle {
	return &roleTemplateLifecycle{
		crClient: uc.RBAC.ClusterRoles(""),
	}
}

type roleTemplateLifecycle struct {
	crClient typesrbacv1.ClusterRoleInterface
}

func (rtl *roleTemplateLifecycle) Create(rt *v3.RoleTemplate) (runtime.Object, error) {
	return rtl.sync(rt)
}

func (rtl *roleTemplateLifecycle) Updated(rt *v3.RoleTemplate) (runtime.Object, error) {
	return rtl.sync(rt)
}

// sync ensures that the following Cluster Roles exist:
// 1. a ClusterRole with the same name as the RoleTemplate
// 2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates with the name "RoleTemplateName-aggregator"
//
// For RoleTemplates with the Context == "Project", we also ensure:
// 1. If the RoleTemplate has any rules for Global Resources, make a ClusterRole with those named "RoleTemplateName-promoted"
// 2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates' promoted Cluster Roles named "RoleTemplateName-promoted-aggregator"
func (rtl *roleTemplateLifecycle) sync(rt *v3.RoleTemplate) (runtime.Object, error) {
	// 1. Cluster role with rules
	cr := rtl.buildClusterRole(clusterRoleNameFor(rt), rt.Name, rt.Rules)
	err := rtl.createOrUpdateClusterRole(cr)
	if err != nil {
		return nil, err
	}
	// 2. Aggregating cluster role
	cr = rtl.buildAggregatingClusterRole(clusterRoleNameFor(rt), rt.Name, rt.RoleTemplateNames)
	err = rtl.createOrUpdateClusterRole(cr)
	if err != nil {
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
			err = rtl.createOrUpdateClusterRole(cr)
			if err != nil {
				return nil, err
			}
		}

		// 4. Project global resources aggregating cluster role
		// It's possible for this role to have no rules if there are no promoted rules in any of the inherited RoleTemplates or in the above ClusterRole (3)
		// but without fetching all those RoleTemplates and looking through their rules, it's not possible to prevent this ahead of time as the Rules in
		// an aggregating cluster role only get populated at run time
		cr = rtl.buildAggregatingClusterRole(promotedClusterRoleNameFor(rt), rt.Name, rt.RoleTemplateNames)
		err = rtl.createOrUpdateClusterRole(cr)
		if err != nil {
			return nil, err
		}
	}
	return rt, nil
}

// createOrUpdateClusterRole creates or updates the given Cluster Role
func (rtl *roleTemplateLifecycle) createOrUpdateClusterRole(cr *v1.ClusterRole) error {
	// attempt to get the cluster role
	clusterRole, err := rtl.crClient.Get(cr.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Cluster Role doesn't exist, create it
		_, err = rtl.crClient.Create(cr)
		if apierrors.IsAlreadyExists(err) {
			// in the case where it got created at the same time somehow, attempt to get it again
			clusterRole, err = rtl.crClient.Get(cr.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			return nil
		}
	} else if err != nil {
		return err
	}

	// check that the existing cluster role is the same as the one we want
	if hasClusterRoleChanged(clusterRole, cr) {
		// if it has changed, update it to the correct version
		_, err := rtl.crClient.Update(cr)
		if err != nil {
			return err
		}
	}
	return nil
}

// hasClusterRoleChanged returns true if the current ClusterRole differs from the expected ClusterRole
// TODO: customize this comparison
// reflect.DeepEqual is too strong of a check because our expected CR won't have metadata, labels applied elsewhere, etc
func hasClusterRoleChanged(currentCR, expectedCR *v1.ClusterRole) bool {
	return !reflect.DeepEqual(currentCR, expectedCR)
}

// Remove deletes all ClusterRoles created by the RoleTemplate
func (rtl *roleTemplateLifecycle) Remove(rt *v3.RoleTemplate) (runtime.Object, error) {
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
func (rtl *roleTemplateLifecycle) buildClusterRole(name, ownerName string, rules []v1.PolicyRule) *v1.ClusterRole {
	return &v1.ClusterRole{
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
func (rtl *roleTemplateLifecycle) buildAggregatingClusterRole(name, ownerName string, roleTemplateNames []string) *v1.ClusterRole {
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
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			// Label so other cluster roles can aggregate this one
			Labels: map[string]string{
				aggregationLabel: name,
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwnerAnnotation: ownerName},
		},
		AggregationRule: &v1.AggregationRule{
			ClusterRoleSelectors: roleTemplateLabels,
		},
	}
}

// TODO handle external rules (ask Raul)
func getPromotedRules(rules []v1.PolicyRule) []v1.PolicyRule {
	var promotedRules []v1.PolicyRule
	for _, r := range rules {
		for resource, apigroup := range promotedRulesForProjects {
			if slice.ContainsString(r.Resources, resource) || slice.ContainsString(r.Resources, "*") {
				if slice.ContainsString(r.APIGroups, apigroup) || slice.ContainsString(r.APIGroups, "*") {
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

// addAggregatorSuffix appends the aggregation suffix to a string safely (ie <= 63 characters)
func addAggregatorSuffix(s string) string {
	return name.SafeConcatName(s + aggregatorSuffix)
}
