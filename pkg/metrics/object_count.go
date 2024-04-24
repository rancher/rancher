package metrics

import (
	"context"

	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type clusterObjectCounter interface {
	ConfigMaps(context.Context) (int64, error)
	Namespaces(context.Context) (int64, error)
	Nodes(context.Context) (int64, error)
	Secrets(context.Context) (int64, error)

	AppRevisions(context.Context) (int64, error)

	ClusterRoleBindings(context.Context) (int64, error)
	RoleBindings(context.Context) (int64, error)

	CatalogTemplateVersions(context.Context) (int64, error)
	Projects(context.Context) (int64, error)
}

type clusterCountClient struct {
	dynamic dynamic.Interface
}

func (h *metricsHandler) getClusterClient(clusterID string) (clusterObjectCounter, error) {
	cluster, err := h.clusterManager.UserContext(clusterID)
	if err != nil {
		return nil, err
	}
	return &clusterCountClient{
		dynamic: dynamic.New(cluster.UnversionedClient),
	}, nil
}

func (c clusterCountClient) countAllObjects(ctx context.Context, gvr schema.GroupVersionResource) (int64, error) {
	list, err := c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		if errors.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	n := int64(len(list.Items))
	if remaining := list.GetRemainingItemCount(); remaining != nil {
		n += *remaining
	}
	return n, nil
}

func (c clusterCountClient) ConfigMaps(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, corev1.ConfigMapGroupVersionResource)
}

func (c clusterCountClient) Namespaces(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, corev1.NamespaceGroupVersionResource)
}

func (c clusterCountClient) Nodes(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, corev1.NodeGroupVersionResource)
}

func (c clusterCountClient) Secrets(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, corev1.SecretGroupVersionResource)
}

func (c clusterCountClient) AppRevisions(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, projectv3.AppRevisionGroupVersionResource)
}

func (c clusterCountClient) ClusterRoleBindings(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, rbacv1.ClusterRoleBindingGroupVersionResource)
}

func (c clusterCountClient) RoleBindings(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, rbacv1.RoleBindingGroupVersionResource)
}

func (c clusterCountClient) CatalogTemplateVersions(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, managementv3.CatalogTemplateVersionGroupVersionResource)
}

func (c clusterCountClient) Projects(ctx context.Context) (int64, error) {
	return c.countAllObjects(ctx, managementv3.ProjectGroupVersionResource)
}
