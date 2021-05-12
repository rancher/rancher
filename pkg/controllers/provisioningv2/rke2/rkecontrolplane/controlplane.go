package rkecontrolplane

import (
	"context"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterCache:              clients.Mgmt.Cluster().Cache(),
		rkeControlPlaneController: clients.RKE.RKEControlPlane(),
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(),
		"", "rke-control-plane", h.OnChange)
}

type handler struct {
	clusterCache              mgmtcontrollers.ClusterCache
	rkeControlPlaneController rkecontrollers.RKEControlPlaneController
}

func (h *handler) OnChange(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	status.ObservedGeneration = obj.Generation
	cluster, err := h.clusterCache.Get(obj.Spec.ManagementClusterName)
	if err != nil {
		h.rkeControlPlaneController.EnqueueAfter(obj.Namespace, obj.Name, 2*time.Second)
		return status, nil
	}

	status.Ready = condition.Cond("Ready").IsTrue(cluster)
	return status, nil
}
