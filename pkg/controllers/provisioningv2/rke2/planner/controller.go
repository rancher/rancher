package planner

import (
	"context"
	"errors"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/bootstrap"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	Provisioned = condition.Cond("Provisioned")
)

type handler struct {
	planner *planner.Planner
}

func Register(ctx context.Context, clients *wrangler.Context, planner *planner.Planner) {
	h := handler{
		planner: planner,
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
	status.ObservedGeneration = cluster.Generation

	err := h.planner.Process(cluster)
	var errWaiting planner.ErrWaiting
	if errors.As(err, &errWaiting) {
		logrus.Infof("rkecluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
		Provisioned.SetStatus(&status, "Unknown")
		Provisioned.Message(&status, err.Error())
		Provisioned.Reason(&status, "Waiting")
		return status, nil
	}

	Provisioned.SetError(&status, "", err)
	return status, err
}
