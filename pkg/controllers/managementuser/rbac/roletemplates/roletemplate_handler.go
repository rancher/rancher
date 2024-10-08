package roletemplates

import (
	"errors"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/slice"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO: use safeconcat for the names
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
	clusterRoleOwner = "authz.cluster.cattle.io/clusterrole-owner"
)

func newRoleTemplateLifecycle(uc *config.UserContext) *roleTemplateLifecycle {
	return &roleTemplateLifecycle{
		crClient: uc.RBAC.ClusterRoles(""),
	}
}

type roleTemplateLifecycle struct {
	crClient typesrbacv1.ClusterRoleInterface
}

// Create func always creates:
// 1. a ClusterRole with the same name as the RoleTemplate
// 2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates with the name "RoleTemplateName-aggregator"
//
// For RoleTemplates with the Context == "Project", we also create:
// 1. If the RoleTemplate has any rules for Global Resources, make a ClusterRole with those named "RoleTemplateName-promoted"
// 2. an Aggregating ClusterRole that aggregates all inherited RoleTemplates' promoted Cluster Roles named "RoleTemplateName-promoted-aggregator"
func (rtl *roleTemplateLifecycle) Create(rt *v3.RoleTemplate) (runtime.Object, error) {
	// 1. Create cluster role
	cr := rtl.buildClusterRole(rt)
	_, err := rtl.crClient.Create(cr)
	if err != nil {
		return nil, err
	}
	// 2. Create aggregating cluster role
	cr = rtl.buildAggregatingClusterRole(rt)
	_, err = rtl.crClient.Create(cr)
	if err != nil {
		return nil, err
	}
	// 3. Project global resources
	if rt.Context == "project" {
		// Create a promoted cluster role
		cr = rtl.buildProjectPromotedClusterRole(rt)
		_, err = rtl.crClient.Create(cr)
		if err != nil {
			return nil, err
		}

		// Create a promoted aggregating cluster role
		cr = rtl.buildProjectPromotedAggregatedClusterRole(rt)
		_, err = rtl.crClient.Create(cr)
		if err != nil {
			return nil, err
		}
	}

	return rt, nil
}

func (rtl *roleTemplateLifecycle) Updated(rt *v3.RoleTemplate) (runtime.Object, error) {
	// 1. Create cluster role
	cr := rtl.buildClusterRole(rt)
	_, err := rtl.crClient.Update(cr)
	if err != nil {
		return nil, err
	}
	// 2. Create aggregating cluster role
	cr = rtl.buildAggregatingClusterRole(rt)
	_, err = rtl.crClient.Update(cr)
	if err != nil {
		return nil, err
	}
	// 3. Project global resources
	if rt.Context == "project" {
		// Create a promoted cluster role
		cr = rtl.buildProjectPromotedClusterRole(rt)
		_, err = rtl.crClient.Update(cr)
		if err != nil {
			return nil, err
		}

		// Create a promoted aggregating cluster role
		cr = rtl.buildProjectPromotedAggregatedClusterRole(rt)
		_, err = rtl.crClient.Update(cr)
		if err != nil {
			return nil, err
		}
	}

	return rt, nil
}

func (rtl *roleTemplateLifecycle) Remove(rt *v3.RoleTemplate) (runtime.Object, error) {
	err := errors.Join(
		rtl.crClient.Delete(rt.Name, &metav1.DeleteOptions{}),
		rtl.crClient.Delete(rt.Name+"-aggregator", &metav1.DeleteOptions{}))
	if rt.Context == "project" {
		err = errors.Join(err,
			rtl.crClient.Delete(rt.Name+"-promoted", &metav1.DeleteOptions{}),
			rtl.crClient.Delete(rt.Name+"-promoted-aggregator", &metav1.DeleteOptions{}))
	}
	return nil, err
}

func (rtl *roleTemplateLifecycle) buildClusterRole(rt *v3.RoleTemplate) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: rt.Name,
			// Label we use for aggregation
			Labels: map[string]string{
				rt.Name: "true",
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwner: rt.Name},
		},
		Rules: rt.Rules,
	}
}

func (rtl *roleTemplateLifecycle) buildAggregatingClusterRole(rt *v3.RoleTemplate) *v1.ClusterRole {
	// We want to aggregate every specified role template
	var roleTemplateLabels []metav1.LabelSelector
	for _, roleTemplateName := range rt.RoleTemplateNames {
		labelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{roleTemplateName: "true"},
		}
		roleTemplateLabels = append(roleTemplateLabels, labelSelector)
	}

	crName := rt.Name + "-aggregator"
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			// Label we use for aggregation
			Labels: map[string]string{
				rt.Name: "true",
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwner: rt.Name},
		},
		AggregationRule: &v1.AggregationRule{
			ClusterRoleSelectors: roleTemplateLabels,
		},
	}
}

func (rtl *roleTemplateLifecycle) buildProjectPromotedClusterRole(rt *v3.RoleTemplate) *v1.ClusterRole {
	promotedRules := getPromotedRules(rt.Rules)
	if len(promotedRules) == 0 {
		return nil
	}
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: rt.Name + "-promoted",
			// Label we use for aggregation
			Labels: map[string]string{
				rt.Name + "-promoted": "true",
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwner: rt.Name},
		},
		Rules: promotedRules,
	}
}

// TODO handle external rules
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

func (rtl *roleTemplateLifecycle) buildProjectPromotedAggregatedClusterRole(rt *v3.RoleTemplate) *v1.ClusterRole {
	// We want to aggregate every specified role template's promoted role
	var roleTemplateLabels []metav1.LabelSelector
	for _, roleTemplateName := range rt.RoleTemplateNames {
		labelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{roleTemplateName + "-promoted": "true"},
		}
		roleTemplateLabels = append(roleTemplateLabels, labelSelector)
	}

	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: rt.Name + "-promoted-aggregator",
			// Label we use for aggregation
			Labels: map[string]string{
				rt.Name + "-promoted": "true",
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwner: rt.Name},
		},
		AggregationRule: &v1.AggregationRule{
			ClusterRoleSelectors: roleTemplateLabels,
		},
	}
}
