package planner

import (
	"context"
	"errors"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/bootstrap"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
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
	v1.RegisterRKEControlPlaneStatusHandler(ctx,
		clients.RKE.RKEControlPlane(), "", "planner", h.OnChange)
	relatedresource.Watch(ctx, "planner", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if secret, ok := obj.(*corev1.Secret); ok {
			clusterName := secret.Labels[bootstrap.ClusterNameLabel]
			if clusterName != "" {
				return []relatedresource.Key{{
					Namespace: secret.Namespace,
					Name:      clusterName,
				}}, nil
			}
		} else if machine, ok := obj.(*capi.Machine); ok {
			return []relatedresource.Key{{
				Namespace: machine.Namespace,
				Name:      machine.Spec.ClusterName,
			}}, nil
		}
		return nil, nil
	}, clients.RKE.RKEControlPlane(), clients.Core.Secret(), clients.CAPI.Machine())
}

func (h *handler) OnChange(cluster *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if !cluster.DeletionTimestamp.IsZero() {
		return status, nil
	}

	status.ObservedGeneration = cluster.Generation

	err := h.planner.Process(cluster)
	var errWaiting planner.ErrWaiting
	if errors.As(err, &errWaiting) {
		logrus.Infof("rkecluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
		planner.Provisioned.SetStatus(&status, "Unknown")
		planner.Provisioned.Message(&status, err.Error())
		planner.Provisioned.Reason(&status, "Waiting")
		return status, nil
	}

	planner.Provisioned.SetError(&status, "", err)
	if err != nil && !errors.Is(err, generic.ErrSkip) {
		// don't return error because the controller will reset the status and then not assign the error
		// because we don't register this handler with an associated condition. This is pretty much a bug in the
		// framework but it's too impactful to change right before 2.6.0 so we should consider changing this later.
		// If you are reading this years later we'll just assume we decided not to change the framework.
		logrus.Errorf("error in planner for '%s/%s': %v", cluster.Namespace, cluster.Name, err)
		h.controlPlanes.EnqueueAfter(cluster.Namespace, cluster.Name, 5*time.Second)
	}
	return status, nil
}
