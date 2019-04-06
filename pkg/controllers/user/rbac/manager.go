package rbac

import (
	"context"

	"github.com/rancher/norman/objectclient"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type Manager interface {
	EnsureBindings(ns string, roles map[string]*v3.RoleTemplate, binding interface{}, client *objectclient.ObjectClient,
		create CreateFn, list ListFn, convert ConvertFn) error
	GetherRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error
	EnsureRoles(rts map[string]*v3.RoleTemplate) error
}

type CreateFn func(objectMeta metav1.ObjectMeta, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) runtime.Object
type ListFn func(ns string, selector labels.Selector) ([]interface{}, error)
type ConvertFn func(i interface{}) (string, string, []rbacv1.Subject)

func (m *manager) EnsureBindings(ns string, roles map[string]*v3.RoleTemplate, binding interface{}, client *objectclient.ObjectClient,
	create CreateFn, list ListFn, convert ConvertFn) error {
	return m.ensureBindings(ns, roles, binding, client, create, list, convert)
}

func (m *manager) GetherRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error {
	return m.gatherRoles(rt, roleTemplates)
}

func (m *manager) EnsureRoles(rts map[string]*v3.RoleTemplate) error {
	return m.ensureRoles(rts)
}

func NewManager(ctx context.Context, workload *config.UserContext) Manager {
	return &manager{
		workload:      workload,
		prtbIndexer:   workload.Management.Management.ProjectRoleTemplateBindings("").Controller().Informer().GetIndexer(),
		crtbIndexer:   workload.Management.Management.ClusterRoleTemplateBindings("").Controller().Informer().GetIndexer(),
		nsIndexer:     workload.Core.Namespaces("").Controller().Informer().GetIndexer(),
		crIndexer:     workload.RBAC.ClusterRoles("").Controller().Informer().GetIndexer(),
		crbIndexer:    workload.RBAC.ClusterRoleBindings("").Controller().Informer().GetIndexer(),
		rtLister:      workload.Management.Management.RoleTemplates("").Controller().Lister(),
		rbLister:      workload.RBAC.RoleBindings("").Controller().Lister(),
		crbLister:     workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:      workload.RBAC.ClusterRoles("").Controller().Lister(),
		nsLister:      workload.Core.Namespaces("").Controller().Lister(),
		nsController:  workload.Core.Namespaces("").Controller(),
		clusterLister: workload.Management.Management.Clusters("").Controller().Lister(),
		projectLister: workload.Management.Management.Projects("").Controller().Lister(),
		clusterName:   workload.ClusterName,
	}
}
