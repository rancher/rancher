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
	"k8s.io/apimachinery/pkg/api/equality"
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
		} else if machine, ok := obj.(*capi.Machine); ok {
			clusterName := machine.Labels[capi.ClusterLabelName]
			if clusterName != "" {
				return []relatedresource.Key{{
					Namespace: machine.Namespace,
					Name:      clusterName,
				}}, nil
			}
		}
		return nil, nil
	}, clients.RKE.RKEControlPlane(), clients.Core.Secret(), clients.CAPI.Machine())
}

func (h *handler) OnChange(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	logrus.Debugf("[planner] rkecluster %s/%s: handler OnChange called", cp.Namespace, cp.Name)
	if !cp.DeletionTimestamp.IsZero() {
		return status, nil
	}

	status.ObservedGeneration = cp.Generation

	if rke2.Reconciled.IsTrue(&status) && !equality.Semantic.DeepEqual(cp.Spec, status.AppliedSpec) {
		rke2.Reconciled.SetStatus(&status, "Unknown")
		rke2.Reconciled.Message(&status, "reconciling control plane")
		rke2.Reconciled.Reason(&status, "Waiting")
	}

	logrus.Debugf("[planner] rkecluster %s/%s: calling planner process", cp.Namespace, cp.Name)
	status, err := h.planner.Process(cp, status)
	if err != nil {
		if planner.IsErrWaiting(err) {
			logrus.Infof("[planner] rkecluster %s/%s: waiting: %v", cp.Namespace, cp.Name, err)
			rke2.Reconciled.Message(&status, "reconciling control plane")
			// if still waiting for same condition, convert err to generic.ErrSkip to avoid updating controlplane status and
			// enqueue until no longer waiting.
			if rke2.Ready.GetMessage(&status) == err.Error() && rke2.Ready.GetStatus(&status) == "Unknown" && rke2.Ready.GetReason(&status) == "Waiting" {
				err = generic.ErrSkip
			} else {
				rke2.Ready.SetStatus(&status, "Unknown")
				rke2.Ready.Message(&status, err.Error())
				rke2.Ready.Reason(&status, "Waiting")
				err = nil
			}
		} else if !errors.Is(err, generic.ErrSkip) {
			logrus.Errorf("[planner] rkecluster %s/%s: error encountered during plan processing was %v", cp.Namespace, cp.Name, err)
			rke2.Ready.SetError(&status, "", err)
			rke2.Reconciled.SetError(&status, "", err)
		}
		if errors.Is(err, generic.ErrSkip) {
			h.controlPlanes.EnqueueAfter(cp.Namespace, cp.Name, 5*time.Second)
		}
	} else {
		logrus.Debugf("[planner] rkecluster %s/%s: reconciliation complete", cp.Namespace, cp.Name)
		rke2.Provisioned.True(&status)
		rke2.Provisioned.Message(&status, "")
		rke2.Provisioned.Reason(&status, "")
		rke2.Ready.True(&status)
		rke2.Ready.Message(&status, "")
		rke2.Ready.Reason(&status, "")
		status.AppliedSpec = &cp.Spec
		rke2.Reconciled.True(&status)
		rke2.Reconciled.Message(&status, "")
		rke2.Reconciled.Reason(&status, "")
	}
	return status, err
}
