package multiclusterapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/namespace"
	"github.com/rancher/types/user"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type MCAppController struct {
	multiClusterApps              v3.MultiClusterAppInterface
	multiClusterAppRevisionLister v3.MultiClusterAppRevisionLister
	managementContext             *config.ManagementContext
	prtbs                         v3.ProjectRoleTemplateBindingInterface
	prtbLister                    v3.ProjectRoleTemplateBindingLister
	crtbs                         v3.ClusterRoleTemplateBindingInterface
	crtbLister                    v3.ClusterRoleTemplateBindingLister
	rtLister                      v3.RoleTemplateLister
	users                         v3.UserInterface
	userManager                   user.Manager
}

type MCAppRevisionController struct {
	managementContext     *config.ManagementContext
	multiClusterAppLister v3.MultiClusterAppLister
}

type ProjectController struct {
	mcAppsLister  v3.MultiClusterAppLister
	mcApps        v3.MultiClusterAppInterface
	projectLister v3.ProjectLister
}

type ClusterController struct {
	mcAppsLister  v3.MultiClusterAppLister
	mcApps        v3.MultiClusterAppInterface
	clusterLister v3.ClusterLister
}

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	mcApps := management.Management.MultiClusterApps("")
	m := MCAppController{
		multiClusterApps:              mcApps,
		multiClusterAppRevisionLister: management.Management.MultiClusterAppRevisions("").Controller().Lister(),
		managementContext:             management,
		prtbs:                         management.Management.ProjectRoleTemplateBindings(""),
		prtbLister:                    management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		crtbs:                         management.Management.ClusterRoleTemplateBindings(""),
		crtbLister:                    management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		rtLister:                      management.Management.RoleTemplates("").Controller().Lister(),
		userManager:                   management.UserManager,
		users:                         management.Management.Users(""),
	}
	r := MCAppRevisionController{
		managementContext:     management,
		multiClusterAppLister: management.Management.MultiClusterApps("").Controller().Lister(),
	}
	projects := management.Management.Projects("")
	p := ProjectController{
		mcAppsLister:  mcApps.Controller().Lister(),
		mcApps:        mcApps,
		projectLister: projects.Controller().Lister(),
	}
	clusters := management.Management.Clusters("")
	c := ClusterController{
		mcAppsLister:  mcApps.Controller().Lister(),
		mcApps:        mcApps,
		clusterLister: clusters.Controller().Lister(),
	}
	m.multiClusterApps.AddHandler(ctx, "management-multiclusterapp-rbac-controller", m.sync)
	management.Management.MultiClusterAppRevisions("").AddHandler(ctx, "management-multiclusterapp-revisions-rbac", r.sync)
	projects.AddHandler(ctx, "management-mcapp-project-controller", p.sync)
	clusters.AddHandler(ctx, "management-mcapp-cluster-controller", c.sync)

	StartMCAppStateController(ctx, management)
	StartMCAppManagementController(ctx, management, clusterManager)
	StartMCAppEnqueueController(ctx, management)
}

func (mc *MCAppController) sync(key string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	if mcapp == nil || mcapp.DeletionTimestamp != nil {
		// multiclusterapp is being deleted, remove the sys acc created for it
		_, mcappName, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, err
		}
		u, err := mc.userManager.GetUserByPrincipalID(fmt.Sprintf("system://%s", mcappName))
		if err != nil {
			return nil, err
		}
		if u == nil {
			// user not found, must have been removed
			return nil, nil
		}
		if err := mc.users.Delete(u.Name, &v1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
			return nil, err
		}
		return nil, nil
	}
	metaAccessor, err := meta.Accessor(mcapp)
	if err != nil {
		return mcapp, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[rbac.CreatorIDAnn]
	if !ok {
		return mcapp, fmt.Errorf("MultiClusterApp %v has no creatorId annotation. Cannot create apps for %v", metaAccessor.GetName(), mcapp.Name)
	}

	systemUserPrincipalID := fmt.Sprintf("system://%s", mcapp.Name)
	systemUser, err := mc.userManager.EnsureUser(systemUserPrincipalID, "System account for Multiclusterapp "+mcapp.Name)
	if err != nil {
		return nil, err
	}
	var prtbName, crtbName string
	// create PRTBs with this service account for all roles of multiclusterapp
	for _, r := range mcapp.Spec.Roles {
		rt, err := mc.rtLister.Get("", r)
		if err != nil {
			return nil, err
		}

		for _, p := range mcapp.Spec.Targets {
			if p.ProjectName == "" {
				continue
			}
			split := strings.SplitN(p.ProjectName, ":", 2)
			if len(split) != 2 {
				return nil, fmt.Errorf("invalid project name")
			}
			clusterName := split[0]
			projectName := split[1]

			if rt.Context == "project" {
				prtbName = mcapp.Name + "-" + r
				// check if these prtbs already exist
				if err = mc.createPrtb(projectName, p.ProjectName, prtbName, r, systemUser.Name, systemUserPrincipalID); err != nil {
					return nil, err
				}
			} else if rt.Context == "cluster" {
				crtbName = mcapp.Name + "-" + r
				mc.createCrtb(clusterName, crtbName, r, systemUser.Name, systemUserPrincipalID)
			}
		}
	}

	if err := rbac.CreateRoleAndRoleBinding(rbac.MultiClusterAppResource, v3.MultiClusterAppGroupVersionKind.Kind, mcapp.Name, namespace.GlobalNamespace,
		rbac.RancherManagementAPIVersion, creatorID, []string{rbac.RancherManagementAPIGroup},
		mcapp.UID,
		mcapp.Spec.Members, mc.managementContext); err != nil {
		return nil, err
	}

	// see if any revision exists and call CreateRoleAndRoleBinding for it so that if this is an update request adding/removing members,
	// it reflects on the permissions for revisions too
	revisions, err := mc.multiClusterAppRevisionLister.List(namespace.GlobalNamespace, labels.SelectorFromSet(map[string]string{mcAppLabel: mcapp.Name}))
	if err != nil {
		return mcapp, err
	}
	for _, rev := range revisions {
		if err := rbac.CreateRoleAndRoleBinding(
			rbac.MultiClusterAppRevisionResource, v3.MultiClusterAppRevisionGroupVersionKind.Kind, rev.Name, namespace.GlobalNamespace, rbac.RancherManagementAPIVersion,
			creatorID, []string{rbac.RancherManagementAPIGroup}, rev.UID, mcapp.Spec.Members,
			mc.managementContext); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (r *MCAppRevisionController) sync(key string, mcappRevision *v3.MultiClusterAppRevision) (runtime.Object, error) {
	if mcappRevision == nil || mcappRevision.DeletionTimestamp != nil {
		return mcappRevision, nil
	}
	metaAccessor, err := meta.Accessor(mcappRevision)
	if err != nil {
		return mcappRevision, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[rbac.CreatorIDAnn]
	if !ok {
		return mcappRevision, fmt.Errorf("mcapp revision %v has no creatorId annotation", mcappRevision.Name)
	}
	// get the Members field from mcapp
	mcappName, ok := mcappRevision.Labels[mcAppLabel]
	if !ok {
		return mcappRevision, fmt.Errorf("mcapp revision created without setting mcapp label")
	}
	mcapp, err := r.multiClusterAppLister.Get(namespace.GlobalNamespace, mcappName)
	if err != nil {
		return mcappRevision, err
	}

	if err := rbac.CreateRoleAndRoleBinding(
		rbac.MultiClusterAppRevisionResource, v3.MultiClusterAppRevisionGroupVersionKind.Kind, mcappRevision.Name, namespace.GlobalNamespace, rbac.RancherManagementAPIVersion,
		creatorID, []string{rbac.RancherManagementAPIGroup}, mcappRevision.UID, mcapp.Spec.Members,
		r.managementContext); err != nil {
		return nil, err
	}

	return mcappRevision, nil
}

func (p *ProjectController) sync(key string, project *v3.Project) (runtime.Object, error) {
	if project != nil && project.DeletionTimestamp == nil {
		return project, nil
	}
	splitKey := strings.SplitN(key, "/", 2)
	if len(splitKey) != 2 || splitKey[0] == "" || splitKey[1] == "" {
		return project, fmt.Errorf("invalid project id %s", key)
	}
	clusterName, projectName := splitKey[0], splitKey[1]
	key = fmt.Sprintf("%s:%s", clusterName, projectName)
	mcApps, err := p.mcAppsLister.List(namespace.GlobalNamespace, labels.NewSelector())
	if err != nil {
		return project, err
	}
	for _, mcApp := range mcApps {
		if mcApp.DeletionTimestamp != nil {
			continue
		}
		var toUpdate *v3.MultiClusterApp
		for i, target := range mcApp.Spec.Targets {
			if target.ProjectName == key {
				toUpdate = mcApp.DeepCopy()
				toUpdate.Spec.Targets = append(toUpdate.Spec.Targets[:i], toUpdate.Spec.Targets[i+1:]...)
				p.updateAnswersForProject(key, toUpdate)
				break
			}
		}
		if toUpdate != nil {
			if _, err := p.mcApps.Update(toUpdate); err != nil {
				return project, fmt.Errorf("error updating mcapp %s for project %s", mcApp.Name, key)
			}
		}
	}
	return project, nil
}

func (p *ProjectController) updateAnswersForProject(projectName string, mcapp *v3.MultiClusterApp) {
	for i := len(mcapp.Spec.Answers) - 1; i >= 0; i-- {
		if mcapp.Spec.Answers[i].ProjectName == projectName {
			mcapp.Spec.Answers = append(mcapp.Spec.Answers[:i], mcapp.Spec.Answers[i+1:]...)
			break
		}
	}
}

func (c *ClusterController) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster != nil && cluster.DeletionTimestamp == nil {
		return cluster, nil
	}
	mcApps, err := c.mcAppsLister.List(namespace.GlobalNamespace, labels.NewSelector())
	if err != nil {
		return cluster, err
	}
	for _, mcApp := range mcApps {
		if mcApp.DeletionTimestamp != nil {
			continue
		}
		var toUpdate *v3.MultiClusterApp
		for i := len(mcApp.Spec.Targets) - 1; i >= 0; i-- {
			clusterName, _ := ref.Parse(mcApp.Spec.Targets[i].ProjectName)
			if clusterName == key {
				if toUpdate == nil {
					toUpdate = mcApp.DeepCopy()
				}
				toUpdate.Spec.Targets = append(toUpdate.Spec.Targets[:i], toUpdate.Spec.Targets[i+1:]...)
			}
		}
		if toUpdate != nil {
			c.updateAnswersForCluster(key, toUpdate)
			if _, err := c.mcApps.Update(toUpdate); err != nil {
				return cluster, fmt.Errorf("error updating mcapp %s for cluster %s", mcApp.Name, key)
			}
		}
	}
	return cluster, nil
}

func (c *ClusterController) updateAnswersForCluster(clusterName string, mcapp *v3.MultiClusterApp) {
	for i := len(mcapp.Spec.Answers) - 1; i >= 0; i-- {
		projClusterName, _ := ref.Parse(mcapp.Spec.Answers[i].ProjectName)
		if mcapp.Spec.Answers[i].ClusterName == clusterName || projClusterName == clusterName {
			mcapp.Spec.Answers = append(mcapp.Spec.Answers[:i], mcapp.Spec.Answers[i+1:]...)
		}
	}
}

func (mc *MCAppController) createPrtb(projectName, projectID, prtbName, roleTemplateName, systemUserName, systemUserPrincipalID string) error {
	_, err := mc.prtbLister.Get(projectName, prtbName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := mc.prtbs.Create(&v3.ProjectRoleTemplateBinding{
				ObjectMeta: v1.ObjectMeta{
					Name:      prtbName,
					Namespace: projectName,
				},
				UserName:          systemUserName,
				UserPrincipalName: systemUserPrincipalID,
				RoleTemplateName:  roleTemplateName,
				ProjectName:       projectID,
			})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func (mc *MCAppController) createCrtb(clusterName, crtbName, roleTemplateName, systemUserName, systemUserPrincipalID string) error {
	_, err := mc.crtbLister.Get(clusterName, crtbName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := mc.crtbs.Create(&v3.ClusterRoleTemplateBinding{
				ObjectMeta: v1.ObjectMeta{
					Name:      crtbName,
					Namespace: clusterName,
				},
				UserName:          systemUserName,
				UserPrincipalName: systemUserPrincipalID,
				RoleTemplateName:  roleTemplateName,
				ClusterName:       clusterName,
			})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
