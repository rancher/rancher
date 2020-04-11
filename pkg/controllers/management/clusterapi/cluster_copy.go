package clusterapi

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/clustermanager"
	apiv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
	rancherCluster := &apiv3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "c-",
		},
	}
	h.RancherClusterClient.Create(rancherCluster)
	return cluster, nil
}
