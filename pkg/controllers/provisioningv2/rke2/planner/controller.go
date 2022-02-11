package planner

import (
	"context"
	"errors"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type handler struct {
	planner       *planner.Planner
	controlPlanes v1.RKEControlPlaneController
}

func Register(ctx context.Context, clients *wrangler.Context, planner *planner.Planner) {
	h := handler{
		planner:       planner,
		controlPlanes: clients.RKE.RKEControlPlane(),
	}
	v1.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(), "", "planner", h.OnChange)
	relatedresource.Watch(ctx, "planner", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if secret, ok := obj.(*corev1.Secret); ok {
			clusterName := secret.Labels[rke2.ClusterNameLabel]
			if clusterName != "" {
				return []relatedresource.Key{{
					Namespace: secret.Namespace,
					Name:      clusterName,
				}}, nil
			}
		}
		return nil, nil
	}, clients.RKE.RKEControlPlane(), clients.Core.Secret())
}

func (h *handler) OnChange(cluster *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if !cluster.DeletionTimestamp.IsZero() {
		return status, nil
	}

	status.ObservedGeneration = cluster.Generation

	err := h.planner.Process(cluster)
	var errWaiting planner.ErrWaiting
	if errors.As(err, &errWaiting) {
		logrus.Infof("[planner] rkecluster %s/%s: waiting: %v", cluster.Namespace, cluster.Name, err)
		rke2.Ready.SetStatus(&status, "Unknown")
		rke2.Ready.Message(&status, err.Error())
		rke2.Ready.Reason(&status, "Waiting")
		return status, nil
	}
	if !errors.Is(err, generic.ErrSkip) {
		rke2.Ready.SetError(&status, "", err)
		if err != nil {
			// don't return error because the controller will reset the status and then not assign the error
			// because we don't register this handler with an associated condition. This is pretty much a bug in the
			// framework but it's too impactful to change right before 2.6.0 so we should consider changing this later.
			// If you are reading this years later we'll just assume we decided not to change the framework.
			logrus.Errorf("[planner] rkecluster %s/%s: error encountered during plan processing was %v", cluster.Namespace, cluster.Name, err)
			h.controlPlanes.EnqueueAfter(cluster.Namespace, cluster.Name, 5*time.Second)
		}
	} else {
		logrus.Debugf("[planner] rkecluster %s/%s: objects changed, waiting for cache sync before finishing reconciliation", cluster.Namespace, cluster.Name)
	}
	logrus.Debugf("[planner] rkecluster %s/%s: reconciliation complete", cluster.Namespace, cluster.Name)
	return status, nil
}
