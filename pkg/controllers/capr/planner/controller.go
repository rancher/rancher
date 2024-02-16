package planner

import (
	"context"
	"errors"
	"strings"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	caprplanner "github.com/rancher/rancher/pkg/capr/planner"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/generic"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type handler struct {
	planner       *caprplanner.Planner
	controlPlanes v1.RKEControlPlaneController
}

func Register(ctx context.Context, clients *wrangler.Context, planner *caprplanner.Planner) {
	h := handler{
		planner:       planner,
		controlPlanes: clients.RKE.RKEControlPlane(),
	}
	v1.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(), "", "planner", h.OnChange)
	relatedresource.Watch(ctx, "planner", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if secret, ok := obj.(*corev1.Secret); ok {
			var relatedResources []relatedresource.Key
			clusterName := secret.Labels[capr.ClusterNameLabel]
			if clusterName != "" {
				logrus.Tracef("[planner] rkecluster %s/%s enqueue triggered by secret %s/%s", secret.Namespace, clusterName, secret.Namespace, secret.Name)
				relatedResources = append(relatedResources, relatedresource.Key{
					Namespace: secret.Namespace,
					Name:      clusterName,
				})
			}
			authorizedObjects := secret.Annotations[capr.AuthorizedObjectAnnotation]
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
			clusterName := machine.Labels[capi.ClusterNameLabel]
			if clusterName != "" {
				logrus.Tracef("[planner] rkecluster %s/%s enqueue triggered by machine %s/%s", machine.Namespace, clusterName, machine.Namespace, machine.Name)
				return []relatedresource.Key{{
					Namespace: machine.Namespace,
					Name:      clusterName,
				}}, nil
			}
		} else if configmap, ok := obj.(*corev1.ConfigMap); ok {
			var relatedResources []relatedresource.Key
			authorizedObjects := configmap.Annotations[capr.AuthorizedObjectAnnotation]
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
		// planner.Process can encounter 3 types of errors:
		// * planner.errWaiting - This is an error that indicates we are waiting for something, and will not re-enqueue the object
		// * generic.ErrSkip - These will cause the object to be re-enqueued after 5 seconds.
		// * error - All other errors. This should be an actual error during planner processing.
		if caprplanner.IsErrWaiting(err) {
			logrus.Infof("[planner] rkecluster %s/%s: %v", cp.Namespace, cp.Name, err)
			capr.Ready.SetStatus(&status, "Unknown")
			capr.Ready.Message(&status, err.Error())
			capr.Ready.Reason(&status, "Waiting")
			// Set err to nil so planner doesn't automatically re-enqueue the object, as we're waiting.
			// If the Reconciled condition is already true and the error was NOT an errIgnore/ErrSkip/ErrWaiting and the status.AppliedSpec (from planner.Process) does not match the controlplane spec, set reconciled to unknown.
			if !equality.Semantic.DeepEqual(cp.Spec, status.AppliedSpec) {
				capr.Reconciled.SetStatus(&status, "Unknown")
				capr.Reconciled.Message(&status, "RKEControlPlane has not been fully reconciled yet")
				capr.Reconciled.Reason(&status, "Waiting")
			}
			return status, nil
		} else if errors.Is(err, generic.ErrSkip) {
			logrus.Debugf("[planner] rkecluster %s/%s: ErrSkip: %v", cp.Namespace, cp.Name, err)
			h.controlPlanes.EnqueueAfter(cp.Namespace, cp.Name, 5*time.Second)
			return status, err
		} else {
			// An actual error occurred, so set the Ready and Reconciled conditions to this error and return
			logrus.Errorf("[planner] rkecluster %s/%s: error during plan processing: %v", cp.Namespace, cp.Name, err)
			capr.Ready.SetError(&status, "", err)
			capr.Reconciled.SetError(&status, "", err)
			return status, err
		}
	}
	// No error encountered during planner.Process
	logrus.Debugf("[planner] rkecluster %s/%s: reconciliation complete", cp.Namespace, cp.Name)
	capr.Ready.True(&status)
	capr.Ready.Message(&status, "")
	capr.Ready.Reason(&status, "")
	capr.Stable.True(&status)
	capr.Stable.Message(&status, "")
	capr.Stable.Reason(&status, "")
	status.AppliedSpec = &cp.Spec
	capr.Reconciled.True(&status)
	capr.Reconciled.Message(&status, "")
	capr.Reconciled.Reason(&status, "")
	return status, nil
}
