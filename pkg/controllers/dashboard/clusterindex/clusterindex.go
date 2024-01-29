package clusterindex

import (
	"context"

	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ClusterV1ByClusterV3Reference = "clusterByClusterReference"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	clusterCache := clients.Provisioning.Cluster().Cache()

	clusterCache.AddIndexer(ClusterV1ByClusterV3Reference, func(obj *rancherv1.Cluster) ([]string, error) {
		return []string{obj.Status.ClusterName}, nil
	})

	relatedresource.Watch(ctx, "cluster-v1-trigger", func(namespace, name string, obj runtime.Object) (result []relatedresource.Key, _ error) {
		clusters, err := clusterCache.GetByIndex(ClusterV1ByClusterV3Reference, name)
		if err != nil {
			return nil, err
		}
		for _, cluster := range clusters {
			result = append(result, relatedresource.Key{
				Namespace: cluster.Namespace,
				Name:      cluster.Name,
			})
		}
		return result, nil
	}, clients.Provisioning.Cluster(), clients.Mgmt.Cluster())
}
