package rkecluster

import (
	"context"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
)

type handler struct {
	rkeClusters      rkecontroller.RKEClusterController
	rkeControlPlanes rkecontroller.RKEControlPlaneCache
	capiClusters     capicontrollers.ClusterCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		rkeClusters:      clients.RKE.RKECluster(),
		rkeControlPlanes: clients.RKE.RKEControlPlane().Cache(),
		capiClusters:     clients.CAPI.Cluster().Cache(),
	}

	clients.RKE.RKECluster().OnChange(ctx, "rke-cluster", h.OnChange)
	relatedresource.Watch(ctx, "rke-cluster-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		return []relatedresource.Key{{
			Namespace: namespace,
			Name:      name,
		}}, nil
	}, clients.RKE.RKECluster(), clients.RKE.RKEControlPlane())
}

func (h *handler) OnChange(_ string, cluster *v1.RKECluster) (*v1.RKECluster, error) {
	if cluster == nil {
		return nil, nil
	}

	capiCluster, err := rke2.FindOwnerCAPICluster(cluster, h.capiClusters)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("[rkecluster] %s/%s: waiting: CAPI cluster does not exist", cluster.Namespace, cluster.Name)
			h.rkeClusters.EnqueueAfter(cluster.Namespace, cluster.Name, 10*time.Second)
			return cluster, generic.ErrSkip
		}
		logrus.Errorf("[rkecluster] %s/%s: error getting CAPI cluster %v", cluster.Namespace, cluster.Name, err)
		return cluster, err
	}

	if capiCluster == nil {
		logrus.Infof("[rkecluster] %s/%s: waiting: CAPI cluster does not exist", cluster.Namespace, cluster.Name)
		h.rkeClusters.EnqueueAfter(cluster.Namespace, cluster.Name, 10*time.Second)
		return cluster, generic.ErrSkip
	}

	if capiannotations.IsPaused(capiCluster, cluster) {
		logrus.Infof("[rkecluster] %s/%s: waiting: CAPI cluster or RKEControlPlane is paused", cluster.Namespace, cluster.Name)
		h.rkeClusters.EnqueueAfter(cluster.Namespace, cluster.Name, 10*time.Second)
		return cluster, generic.ErrSkip
	}

	if cluster.Spec.ControlPlaneEndpoint == nil || !cluster.Spec.ControlPlaneEndpoint.IsValid() {
		cluster = cluster.DeepCopy()
		cluster.Spec.ControlPlaneEndpoint = &capi.APIEndpoint{
			Host: "localhost",
			Port: 6443,
		}
		logrus.Infof("[rkecluster] %s/%s: setting Spec.ControlPlaneEndpoint", cluster.Namespace, cluster.Name)
		return h.rkeClusters.Update(cluster)
	}

	if cluster.Status.Ready != true {
		cluster = cluster.DeepCopy()
		cluster.Status.Ready = true
		logrus.Infof("[rkecluster] %s/%s: setting Status.Ready: %t", cluster.Namespace, cluster.Name, cluster.Status.Ready)
		return h.rkeClusters.UpdateStatus(cluster)
	}

	return cluster, nil
}
