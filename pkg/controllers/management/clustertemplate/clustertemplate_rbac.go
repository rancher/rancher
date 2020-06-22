package clustertemplate

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/namespace"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	creatorIDAnn = "field.cattle.io/creatorId"
	ctLabel      = "io.cattle.field/clusterTemplateId"
)

type clusterTemplateController struct {
	clusterTemplates              v3.ClusterTemplateInterface
	clusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	managementContext             *config.ManagementContext
}

type clusterTemplateRevisionController struct {
	clusterTemplateRevisions      v3.ClusterTemplateRevisionInterface
	clusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	clusterTemplateLister         v3.ClusterTemplateLister
	managementContext             *config.ManagementContext
}

func registerRbacControllers(ctx context.Context, mgmt *config.ManagementContext) {
	ct := clusterTemplateController{
		managementContext:             mgmt,
		clusterTemplates:              mgmt.Management.ClusterTemplates(""),
		clusterTemplateRevisionLister: mgmt.Management.ClusterTemplateRevisions("").Controller().Lister(),
	}
	ct.clusterTemplates.AddHandler(ctx, "cluster-template-rbac-controller", ct.sync)

	ctr := clusterTemplateRevisionController{
		managementContext:             mgmt,
		clusterTemplateRevisions:      mgmt.Management.ClusterTemplateRevisions(""),
		clusterTemplateRevisionLister: mgmt.Management.ClusterTemplateRevisions("").Controller().Lister(),
		clusterTemplateLister:         mgmt.Management.ClusterTemplates("").Controller().Lister(),
	}
	ctr.clusterTemplateRevisions.AddHandler(ctx, "cluster-template-rev-rbac-controller", ctr.sync)
}

func (ct *clusterTemplateController) sync(key string, clusterTemplate *v3.ClusterTemplate) (runtime.Object, error) {
	if clusterTemplate == nil || clusterTemplate.DeletionTimestamp != nil {
		return nil, nil
	}
	metaAccessor, err := meta.Accessor(clusterTemplate)
	if err != nil {
		return clusterTemplate, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[rbac.CreatorIDAnn]
	if !ok {
		return clusterTemplate, fmt.Errorf("clusterTemplate %v has no creatorId annotation", metaAccessor.GetName())
	}

	if err := rbac.CreateRoleAndRoleBinding(rbac.ClusterTemplateResource, v3.ClusterTemplateGroupVersionKind.Kind, clusterTemplate.Name, namespace.GlobalNamespace,
		rbac.RancherManagementAPIVersion, creatorID, []string{rbac.RancherManagementAPIGroup},
		clusterTemplate.UID,
		clusterTemplate.Spec.Members, ct.managementContext); err != nil {
		return nil, err
	}

	//see if any revision exists and call CreateRoleAndRoleBinding for it so that if this is an update request adding/removing members,
	//it reflects on the permissions for revisions too
	revisions, err := ct.clusterTemplateRevisionLister.List(namespace.GlobalNamespace, labels.SelectorFromSet(map[string]string{ctLabel: clusterTemplate.Name}))
	if err != nil && !apierrors.IsNotFound(err) {
		return clusterTemplate, err
	}
	for _, rev := range revisions {
		ct.managementContext.Management.ClusterTemplateRevisions("").Controller().Enqueue(namespace.GlobalNamespace, rev.Name)
	}
	return clusterTemplate, nil
}

func (ctr *clusterTemplateRevisionController) sync(key string, clusterTemplateRev *v3.ClusterTemplateRevision) (runtime.Object, error) {
	if clusterTemplateRev == nil || clusterTemplateRev.DeletionTimestamp != nil {
		return nil, nil
	}
	metaAccessor, err := meta.Accessor(clusterTemplateRev)
	if err != nil {
		return clusterTemplateRev, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[rbac.CreatorIDAnn]
	if !ok {
		return clusterTemplateRev, fmt.Errorf("clusterTemplateRevision %v has no creatorId annotation", metaAccessor.GetName())
	}
	// get members field from clusterTemplate
	split := strings.SplitN(clusterTemplateRev.Spec.ClusterTemplateName, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("error in splitting clusterTemplate name %v", clusterTemplateRev.Spec.ClusterTemplateName)
	}
	templateName := split[1]
	clusterTemp, err := ctr.clusterTemplateLister.Get(namespace.GlobalNamespace, templateName)
	if err != nil {
		return nil, err
	}
	if err := rbac.CreateRoleAndRoleBinding(rbac.ClusterTemplateRevisionResource, v3.ClusterTemplateRevisionGroupVersionKind.Kind, clusterTemplateRev.Name, namespace.GlobalNamespace,
		rbac.RancherManagementAPIVersion, creatorID, []string{rbac.RancherManagementAPIGroup},
		clusterTemplateRev.UID,
		clusterTemp.Spec.Members, ctr.managementContext); err != nil {
		return nil, err
	}
	return clusterTemplateRev, nil
}
