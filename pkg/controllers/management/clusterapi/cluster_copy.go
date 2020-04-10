package clusterapi

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
)

type handler struct {
	RancherClusterClient v3.ClusterClient
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext, manager *clustermanager.Manager) {
	h := &handler{
	}

	wContext.V1alpha3.Cluster().OnChange(ctx, "clusterapi-copier", h.onClusterChange)
}

func (h *handler) onClusterChange(key string, cluster *v1alpha3.Cluster) (*v1alpha3.Cluster, error) {
	fmt.Println("made it")
	return cluster, nil
}
