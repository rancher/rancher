package authprovisioningv2

import (
	"context"
	"sync"

	"github.com/moby/locker"
	"github.com/rancher/lasso/pkg/dynamic"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	apiextcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
	rbacv1 "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/gvk"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	byClusterName          = "byClusterName"
	crbtByRoleTemplateName = "crbtByRoleTemplateName"
)

type handler struct {
	mgmtCtx                              *config.ManagementContext
	roleLocker                           locker.Locker
	roleCache                            rbacv1.RoleCache
	roleController                       rbacv1.RoleController
	roleBindingController                rbacv1.RoleBindingController
	clusterRoleCache                     rbacv1.ClusterRoleCache
	roleTemplateController               mgmtcontrollers.RoleTemplateController
	clusterRoleTemplateBindings          mgmtcontrollers.ClusterRoleTemplateBindingCache
	clusterRoleTemplateBindingController mgmtcontrollers.ClusterRoleTemplateBindingController
	projectRoleTemplateBindingController mgmtcontrollers.ProjectRoleTemplateBindingController
	roleTemplatesCache                   mgmtcontrollers.RoleTemplateCache
	clusters                             provisioningcontrollers.ClusterCache
	crdCache                             apiextcontrollers.CustomResourceDefinitionCache
	dynamic                              *dynamic.Controller
	resources                            map[schema.GroupVersionKind]resourceMatch
	resourcesList                        []resourceMatch
	resourcesLock                        sync.RWMutex
	apply                                apply.Apply
	roleBindingApply                     apply.Apply
	provisioningClusterGVK               schema.GroupVersionKind
}

func Register(ctx context.Context, clients *wrangler.Context, management *config.ManagementContext) error {
	clusterGVK, err := gvk.Get(&v1.Cluster{})
	if err != nil {
		// this is a build issue if it happens
		panic(err)
	}

	h := &handler{
		mgmtCtx:                              management,
		roleCache:                            clients.RBAC.Role().Cache(),
		roleController:                       clients.RBAC.Role(),
		roleBindingController:                clients.RBAC.RoleBinding(),
		clusterRoleCache:                     clients.RBAC.ClusterRole().Cache(),
		roleTemplateController:               clients.Mgmt.RoleTemplate(),
		clusterRoleTemplateBindings:          clients.Mgmt.ClusterRoleTemplateBinding().Cache(),
		clusterRoleTemplateBindingController: clients.Mgmt.ClusterRoleTemplateBinding(),
		projectRoleTemplateBindingController: clients.Mgmt.ProjectRoleTemplateBinding(),
		roleTemplatesCache:                   clients.Mgmt.RoleTemplate().Cache(),
		clusters:                             clients.Provisioning.Cluster().Cache(),
		crdCache:                             clients.CRD.CustomResourceDefinition().Cache(),
		dynamic:                              clients.Dynamic,
		apply: clients.Apply.WithCacheTypes(
			clients.Mgmt.RoleTemplate(),
			clients.RBAC.Role()),
		roleBindingApply: clients.Apply.WithCacheTypes(
			clients.Mgmt.ClusterRoleTemplateBinding(),
			clients.RBAC.RoleBinding()),
		resources:              map[schema.GroupVersionKind]resourceMatch{},
		provisioningClusterGVK: clusterGVK,
	}

	if err := h.initializeCRDs(clients.CRD.CustomResourceDefinition()); err != nil {
		return err
	}

	h.dynamic.AddIndexer(clusterIndexed, h.gvkMatcher, indexByCluster)
	h.dynamic.OnChange(ctx, "auth-prov-v2-trigger", h.gvkMatcher, h.OnClusterObjectChanged)
	clients.Mgmt.RoleTemplate().OnChange(ctx, "auth-prov-v2-roletemplate", h.OnChange)
	clients.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, "auth-prov-v2-crtb", h.OnCRTB)
	clients.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, "auth-prov-v2-prtb", h.OnPRTB)
	clients.Provisioning.Cluster().OnChange(ctx, "auth-prov-v2-cluster", h.OnCluster)
	clients.CRD.CustomResourceDefinition().OnChange(ctx, "auth-prov-v2-crd", h.OnCRD)
	if features.RKE2.Enabled() {
		clients.Dynamic.OnChange(ctx, "auth-prov-v2-rke-machine-config", validMachineConfigGVK, h.OnMachineConfigChange)
	}
	clients.Provisioning.Cluster().Cache().AddIndexer(byClusterName, func(obj *v1.Cluster) ([]string, error) {
		return []string{obj.Status.ClusterName}, nil
	})
	clients.Mgmt.ClusterRoleTemplateBinding().Cache().AddIndexer(crbtByRoleTemplateName, func(obj *v3.ClusterRoleTemplateBinding) ([]string, error) {
		return []string{obj.RoleTemplateName}, nil
	})
	return nil
}
