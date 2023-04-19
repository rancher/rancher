package planner

import (
	"context"
	"errors"
	"strings"
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
			var relatedResources []relatedresource.Key
			clusterName := secret.Labels[rke2.ClusterNameLabel]
			if clusterName != "" {
				logrus.Tracef("[planner] rkecluster %s/%s enqueue triggered by secret %s/%s", secret.Namespace, clusterName, secret.Namespace, secret.Name)
				relatedResources = append(relatedResources, relatedresource.Key{
					Namespace: secret.Namespace,
					Name:      clusterName,
				})
			}
			authorizedObjects := secret.Annotations[rke2.AuthorizedObjectAnnotation]
			if authorizedObjects != "" {
				for _, clusterName = range strings.Split(authorizedObjects, ",") {
					logrus.Tracef("[planner] rkecluster %s/%s enqueue triggered by authorized secret %s/%s", secret.Namespace, clusterName, secret.Namespace, secret.Name)
					relatedResources = append(relatedResources, relatedresource.Key{
						Namespace: secret.Namespace,
						Name:      clusterName,
					})
				}
			}
			return relatedResources, nil
		} else if machine, ok := obj.(*capi.Machine); ok {
			clusterName := machine.Labels[capi.ClusterLabelName]
			if clusterName != "" {
				logrus.Tracef("[planner] rkecluster %s/%s enqueue triggered by machine %s/%s", machine.Namespace, clusterName, machine.Namespace, machine.Name)
				return []relatedresource.Key{{
					Namespace: machine.Namespace,
					Name:      clusterName,
				}}, nil
			}
		} else if configmap, ok := obj.(*corev1.ConfigMap); ok {
			var relatedResources []relatedresource.Key
			authorizedObjects := configmap.Annotations[rke2.AuthorizedObjectAnnotation]
			if authorizedObjects != "" {
				for _, clusterName := range strings.Split(authorizedObjects, ",") {
					logrus.Tracef("[planner] rkecluster %s/%s enqueue triggered by authorized configmap %s/%s", configmap.Namespace, clusterName, configmap.Namespace, configmap.Name)
					relatedResources = append(relatedResources, relatedresource.Key{
						Namespace: configmap.Namespace,
						Name:      clusterName,
					})
				}
			}
			return relatedResources, nil
		}
		return nil, nil
	}, clients.RKE.RKEControlPlane(), clients.Core.Secret(), clients.CAPI.Machine(), clients.Core.ConfigMap())
}

func (h *handler) OnChange(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	logrus.Debugf("[planner] rkecluster %s/%s: handler OnChange called", cp.Namespace, cp.Name)
	if !cp.DeletionTimestamp.IsZero() {
		return status, nil
	}

	status.ObservedGeneration = cp.Generation

	logrus.Debugf("[planner] rkecluster %s/%s: calling planner process", cp.Namespace, cp.Name)
	status, err := h.planner.Process(cp, status)
	if err != nil {
		// planner.Process can encounter 4 types of errors:
		// * planner.errWaiting - This is an error that indicates we are waiting for something. This will trigger an enqueue (due to error)
		// * generic.ErrSkip / planner.errIgnore - These are both errors that do not automatically re-enqueue the object is the same as a planner.errIgnore and should be treated as such
		// and we're waiting on that object to sync and/or be processed by some other controller, and when it is
		// processed the rkecontrolplane will be re-enqueued.
		// * error - All other errors. This should be an actual error during planner processing.
		if planner.IsErrWaiting(err) {
			logrus.Infof("[planner] rkecluster %s/%s: waiting: %v", cp.Namespace, cp.Name, err)
			// if still waiting for same condition, convert err to generic.ErrSkip to avoid updating controlplane status and
			// enqueue until no longer waiting.
			if rke2.Ready.GetMessage(&status) == err.Error() && rke2.Ready.GetStatus(&status) == "Unknown" && rke2.Ready.GetReason(&status) == "Waiting" {
				// This is effectively a bug, wherein "ErrWaiting" is returned with the same message for two different back to back changesets
				// This design needs to be revisited in the future to prevent unnecessary churn.
				if !equality.Semantic.DeepEqual(cp.Status, status) {
					err = nil
				} else {
					err = generic.ErrSkip
				}
			} else {
				rke2.Ready.SetStatus(&status, "Unknown")
				rke2.Ready.Message(&status, err.Error())
				rke2.Ready.Reason(&status, "Waiting")
				// Set err to nil so planner doesn't automatically re-enqueue the object, as we're waiting.
				err = nil
			}
		} else if errors.Is(err, generic.ErrSkip) || planner.IsErrIgnore(err) {
			logrus.Debugf("[planner] rkecluster %s/%s: ErrSkip | errIgnore : %v", cp.Namespace, cp.Name, err)
			h.controlPlanes.EnqueueAfter(cp.Namespace, cp.Name, 5*time.Second)
		} else {
			// An actual error occurred, so set the Ready and Reconciled condiitons to this error and return
			logrus.Errorf("[planner] rkecluster %s/%s: error encountered during plan processing was %v", cp.Namespace, cp.Name, err)
			rke2.Ready.SetError(&status, "", err)
			rke2.Reconciled.SetError(&status, "", err)
			return status, err
		}
		// If the Reconciled condition is already true and the error was NOT an errIgnore/ErrSkip/ErrWaiting and the status.AppliedSpec (from planner.Process) does not match the controlplane spec, set reconciled to unknown.
		if !equality.Semantic.DeepEqual(cp.Spec, status.AppliedSpec) {
			rke2.Reconciled.SetStatus(&status, "Unknown")
			rke2.Reconciled.Message(&status, "reconciling control plane")
			rke2.Reconciled.Reason(&status, "Waiting")
		}
	} else {
		// No error encountered during planner.Process
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
