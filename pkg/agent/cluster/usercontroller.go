package cluster

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func StartUserController(ctx context.Context, manager *clustermanager.Manager, cluster *v3.Cluster) error {
	return manager.Start(ctx, cluster, true)
}
