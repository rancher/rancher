package clustertemplate

import (
	"context"

	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	RevisionController   = "mgmt-cluster-template-revision-controller"
	clusterTemplateLabel = "io.cattle.field/clusterTemplateId"
)

type RevController struct {
	clusterTemplates              v3.ClusterTemplateInterface
	clusterTemplateLister         v3.ClusterTemplateLister
	clusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
}

func newRevController(ctx context.Context, mgmt *config.ManagementContext) *RevController {
	n := &RevController{
		clusterTemplates:              mgmt.Management.ClusterTemplates(namespace.GlobalNamespace),
		clusterTemplateLister:         mgmt.Management.ClusterTemplates(namespace.GlobalNamespace).Controller().Lister(),
		clusterTemplateRevisionLister: mgmt.Management.ClusterTemplateRevisions(namespace.GlobalNamespace).Controller().Lister(),
	}
	return n
}

func Register(ctx context.Context, management *config.ManagementContext) {
	n := newRevController(ctx, management)
	if n != nil {
		management.Management.ClusterTemplateRevisions("").AddHandler(ctx, RevisionController, n.sync)
	}
	registerRbacControllers(ctx, management)
}

//sync is called periodically and on real updates
func (n *RevController) sync(key string, obj *v3.ClusterTemplateRevision) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.Spec.ClusterTemplateName == "" {
		return nil, nil
	}
	//load the template
	split := strings.SplitN(obj.Spec.ClusterTemplateName, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("error in splitting clusterTemplate name %v", obj.Spec.ClusterTemplateName)
	}
	templateName := split[1]
	template, err := n.clusterTemplateLister.Get(namespace.GlobalNamespace, templateName)
	if err != nil {
		return nil, err
	}
	if template.Spec.DefaultRevisionName != "" {
		return nil, nil
	}
	//if default is not set, set the revision to this revision if only one found
	set := labels.Set(map[string]string{clusterTemplateLabel: template.Spec.DisplayName})
	revisionList, err := n.clusterTemplateRevisionLister.List(namespace.GlobalNamespace, set.AsSelector())
	if err != nil {
		return nil, err
	}
	if len(revisionList) == 1 {
		templateCopy := template.DeepCopy()
		templateCopy.Spec.DefaultRevisionName = namespace.GlobalNamespace + ":" + obj.Name
		_, err := n.clusterTemplates.Update(templateCopy)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
