package rkecontrolplane

import (
	"context"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

var Ready condition.Cond = "Ready"

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterCache:              clients.Mgmt.Cluster().Cache(),
		rkeControlPlaneController: clients.RKE.RKEControlPlane(),
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(),
		"", "rke-control-plane", h.OnChange)
	relatedresource.Watch(ctx, "rke-control-plane-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		return []relatedresource.Key{{
			Namespace: namespace,
			Name:      name,
		}}, nil
	}, clients.RKE.RKEControlPlane(), clients.Mgmt.Cluster())
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

	Ready.SetStatus(&status, Ready.GetStatus(cluster))
	Ready.Reason(&status, Ready.GetReason(cluster))
	Ready.Message(&status, Ready.GetMessage(cluster))

	status.Ready = Ready.IsTrue(cluster)
	status.Initialized = Ready.IsTrue(cluster)
	return status, nil
}
