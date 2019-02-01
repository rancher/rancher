package multiclusterapp

import (
	"context"
	"fmt"
	access "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/rancher/pkg/controllers/management/globalnamespacerbac"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strings"

	"github.com/rancher/types/client/management/v3"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type MCAppController struct {
	multiClusterApps  v3.MultiClusterAppInterface
	managementContext *config.ManagementContext
	prtbs             v3.ProjectRoleTemplateBindingInterface
	prtbLister        v3.ProjectRoleTemplateBindingLister
	rtLister          v3.RoleTemplateLister
	gDNSs             v3.GlobalDNSInterface
}

type MCAppRevisionController struct {
	managementContext *config.ManagementContext
}

type ProjectController struct {
	mcAppsLister  v3.MultiClusterAppLister
	mcApps        v3.MultiClusterAppInterface
	clusters      v3.ClusterInterface
	clusterLister v3.ClusterLister
	projectLister v3.ProjectLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	m := MCAppController{
		multiClusterApps:  management.Management.MultiClusterApps(""),
		managementContext: management,
		prtbs:             management.Management.ProjectRoleTemplateBindings(""),
		prtbLister:        management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		rtLister:          management.Management.RoleTemplates("").Controller().Lister(),
		gDNSs:             management.Management.GlobalDNSs(""),
	}
	r := MCAppRevisionController{
		managementContext: management,
	}
	projects := management.Management.Projects("")
	p := ProjectController{
		mcAppsLister:  management.Management.MultiClusterApps("").Controller().Lister(),
		mcApps:        management.Management.MultiClusterApps(""),
		clusters:      management.Management.Clusters(""),
		clusterLister: management.Management.Clusters("").Controller().Lister(),
		projectLister: projects.Controller().Lister(),
	}
	m.multiClusterApps.AddHandler(ctx, "management-multiclusterapp-controller", m.sync)
	management.Management.MultiClusterAppRevisions("").AddHandler(ctx, "management-multiclusterapp-revisions-rbac", r.sync)
	m.prtbs.AddHandler(ctx, "management-prtb-controller-global-resource", m.prtbSync)
	projects.AddHandler(ctx, "management-mcapp-project-controller", p.sync)
}

func (mc *MCAppController) sync(key string, mcapp *v3.MultiClusterApp) (runtime.Object, error) {
	if mcapp == nil || mcapp.DeletionTimestamp != nil {
		return nil, nil
	}
	metaAccessor, err := meta.Accessor(mcapp)
	if err != nil {
		return mcapp, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[globalnamespacerbac.CreatorIDAnn]
	if !ok {
		return mcapp, fmt.Errorf("MultiClusterApp %v has no creatorId annotation. Cannot create apps for %v", metaAccessor.GetName(), mcapp.Name)
	}

	// check if all member groups have access to target projects
	groups := globalnamespacerbac.GetMemberGroups(mcapp.Spec.Members)
	var targets []string
	for _, t := range mcapp.Spec.Targets {
		targets = append(targets, t.ProjectName)
	}
	currentMembers := globalnamespacerbac.GetCurrentMembers(mcapp.Spec.Members)
	updatedMembers, err := globalnamespacerbac.GetUpdatedMembers(targets, mcapp.Spec.Members, mc.prtbLister)
	if err := access.CheckGroupAccess(groups, targets, mc.prtbLister, mc.rtLister, pv3.AppGroupVersionKind.Group, client.MultiClusterAppType); err != nil {
		return nil, err
	}
	if err := globalnamespacerbac.CreateRoleAndRoleBinding(globalnamespacerbac.MultiClusterAppResource, mcapp.Name, mcapp.UID,
		updatedMembers, creatorID, mc.managementContext); err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(updatedMembers, currentMembers) {
		toUpdate := mcapp.DeepCopy()
		toUpdate.Spec.Members = updatedMembers
		_, err := mc.multiClusterApps.Update(toUpdate)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (mc *MCAppController) prtbSync(key string, prtb *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	if prtb == nil || prtb.DeletionTimestamp != nil {
		mcapps, err := mc.multiClusterApps.Controller().Lister().List(namespace.GlobalNamespace, labels.NewSelector())
		if err != nil {
			return nil, err
		}
		for _, mcapp := range mcapps {
			mc.multiClusterApps.Controller().Enqueue(namespace.GlobalNamespace, mcapp.Name)
		}
		gdnses, err := mc.gDNSs.Controller().Lister().List(namespace.GlobalNamespace, labels.NewSelector())
		if err != nil {
			return nil, err
		}
		for _, gdns := range gdnses {
			if len(gdns.Spec.ProjectNames) == 0 {
				continue
			}
			mc.gDNSs.Controller().Enqueue(namespace.GlobalNamespace, gdns.Name)
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
	creatorID, ok := metaAccessor.GetAnnotations()[globalnamespacerbac.CreatorIDAnn]
	if !ok {
		return mcappRevision, fmt.Errorf("mcapp revision %v has no creatorId annotation", mcappRevision.Name)
	}
	if err := globalnamespacerbac.CreateRoleAndRoleBinding(
		globalnamespacerbac.MultiClusterAppRevisionResource, mcappRevision.Name, mcappRevision.UID, []v3.Member{}, creatorID,
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
				if err := p.updateAnswers(clusterName, key, toUpdate); err != nil {
					return project, err
				}
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

func (p *ProjectController) updateAnswers(clusterName string, projectName string, mcapp *v3.MultiClusterApp) error {
	deleted, err := p.clusterDeleted(clusterName, mcapp)
	if err != nil {
		return err
	}
	if deleted {
		for i := len(mcapp.Spec.Answers) - 1; i >= 0; i-- {
			projClusterName, _ := ref.Parse(mcapp.Spec.Answers[i].ProjectName)
			if mcapp.Spec.Answers[i].ClusterName == clusterName || projClusterName == clusterName {
				mcapp.Spec.Answers = append(mcapp.Spec.Answers[:i], mcapp.Spec.Answers[i+1:]...)
			}
		}
		return nil
	}
	for i := len(mcapp.Spec.Answers) - 1; i >= 0; i-- {
		if mcapp.Spec.Answers[i].ProjectName == projectName {
			mcapp.Spec.Answers = append(mcapp.Spec.Answers[:i], mcapp.Spec.Answers[i+1:]...)
			break
		}
	}
	return nil
}

func (p *ProjectController) clusterDeleted(clusterName string, mcapp *v3.MultiClusterApp) (bool, error) {
	cluster, err := p.clusterLister.Get("", clusterName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}
		//  check if cluster override is there in targets, if yes then make sure it is deleted
		if clusterOverrideExists(clusterName, mcapp) {
			cluster, err = p.clusters.Get(clusterName, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					return false, err
				}
				return true, nil
			}
		}

	}
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return true, nil
	}
	return false, nil
}

func clusterOverrideExists(clusterName string, mcapp *v3.MultiClusterApp) bool {
	for _, ans := range mcapp.Spec.Answers {
		if ans.ClusterName == clusterName && ans.ProjectName == "" {
			return true
		}
	}
	return false
}
