package rkecluster

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/relatedresource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	Provisioned condition.Cond = "Provisioned"
)

type handler struct {
	clusterClient    rkecontroller.RKEClusterClient
	rkeControlPlanes rkecontroller.RKEControlPlaneCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		clusterClient:    clients.RKE.RKECluster(),
		rkeControlPlanes: clients.RKE.RKEControlPlane().Cache(),
	}

	clients.RKE.RKECluster().OnChange(ctx, "rke", h.UpdateSpec)
	rkecontroller.RegisterRKEClusterStatusHandler(ctx,
		clients.RKE.RKECluster(),
		"Defined",
		"rke-cluster-status",
		h.OnChange)
	relatedresource.Watch(ctx, "rke-cluster-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		return []relatedresource.Key{{
			Namespace: namespace,
			Name:      name,
		}}, nil
	}, clients.RKE.RKECluster(), clients.RKE.RKEControlPlane())
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
	cp, err := h.rkeControlPlanes.Get(obj.Namespace, obj.Name)
	if err == nil {
		Provisioned.SetStatus(&status, Provisioned.GetStatus(cp))
		Provisioned.Reason(&status, Provisioned.GetReason(cp))
		Provisioned.Message(&status, Provisioned.GetMessage(cp))
	} else if !apierrors.IsNotFound(err) {
		return status, err
	}

	status.Ready = condition.Cond("Provisioned").IsTrue(&status)
	status.ObservedGeneration = obj.Generation
	return status, nil
}
