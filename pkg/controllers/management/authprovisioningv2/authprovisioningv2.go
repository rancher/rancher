package authprovisioningv2

import (
	"context"
	"sync"

	"github.com/moby/locker"
	"github.com/rancher/lasso/pkg/dynamic"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	apiextcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	byClusterName          = "byClusterName"
	crbtByRoleTemplateName = "crbtByRoleTemplateName"
)

type handler struct {
	roleLocker                           locker.Locker
	roleTemplateController               mgmtcontrollers.RoleTemplateController
	clusterRoleTemplateBindings          mgmtcontrollers.ClusterRoleTemplateBindingCache
	clusterRoleTemplateBindingController mgmtcontrollers.ClusterRoleTemplateBindingController
	roleTemplates                        mgmtcontrollers.RoleTemplateCache
	clusters                             provisioningcontrollers.ClusterCache
	crdCache                             apiextcontrollers.CustomResourceDefinitionCache
	dynamic                              *dynamic.Controller
	resources                            map[schema.GroupVersionKind]resourceMatch
	resourcesList                        []resourceMatch
	resourcesLock                        sync.RWMutex
	apply                                apply.Apply
	roleBindingApply                     apply.Apply
}

func Register(ctx context.Context, clients *wrangler.Context) error {
	h := &handler{
		roleTemplateController:               clients.Mgmt.RoleTemplate(),
		clusterRoleTemplateBindings:          clients.Mgmt.ClusterRoleTemplateBinding().Cache(),
		clusterRoleTemplateBindingController: clients.Mgmt.ClusterRoleTemplateBinding(),
		roleTemplates:                        clients.Mgmt.RoleTemplate().Cache(),
		clusters:                             clients.Provisioning.Cluster().Cache(),
		crdCache:                             clients.CRD.CustomResourceDefinition().Cache(),
		dynamic:                              clients.Dynamic,
		apply: clients.Apply.WithCacheTypes(
			clients.Mgmt.RoleTemplate(),
			clients.RBAC.Role()),
		roleBindingApply: clients.Apply.WithCacheTypes(
			clients.Mgmt.ClusterRoleTemplateBinding(),
			clients.RBAC.RoleBinding()),
		resources: map[schema.GroupVersionKind]resourceMatch{},
	}

	if err := h.initializeCRDs(clients.CRD.CustomResourceDefinition()); err != nil {
		return err
	}

	h.dynamic.AddIndexer(clusterIndexed, h.gvkMatcher, indexByCluster)
	h.dynamic.OnChange(ctx, "auth-prov-v2-trigger", h.gvkMatcher, h.OnClusterObjectChanged)
	clients.Mgmt.RoleTemplate().OnChange(ctx, "auth-prov-v2", h.OnChange)
	clients.CRD.CustomResourceDefinition().OnChange(ctx, "auth-prov-v2-crd", h.OnCRD)
	clients.Provisioning.Cluster().Cache().AddIndexer(byClusterName, func(obj *v1.Cluster) ([]string, error) {
		return []string{obj.Status.ClusterName}, nil
	})
	clients.Mgmt.ClusterRoleTemplateBinding().Cache().AddIndexer(crbtByRoleTemplateName, func(obj *v3.ClusterRoleTemplateBinding) ([]string, error) {
		return []string{obj.RoleTemplateName}, nil
	})
	return nil
}
