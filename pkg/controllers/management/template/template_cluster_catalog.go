package template

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/rancher/types/apis/management.cattle.io/v3"

	"k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

var rolesWithAccessToClusterCatalogTemplates = map[string]bool{
	"cluster-owner":          true,
	"cluster-member":         true,
	"clustercatalogs-manage": true,
	"clustercatalogs-view":   true,
}

func (tm *RBACTemplateManager) reconcileClusterCatalog(clusterID string) error {
	if clusterID == "" {
		return nil
	}
	var clusterTemplates, clusterTemplateVersions []string
	crName := "cluster-" + clusterID + clusterRoleSuffix
	crbName := clusterID + clusterRoleBindingSuffix
	// Get all cluster catalogs created in this cluster
	clusterCatalogs, err := tm.clusterCatalogLister.List(clusterID, labels.NewSelector())
	if err != nil {
		return fmt.Errorf("error while listing cluster catalogs: %v", err)
	}
	// Get all templates and template versions for each of these cluster catalogs.
	// Templates for cluster catalog are created with label "clusterId-catalogName=catalogName"
	for _, clusterCatalog := range clusterCatalogs {
		c := clusterID + "-" + clusterCatalog.Name
		r, err := labels.NewRequirement(c, selection.Equals, []string{clusterCatalog.Name})
		if err != nil {
			return err
		}
		//get templates, template versions for this cluster catalog
		templates, templateVersions, err := tm.getTemplateAndTemplateVersions(r)
		// add it to the cluster's templates templateVersions list
		clusterTemplates = append(clusterTemplates, templates...)
		clusterTemplateVersions = append(clusterTemplateVersions, templateVersions...)
	}
	// have them sorted for deepEqual calls later
	sort.Strings(clusterTemplates)
	sort.Strings(clusterTemplateVersions)
	newRules := []v1.PolicyRule{
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{templateRule},
			ResourceNames: clusterTemplates,
			Verbs:         []string{"get", "list", "watch"},
		},
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{templateVersionRule},
			ResourceNames: clusterTemplateVersions,
			Verbs:         []string{"get", "list", "watch"},
		},
	}

	// Add these templates/templateversions to the clusterRole for this cluster's templates/templateVersions
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
	// Get all subjects for this CRB, which includes all users in this cluster, and in all projects of this cluster
	// get a list of all members/owners of this cluster using CRTBs
	crtbs, err := tm.crtbLister.List(clusterID, labels.NewSelector())
	if err != nil {
		return err
	}
	subjects := []v1.Subject{}
	subjectMap := make(map[v1.Subject]bool)
	for _, crtb := range crtbs {
		s, valid, err := buildSubjectFromRTB(crtb)
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

	// Also get all project members in this cluster
	var prtbs []*v3.ProjectRoleTemplateBinding
	projects, err := tm.projectLister.List(clusterID, labels.NewSelector())
	for _, p := range projects {
		prtbs, err = tm.prtbLister.List(p.Name, labels.NewSelector())
		if err != nil {
			return err
		}
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
