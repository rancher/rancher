package rkecluster

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
)

type handler struct {
	clusterClient rkecontroller.RKEClusterClient
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		clusterClient: clients.RKE.RKECluster(),
	}

	clients.RKE.RKECluster().OnChange(ctx, "rke", h.UpdateSpec)
	rkecontroller.RegisterRKEClusterStatusHandler(ctx,
		clients.RKE.RKECluster(),
		"Defined",
		"rke-cluster-status",
		h.OnChange)
}

func (h *handler) UpdateSpec(key string, cluster *v1.RKECluster) (*v1.RKECluster, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.Spec.ControlPlaneEndpoint == nil {
		cluster := cluster.DeepCopy()
		cluster.Spec.ControlPlaneEndpoint = &v1.Endpoint{
			Host: "localhost",
			Port: 6443,
		}
		return h.clusterClient.Update(cluster)
	}

	return cluster, nil
}

func (h *handler) OnChange(obj *v1.RKECluster, status v1.RKEClusterStatus) (v1.RKEClusterStatus, error) {
	status.Ready = condition.Cond("Provisioned").IsTrue(&status)
	return status, nil
}
