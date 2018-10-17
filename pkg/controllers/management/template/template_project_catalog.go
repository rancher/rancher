package template

import (
	"fmt"
	"reflect"
	"sort"

	"k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

var rolesWithAccessToProjectCatalogTemplates = map[string]bool{
	"project-owner":          true,
	"project-member":         true,
	"projectcatalogs-manage": true,
	"projectcatalogs-view":   true,
}

func (tm *RBACTemplateManager) reconcileProjectCatalog(projectID string, clusterID string) error {
	if projectID == "" || clusterID == "" {
		return nil
	}
	var projectTemplates, projectTemplateVersions []string
	crName := "project-" + clusterID + "-" + projectID + clusterRoleSuffix
	crbName := clusterID + "-" + projectID + clusterRoleBindingSuffix
	// Get all project catalogs created in this project
	projectCatalogs, err := tm.projectCatalogLister.List(projectID, labels.NewSelector())
	if err != nil {
		return fmt.Errorf("error while listing project catalogs: %v", err)
	}
	// Get all templates and template versions for each of these project catalogs.
	// Templates for cluster catalog are created with label "clusterId-projectId-catalogName=catalogName"
	for _, projectCatalog := range projectCatalogs {
		c := clusterID + "-" + projectID + "-" + projectCatalog.Name
		r, err := labels.NewRequirement(c, selection.Equals, []string{projectCatalog.Name})
		if err != nil {
			return err
		}
		//get templates, template versions for this cluster catalog
		templates, templateVersions, err := tm.getTemplateAndTemplateVersions(r)
		// add it to the cluster's templates templateVersions list
		projectTemplates = append(projectTemplates, templates...)
		projectTemplateVersions = append(projectTemplateVersions, templateVersions...)
	}
	// have them sorted for deepEqual calls later
	sort.Strings(projectTemplates)
	sort.Strings(projectTemplateVersions)

	newRules := []v1.PolicyRule{
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{templateRule},
			ResourceNames: projectTemplates,
			Verbs:         []string{"get", "list", "watch"},
		},
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{templateVersionRule},
			ResourceNames: projectTemplateVersions,
			Verbs:         []string{"get", "list", "watch"},
		},
	}

	// Add these templates/templateversions to the clusterRole for this project's templates/templateVersions
	// check if the clusterRole exists
	cRole, err := tm.clusterRoleClient.Get(crName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		// error is IsNotFound, so create it
		newCR := &v1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: crName,
			},
			Rules: newRules,
		}
		if _, err := tm.clusterRoleClient.Create(newCR); err != nil {
			return err
		}
	} else {
		// update the role with the latest templates and templateversions
		if !reflect.DeepEqual(cRole.Rules, newRules) {
			return tm.updateClusterRole(cRole, newRules)
		}
	}

	// Now that the cluster role is created/updated, create/update the cluster role binding with appropriate subjects
	// Get all subjects for this CRB, which includes all users in this projects
	// get a list of all members/owners of this project using PRTBs
	prtbs, err := tm.prtbLister.List(projectID, labels.NewSelector())
	if err != nil {
		return err
	}
	subjects := []v1.Subject{}
	subjectMap := make(map[v1.Subject]bool)
	for _, prtb := range prtbs {
		s, valid, err := buildSubjectFromRTB(prtb)
		if err != nil {
			return err
		}
		if !valid {
			continue
		}
		if !subjectMap[s] {
			subjectMap[s] = true
		}
	}

	for key := range subjectMap {
		subjects = append(subjects, key)
	}

	sort.Slice(subjects, func(i, j int) bool { return subjects[i].Name < subjects[j].Name })

	// Now create or update the  CRB
	// First check if a clusterRoleBinding exists for the templates/templateversions in this cluster
	crb, err := tm.crbClient.Get(crbName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		// crb not found, so create it
		newCRB := &v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: crbName,
			},
			RoleRef: v1.RoleRef{
				Name: crName,
				Kind: "ClusterRole",
			},
			Subjects: subjects,
		}
		if _, err := tm.crbClient.Create(newCRB); err != nil {
			return err
		}
		return nil
	}
	// crb exists, so update it with the new subjects
	if !reflect.DeepEqual(crb.Subjects, subjects) {
		return tm.updateCRB(crb, subjects)
	}

	return nil
}
