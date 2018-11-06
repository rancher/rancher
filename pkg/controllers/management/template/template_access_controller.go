package template

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	log "github.com/sirupsen/logrus"

	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/cache"
)

const (
	catalogTypeLabel         = "catalog.cattle.io/catalog_type"
	templateNameLabel        = "catalog.cattle.io/template_name"
	clusterRoleSuffix        = "-use-templates-templateversions"
	clusterRoleBindingSuffix = "-templates-templateversions-crb"
	templateRule             = "templates"
	templateVersionRule      = "templateversions"
)

// RBACTemplateManager controller manages access to the templates and templateversions of global, cluster and project level catalogs, via ClusterRoles and ClusterRoleBindings.
// A clusterRole is created for each cluster, and it includes all templates and templateversions of all cluster-level catalogs created within this cluster as rules
// These templates/templateversions are added via resourceNames
type RBACTemplateManager struct {
	templateLister        v3.TemplateLister
	templateVersionLister v3.TemplateVersionLister
	clusterCatalogLister  v3.ClusterCatalogLister
	clusterLister         v3.ClusterLister
	crtbLister            v3.ClusterRoleTemplateBindingLister
	projectCatalogLister  v3.ProjectCatalogLister
	projectLister         v3.ProjectLister
	prtbLister            v3.ProjectRoleTemplateBindingLister
	crbClient             rrbacv1.ClusterRoleBindingInterface
	crbLister             rrbacv1.ClusterRoleBindingLister
	clusterRoleClient     rrbacv1.ClusterRoleInterface
	clusterRoleLister     rrbacv1.ClusterRoleLister
	globalRoleClient      v3.GlobalRoleInterface
}

func Register(ctx context.Context, management *config.ManagementContext) {
	t := RBACTemplateManager{
		templateLister:        management.Management.Templates("").Controller().Lister(),
		templateVersionLister: management.Management.TemplateVersions("").Controller().Lister(),
		clusterCatalogLister:  management.Management.ClusterCatalogs("").Controller().Lister(),
		clusterLister:         management.Management.Clusters("").Controller().Lister(),
		crtbLister:            management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		projectCatalogLister:  management.Management.ProjectCatalogs("").Controller().Lister(),
		prtbLister:            management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		projectLister:         management.Management.Projects("").Controller().Lister(),
		crbClient:             management.RBAC.ClusterRoleBindings(""),
		crbLister:             management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		clusterRoleClient:     management.RBAC.ClusterRoles(""),
		clusterRoleLister:     management.RBAC.ClusterRoles("").Controller().Lister(),
		globalRoleClient:      management.Management.GlobalRoles(""),
	}

	management.Management.Catalogs("").Controller().AddHandler(ctx, "createGlobalTemplateBinding", t.syncForGlobalCatalog)
	management.Management.ClusterCatalogs("").Controller().AddHandler(ctx, "createClusterTemplateBinding", t.syncForClusterCatalog)
	management.Management.ProjectCatalogs("").Controller().AddHandler(ctx, "createProjectTemplateBinding", t.syncForProjectCatalog)
	management.Management.ClusterRoleTemplateBindings("").Controller().AddHandler(ctx, "addUserToClusterCatalog", t.syncCRBT)
	management.Management.ProjectRoleTemplateBindings("").Controller().AddHandler(ctx, "addUserToProjectCatalog", t.syncPRTB)
	management.Management.Clusters("").Controller().AddHandler(ctx, "syncClusters", t.syncCluster)
	management.Management.Projects("").Controller().AddHandler(ctx, "syncProjects", t.syncProject)
}

func buildSubjectFromRTB(binding interface{}) (v1.Subject, bool, error) {
	var userName, groupPrincipalName, groupName, name, kind string
	if rtb, ok := binding.(*v3.ProjectRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
		// only create subjects to add to the projectcatalog's templates bindings, if the roletemplate on the current rtb should have access to project catalogs
		if !rolesWithAccessToProjectCatalogTemplates[rtb.RoleTemplateName] {
			return v1.Subject{}, false, nil
		}
	} else if rtb, ok := binding.(*v3.ClusterRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
		// only create subjects to add to the clustercatalog's templates bindings, if the roletemplate on the current rtb should have access to cluster catalogs
		if !rolesWithAccessToClusterCatalogTemplates[rtb.RoleTemplateName] {
			return v1.Subject{}, false, nil
		}
	} else {
		return v1.Subject{}, false, fmt.Errorf("unrecognized roleTemplateBinding type: %v", binding)
	}

	if userName != "" {
		name = userName
		kind = "User"
	}

	if groupPrincipalName != "" {
		if name != "" {
			return v1.Subject{}, false, fmt.Errorf("roletemplatebinding has more than one subject fields set: %v", binding)
		}
		name = groupPrincipalName
		kind = "Group"
	}

	if groupName != "" {
		if name != "" {
			return v1.Subject{}, false, fmt.Errorf("roletemplatebinding has more than one subject fields set: %v", binding)
		}
		name = groupName
		kind = "Group"
	}

	if name == "" {
		return v1.Subject{}, false, fmt.Errorf("roletemplatebinding doesn't have any subject fields set: %v", binding)
	}

	return v1.Subject{
		Kind: kind,
		Name: name,
	}, true, nil
}

func (tm *RBACTemplateManager) getTemplateAndTemplateVersions(r *labels.Requirement) (tempArray []string, tempVersionArray []string, err error) {
	templates, err := tm.templateLister.List("", labels.NewSelector().Add(*r))
	if len(templates) == 0 {
		return []string{}, []string{}, nil
	}
	if err != nil {
		return []string{}, []string{}, nil
	}
	tempArray = []string{}
	for _, t := range templates {
		tempArray = append(tempArray, t.Name)
	}
	//get template versions for these templates
	selector := labels.NewSelector()
	newSelector := labels.NewSelector()
	for _, t := range templates {
		r, err := labels.NewRequirement(templateNameLabel, selection.Equals, []string{t.Name})
		if err != nil {
			return []string{}, []string{}, nil
		}
		selector = selector.Add(*r)
		templateVersions, err := tm.templateVersionLister.List("", selector)
		for _, tv := range templateVersions {
			tempVersionArray = append(tempVersionArray, tv.Name)
		}
		selector = newSelector
	}
	return tempArray, tempVersionArray, err
}

func (tm *RBACTemplateManager) syncForClusterCatalog(key string, obj *v3.ClusterCatalog) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		// cluster catalog deleted, reconcile this cluster
		clusterName, _, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, err
		}
		return nil, tm.reconcileClusterCatalog(clusterName)
	}

	return nil, tm.reconcileClusterCatalog(obj.ClusterName)
}

func (tm *RBACTemplateManager) syncForProjectCatalog(key string, obj *v3.ProjectCatalog) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		// project catalog deleted, reconcile this project
		projectName, _, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, err
		}
		// find cluster this project belongs to
		clusters, err := tm.clusterLister.List("", labels.NewSelector())
		for _, c := range clusters {
			projects, err := tm.projectLister.List(c.Name, labels.NewSelector())
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				if p.Name == projectName {
					if err = tm.reconcileProjectCatalog(projectName, c.Name); err != nil {
						return nil, err
					}
				}
			}
		}
		return nil, nil
	}
	split := strings.SplitN(obj.ProjectName, ":", 2)
	if len(split) != 2 {
		return nil, nil
	}
	clusterName, projectName := split[0], split[1]

	return nil, tm.reconcileProjectCatalog(projectName, clusterName)
}

func (tm *RBACTemplateManager) syncCRBT(key string, obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	var clusterName string
	var err error
	if obj == nil || obj.DeletionTimestamp != nil {
		// crtb is being removed, remove the user of this crtb from any CRBs for the templates
		if key == "" {
			return nil, nil
		}
		clusterName, _, err = cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, err
		}
	} else {
		clusterName = obj.ClusterName
	}
	return nil, tm.reconcileClusterCatalog(clusterName)
}

func (tm *RBACTemplateManager) syncPRTB(key string, obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	var clusterName, projectName string

	if obj == nil || obj.DeletionTimestamp != nil {
		// reconcileClusterCatalog all clusters as it's impossible to find cluster that this PRTB belongs to,
		// we need to remove this user from CRB for templates of the project too
		clusters, err := tm.clusterLister.List("", labels.NewSelector())
		if err != nil {
			return nil, err
		}
		for _, c := range clusters {
			if err = tm.reconcileClusterCatalog(c.Name); err != nil {
				return nil, err
			}
		}
		if key == "" {
			return nil, nil
		}
		projectName, _, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, err
		}
		for _, c := range clusters {
			projects, err := tm.projectLister.List(c.Name, labels.NewSelector())
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				if p.Name == projectName {
					if err = tm.reconcileProjectCatalog(projectName, c.Name); err != nil {
						return nil, err
					}
				}
			}
		}
	} else {
		pName := strings.SplitN(obj.ProjectName, ":", 2)
		if len(pName) != 2 {
			log.Errorf("Project name incorrect")
			return nil, fmt.Errorf("project name incorrect")
		}
		clusterName, projectName = pName[0], pName[1]
		if err := tm.reconcileClusterCatalog(clusterName); err != nil {
			return nil, err
		}
		return nil, tm.reconcileProjectCatalog(projectName, clusterName)
	}
	return nil, nil
}

func (tm *RBACTemplateManager) syncCluster(key string, obj *v3.Cluster) (runtime.Object, error) {
	if key == "" && obj == nil {
		return nil, nil
	}
	return nil, tm.reconcileClusterCatalog(key)
}

func (tm *RBACTemplateManager) syncProject(key string, obj *v3.Project) (runtime.Object, error) {
	if key == "" && obj == nil {
		return nil, nil
	}
	split := strings.SplitN(key, "/", 2)
	if len(split) != 2 {
		return nil, nil
	}
	clusterName, projectName := split[0], split[1]
	return nil, tm.reconcileProjectCatalog(projectName, clusterName)
}

func (tm *RBACTemplateManager) updateCRB(crb *v1.ClusterRoleBinding, subjects []v1.Subject) error {
	crbToUpdate := crb.DeepCopy()
	crbToUpdate.Subjects = subjects
	_, err := tm.crbClient.Update(crbToUpdate)
	if err != nil {
		return err
	}
	return nil
}

func (tm *RBACTemplateManager) updateClusterRole(cRole *v1.ClusterRole, rules []v1.PolicyRule) error {
	cRoleToUpdate := cRole.DeepCopy()
	cRoleToUpdate.Rules = rules
	_, err := tm.clusterRoleClient.Update(cRoleToUpdate)
	if err != nil {
		return err
	}
	return nil
}
