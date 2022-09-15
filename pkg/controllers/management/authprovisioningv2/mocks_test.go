package authprovisioningv2

import (
	"context"
	"errors"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/generic"
	v1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type mockCluster struct{}

func (m mockCluster) Get(namespace, name string) (*provisioningv1.Cluster, error) {
	if name == "deleted" {
		return nil, apierror.NewNotFound(schema.GroupResource{}, name)
	}
	if name == "deleting" {
		c := &provisioningv1.Cluster{}
		c.DeletionTimestamp = &metav1.Time{}
		return c, nil
	}
	if name == "other error" {
		return nil, errors.New("other error")
	}
	return &provisioningv1.Cluster{}, nil
}
func (m mockCluster) List(namespace string, selector labels.Selector) ([]*provisioningv1.Cluster, error) {
	return nil, nil
}
func (m mockCluster) AddIndexer(indexName string, indexer provisioningcontrollers.ClusterIndexer) {}
func (m mockCluster) GetByIndex(indexName, key string) ([]*provisioningv1.Cluster, error) {
	return nil, nil
}

type mockMgmtCluster struct{}

func (m mockMgmtCluster) Get(name string) (*v3.Cluster, error) {
	if name == "deleted" {
		return nil, apierror.NewNotFound(schema.GroupResource{}, name)
	}
	if name == "deleting" {
		c := &v3.Cluster{}
		c.DeletionTimestamp = &metav1.Time{}
		return c, nil
	}
	if name == "other error" {
		return nil, errors.New("other error")
	}
	return &v3.Cluster{}, nil
}
func (m mockMgmtCluster) List(selector labels.Selector) ([]*v3.Cluster, error) {
	return []*v3.Cluster{}, nil
}
func (m mockMgmtCluster) AddIndexer(indexName string, indexer managementv3.ClusterIndexer) {}
func (m mockMgmtCluster) GetByIndex(indexName, key string) ([]*v3.Cluster, error) {
	return []*v3.Cluster{}, nil
}

type mockRoleController struct{}

func (m mockRoleController) OnRemove(ctx context.Context, name string, sync rbacv1.RoleHandler) {}
func (m mockRoleController) OnChange(ctx context.Context, name string, sync rbacv1.RoleHandler) {}
func (m mockRoleController) Enqueue(namespace, name string)                                     {}
func (m mockRoleController) EnqueueAfter(namespace, name string, duration time.Duration)        {}
func (m mockRoleController) Cache() rbacv1.RoleCache {
	return nil
}
func (m mockRoleController) Create(*v1.Role) (*v1.Role, error) {
	return nil, nil
}
func (m mockRoleController) Update(*v1.Role) (*v1.Role, error) {
	return nil, nil
}
func (m mockRoleController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return nil
}
func (m mockRoleController) Get(namespace, name string, options metav1.GetOptions) (*v1.Role, error) {
	return nil, nil
}
func (m mockRoleController) List(namespace string, opts metav1.ListOptions) (*v1.RoleList, error) {
	return nil, nil
}
func (m mockRoleController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (m mockRoleController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Role, err error) {
	return nil, nil
}
func (m mockRoleController) Informer() cache.SharedIndexInformer {
	return nil
}
func (m mockRoleController) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"}
}
func (m mockRoleController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockRoleController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockRoleController) Updater() generic.Updater {
	return nil
}

type mockRoleBindingController struct{}

func (m mockRoleBindingController) OnRemove(ctx context.Context, name string, sync rbacv1.RoleBindingHandler) {
}
func (m mockRoleBindingController) OnChange(ctx context.Context, name string, sync rbacv1.RoleBindingHandler) {
}
func (m mockRoleBindingController) Enqueue(namespace, name string)                              {}
func (m mockRoleBindingController) EnqueueAfter(namespace, name string, duration time.Duration) {}
func (m mockRoleBindingController) Cache() rbacv1.RoleBindingCache {
	return nil
}
func (m mockRoleBindingController) Create(*v1.RoleBinding) (*v1.RoleBinding, error) {
	return nil, nil
}
func (m mockRoleBindingController) Update(*v1.RoleBinding) (*v1.RoleBinding, error) {
	return nil, nil
}
func (m mockRoleBindingController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return nil
}
func (m mockRoleBindingController) Get(namespace, name string, options metav1.GetOptions) (*v1.RoleBinding, error) {
	return nil, nil
}
func (m mockRoleBindingController) List(namespace string, opts metav1.ListOptions) (*v1.RoleBindingList, error) {
	return nil, nil
}
func (m mockRoleBindingController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (m mockRoleBindingController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.RoleBinding, err error) {
	return nil, nil
}
func (m mockRoleBindingController) Informer() cache.SharedIndexInformer {
	return nil
}
func (m mockRoleBindingController) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"}
}
func (m mockRoleBindingController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockRoleBindingController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockRoleBindingController) Updater() generic.Updater {
	return nil
}

type mockClusterRoleController struct{}

func (m mockClusterRoleController) OnRemove(ctx context.Context, name string, sync rbacv1.ClusterRoleHandler) {
}
func (m mockClusterRoleController) OnChange(ctx context.Context, name string, sync rbacv1.ClusterRoleHandler) {
}
func (m mockClusterRoleController) Enqueue(name string)                              {}
func (m mockClusterRoleController) EnqueueAfter(name string, duration time.Duration) {}
func (m mockClusterRoleController) Cache() rbacv1.ClusterRoleCache {
	return nil
}
func (m mockClusterRoleController) Create(*v1.ClusterRole) (*v1.ClusterRole, error) {
	return nil, nil
}
func (m mockClusterRoleController) Update(*v1.ClusterRole) (*v1.ClusterRole, error) {
	return nil, nil
}
func (m mockClusterRoleController) Delete(name string, options *metav1.DeleteOptions) error {
	return nil
}
func (m mockClusterRoleController) Get(name string, options metav1.GetOptions) (*v1.ClusterRole, error) {
	return nil, nil
}
func (m mockClusterRoleController) List(opts metav1.ListOptions) (*v1.ClusterRoleList, error) {
	return nil, nil
}
func (m mockClusterRoleController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (m mockClusterRoleController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterRole, err error) {
	return nil, nil
}
func (m mockClusterRoleController) Informer() cache.SharedIndexInformer {
	return nil
}
func (m mockClusterRoleController) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}
}
func (m mockClusterRoleController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockClusterRoleController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockClusterRoleController) Updater() generic.Updater {
	return nil
}

type mockClusterRoleBindingController struct{}

func (m mockClusterRoleBindingController) OnRemove(ctx context.Context, name string, sync rbacv1.ClusterRoleBindingHandler) {
}
func (m mockClusterRoleBindingController) OnChange(ctx context.Context, name string, sync rbacv1.ClusterRoleBindingHandler) {
}
func (m mockClusterRoleBindingController) Enqueue(name string) {}
func (m mockClusterRoleBindingController) EnqueueAfter(name string, duration time.Duration) {
}
func (m mockClusterRoleBindingController) Cache() rbacv1.ClusterRoleBindingCache {
	return nil
}
func (m mockClusterRoleBindingController) Create(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	return nil, nil
}
func (m mockClusterRoleBindingController) Update(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	return nil, nil
}
func (m mockClusterRoleBindingController) Delete(name string, options *metav1.DeleteOptions) error {
	return nil
}
func (m mockClusterRoleBindingController) Get(name string, options metav1.GetOptions) (*v1.ClusterRoleBinding, error) {
	return nil, nil
}
func (m mockClusterRoleBindingController) List(opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error) {
	return nil, nil
}
func (m mockClusterRoleBindingController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (m mockClusterRoleBindingController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ClusterRoleBinding, err error) {
	return nil, nil
}
func (m mockClusterRoleBindingController) Informer() cache.SharedIndexInformer {
	return nil
}
func (m mockClusterRoleBindingController) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}
}
func (m mockClusterRoleBindingController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockClusterRoleBindingController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
}
func (m mockClusterRoleBindingController) Updater() generic.Updater {
	return nil
}
