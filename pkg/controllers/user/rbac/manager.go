package rbac

import (
	"context"

	"github.com/rancher/norman/objectclient"
	nsutils "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type Manager interface {
	EnsureBindings(ns string, roles map[string]*v3.RoleTemplate, binding interface{}, client *objectclient.ObjectClient,
		create CreateFn, list ListFn, convert ConvertFn) error
	GatherRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error
	EnsureRoles(rts map[string]*v3.RoleTemplate) error
	RoleTemplateLister() v3.RoleTemplateLister
	NamespaceIndexer() cache.Indexer
	UserContext() *config.UserContext
	RoleBindingLister() typesrbacv1.RoleBindingLister
	ReconcileProjectAccessToGlobalResources(binding interface{}, rts map[string]*v3.RoleTemplate) error
	ReconcileProjectAccessToGlobalResourcesForDelete(binding interface{}) error
}

type CreateFn func(objectMeta metav1.ObjectMeta, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) runtime.Object
type ListFn func(ns string, selector labels.Selector) ([]interface{}, error)
type ConvertFn func(i interface{}) (string, string, []rbacv1.Subject)

func (m *manager) EnsureBindings(ns string, roles map[string]*v3.RoleTemplate, binding interface{}, client *objectclient.ObjectClient,
	create CreateFn, list ListFn, convert ConvertFn) error {
	return m.ensureBindings(ns, roles, binding, client, create, list, convert)
}

func (m *manager) RoleTemplateLister() v3.RoleTemplateLister {
	return m.rtLister
}

func (m *manager) NamespaceIndexer() cache.Indexer {
	return m.nsIndexer
}

func (m *manager) GatherRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error {
	return m.gatherRoles(rt, roleTemplates)
}

func (m *manager) EnsureRoles(rts map[string]*v3.RoleTemplate) error {
	return m.ensureRoles(rts)
}

func (m *manager) UserContext() *config.UserContext {
	return m.workload
}

func (m *manager) RoleBindingLister() typesrbacv1.RoleBindingLister {
	return m.rbLister
}

func (m *manager) ReconcileProjectAccessToGlobalResources(binding interface{}, rts map[string]*v3.RoleTemplate) error {
	return m.reconcileProjectAccessToGlobalResources(binding, rts)
}

func (m *manager) ReconcileProjectAccessToGlobalResourcesForDelete(binding interface{}) error {
	return m.reconcileProjectAccessToGlobalResourcesForDelete(binding)
}

func NewManager(ctx context.Context, workload *config.UserContext) Manager {
	// Add cache informer to project role template bindings
	prtbInformer := workload.Management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbByProjectIndex:               prtbByProjectName,
		prtbByProjecSubjectIndex:         prtbByProjectAndSubject,
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
		prtbByUIDIndex:                   prtbByUID,
	}
	prtbInformer.AddIndexers(prtbIndexers)

	crtbInformer := workload.Management.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
	}
	crtbInformer.AddIndexers(crtbIndexers)

	// Index for looking up namespaces by projectID annotation
	nsInformer := workload.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsByProjectIndex: nsutils.NsByProjectID,
	}
	nsInformer.AddIndexers(nsIndexers)

	// Get ClusterRoles by the namespaces the authorizes because they are in a project
	crInformer := workload.RBAC.ClusterRoles("").Controller().Informer()
	crIndexers := map[string]cache.IndexFunc{
		crByNSIndex: crByNS,
	}
	crInformer.AddIndexers(crIndexers)

	// Get ClusterRoleBindings by subject name and kind
	crbInformer := workload.RBAC.ClusterRoleBindings("").Controller().Informer()
	crbIndexers := map[string]cache.IndexFunc{
		crbByRoleAndSubjectIndex: crbByRoleAndSubject,
	}
	crbInformer.AddIndexers(crbIndexers)

	appInformer := workload.Management.Project.Apps("").Controller().Informer()
	appIndexers := map[string]cache.IndexFunc{
		appByUIDIndex: appByUID,
	}
	appInformer.AddIndexers(appIndexers)

	return &manager{
		workload:      workload,
		appIndexer:    appInformer.GetIndexer(),
		prtbIndexer:   prtbInformer.GetIndexer(),
		crtbIndexer:   crtbInformer.GetIndexer(),
		nsIndexer:     nsInformer.GetIndexer(),
		crIndexer:     crInformer.GetIndexer(),
		crbIndexer:    crbInformer.GetIndexer(),
		rtLister:      workload.Management.Management.RoleTemplates("").Controller().Lister(),
		rbLister:      workload.RBAC.RoleBindings("").Controller().Lister(),
		crbLister:     workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:      workload.RBAC.ClusterRoles("").Controller().Lister(),
		nsLister:      workload.Core.Namespaces("").Controller().Lister(),
		nsController:  workload.Core.Namespaces("").Controller(),
		clusterLister: workload.Management.Management.Clusters("").Controller().Lister(),
		projectLister: workload.Management.Management.Projects("").Controller().Lister(),
		appLister:     workload.Management.Project.Apps("").Controller().Lister(),
		clusterName:   workload.ClusterName,
	}

}
