package cluster

import (
	"context"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type keyType string

var (
	clusterKey = keyType("cluster")
)

func GetCluster(ctx context.Context) *v3.Cluster {
	cluster, _ := ctx.Value(clusterKey).(*v3.Cluster)
	return cluster
}

func StoreCluster(ctx context.Context, cluster *v3.Cluster) context.Context {
	return context.WithValue(ctx, clusterKey, cluster)
}
