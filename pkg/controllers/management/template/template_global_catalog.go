package template

import (
	"reflect"
	"sort"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (tm *RBACTemplateManager) syncForGlobalCatalog(key string, obj *v3.Catalog) (*v3.Catalog, error) {
	var templateRuleExists, templateVersionsRuleExists bool
	// Not going to check here whether obj is nil/deleted or not. Whether global catalog is created, updated or deleted, this method reconciles
	// the global user role. It gets all templates/templateversions for the global catalogs and updates the user role

	//Get all templates created for global catalogs only and add them via resourceNames to the "user" role
	r, err := labels.NewRequirement(catalogTypeLabel, selection.Equals, []string{"globalCatalog"})
	if err != nil {
		return nil, err
	}

	userRole, err := tm.globalRoleClient.Get("user", metav1.GetOptions{})
	userRole = userRole.DeepCopy()

	templates, templateVersions, err := tm.getTemplateAndTemplateVersions(r)
	sort.Strings(templates)
	sort.Strings(templateVersions)
	updatedRules := userRole.Rules
	newTemplateRule := []v1.PolicyRule{
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{templateRule},
			ResourceNames: templates,
			Verbs:         []string{"get", "list", "watch"},
		},
	}

	newTemplateVersionRule := []v1.PolicyRule{
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{templateVersionRule},
			ResourceNames: templateVersions,
			Verbs:         []string{"get", "list", "watch"},
		},
	}

	for ind, r := range updatedRules {
		switch r.Resources[0] {
		case templateRule:
			r.ResourceNames = templates
			updatedRules[ind] = r
			templateRuleExists = true
		case templateVersionRule:
			r.ResourceNames = templateVersions
			updatedRules[ind] = r
			templateVersionsRuleExists = true
		}
	}

	if !templateRuleExists {
		updatedRules = append(updatedRules, newTemplateRule...)
	}
	if !templateVersionsRuleExists {
		updatedRules = append(updatedRules, newTemplateVersionRule...)
	}

	if !reflect.DeepEqual(userRole.Rules, updatedRules) {
		newRole := userRole.DeepCopy()
		newRole.Rules = updatedRules
		_, err = tm.globalRoleClient.Update(newRole)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
