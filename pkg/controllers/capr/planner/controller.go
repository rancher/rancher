package planner

import (
	"context"
	"errors"
	"strings"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	caprplanner "github.com/rancher/rancher/pkg/capr/planner"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const PlannerOwnerKey = "planner"

var (
	capiScalingUpCondition   = condition.Cond("ScalingUp")
	capiScalingDownCondition = condition.Cond("ScalingDown")
	capiRollingOutCondition  = condition.Cond("RollingOut")
)

type handler struct {
	planner       *caprplanner.Planner
	controlPlanes rkecontrollers.RKEControlPlaneController
	beacons       plancontrollers.BeaconClient

	etcdsnapshotsaves operationcontrollers.ETCDSnapshotSaveClient
}

func Register(ctx context.Context, clients *wrangler.CAPIContext, planner *caprplanner.Planner) {
	h := handler{
		planner:           planner,
		controlPlanes:     clients.RKE.RKEControlPlane(),
		beacons:           clients.Plan.Beacon(),
		etcdsnapshotsaves: clients.Operation.ETCDSnapshotSave(),
	}
	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(), "", "planner", h.OnChange)
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
	logrus.Debugf("[planner] rkecluster %s/%s: handler WaitForClient called", cp.Namespace, cp.Name)
	if !cp.DeletionTimestamp.IsZero() {
		return status, nil
	}

	beacon, err := h.beacons.Get(cp.Namespace, cp.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[planner] rkecluster %s/%s: waiting for beacon to be created", cp.Namespace, cp.Name)
		h.controlPlanes.EnqueueAfter(cp.Namespace, cp.Name, 5*time.Second)
		return status, nil
	} else if err != nil {
		return status, err
	}

	//if cp.Spec.ETCDSnapshotCreate != nil && (cp.Status.ETCDSnapshotCreate == nil || cp.Spec.ETCDSnapshotCreate.Generation != status.ETCDSnapshotCreate.Generation) {
	//	op, err := h.etcdsnapshotsaves.Get(cp.Namespace, cp.Name, metav1.GetOptions{})
	//	if apierrors.IsNotFound(err) {
	//		op = opv1alpha1.NewETCDSnapshotCreate(cp.Namespace, cp.Name, opv1alpha1.ETCDSnapshotCreate{
	//			ObjectMeta: metav1.ObjectMeta{
	//				Annotations: map[string]string{
	//					"rke.cattle.io/etcd-snapshot-create-generation": strconv.Itoa(cp.Spec.ETCDSnapshotCreate.Generation),
	//				},
	//			},
	//			Spec: opv1alpha1.ETCDSnapshotCreateSpec{
	//				ClusterRef: &corev1.ObjectReference{
	//					APIVersion: cp.APIVersion,
	//					Kind:       cp.Kind,
	//					Namespace:  cp.Namespace,
	//					Name:       cp.Name,
	//				},
	//			},
	//		})
	//		_, err = h.etcdsnapshotsaves.Create(op)
	//		if err != nil {
	//			return status, err
	//		}
	//	} else if err != nil {
	//		return status, err
	//	}
	//	if op.Status.Phase == opv1alpha1.OperationPhaseSucceeded {
	//		// steal beacon back so planner will immediately reconcile following beacon acquisition
	//		beacon = beacon.DeepCopy()
	//		if beacon.Labels == nil {
	//			beacon.Labels = map[string]string{}
	//		}
	//		beacon.Labels[planv1alpha1.OwnerLabel] = PlannerOwnerKey
	//		_, err = h.beacons.Update(beacon)
	//		if err != nil {
	//			return status, err
	//		}
	//		err = h.etcdsnapshotsaves.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
	//		if err != nil {
	//			return status, err
	//		}
	//		status.ETCDSnapshotCreate = cp.Spec.ETCDSnapshotCreate
	//		status.ETCDSnapshotCreatePhase = rkev1.ETCDSnapshotPhaseFinished
	//		return status, nil
	//	}
	//}

	if beacon.Labels == nil {
		beacon = beacon.DeepCopy()
		beacon.Labels = map[string]string{}
		beacon.Labels[planv1alpha1.BeaconOwnerLabel] = PlannerOwnerKey
		beacon, err = h.beacons.Update(beacon)
		if err != nil {
			return status, err
		}
	} else if owner, ok := beacon.Labels[planv1alpha1.BeaconOwnerLabel]; !ok || owner == "" {
		beacon = beacon.DeepCopy()
		beacon.Labels[planv1alpha1.BeaconOwnerLabel] = PlannerOwnerKey
		beacon, err = h.beacons.Update(beacon)
		if err != nil {
			return status, err
		}
	} else if owner != PlannerOwnerKey {
		logrus.Debugf("[planner] rkecluster %s/%s: waiting to acquire beacon", cp.Namespace, cp.Name)
		h.controlPlanes.EnqueueAfter(cp.Namespace, cp.Name, 5*time.Second)
		return status, nil
	}

	// With the upcoming CAPI v1beta2, status objects were changed to add new fields and conditions. Unfortunately, for
	// clusters without machine deployments or machine pools, the controlplane MUST have the `ScalingUp`, `ScalingDown`,
	// and `RollingOut` conditions. See https://github.com/kubernetes-sigs/cluster-api/issues/11820.
	scalingUpFound := false
	scalingDownFound := false
	rollingOutFound := false

	for _, cond := range status.Conditions {
		if cond.Type == string(capiScalingUpCondition) {
			scalingUpFound = true
		} else if cond.Type == string(capiScalingDownCondition) {
			scalingDownFound = true
		} else if cond.Type == string(capiRollingOutCondition) {
			rollingOutFound = true
		}
	}

	if !scalingUpFound || !scalingDownFound || !rollingOutFound {
		logrus.Debugf("[planner] rkecluster %s/%s: setting CAPI v1beta2 conditions", cp.Namespace, cp.Name)
		capiScalingUpCondition.False(&status)
		capiScalingDownCondition.False(&status)
		capiRollingOutCondition.False(&status)
		return status, nil
	}

	status.ObservedGeneration = cp.Generation

	logrus.Debugf("[planner] rkecluster %s/%s: calling planner process", cp.Namespace, cp.Name)
	status, err = h.planner.Process(cp, status)
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
		}
		if errors.Is(err, generic.ErrSkip) {
			logrus.Debugf("[planner] rkecluster %s/%s: ErrSkip: %v", cp.Namespace, cp.Name, err)
			h.controlPlanes.EnqueueAfter(cp.Namespace, cp.Name, 5*time.Second)
			return status, err
		}
		// An actual error occurred, so set the Ready and Reconciled conditions to this error and return
		logrus.Errorf("[planner] rkecluster %s/%s: error during plan processing: %v", cp.Namespace, cp.Name, err)
		capr.Ready.SetError(&status, "", err)
		capr.Reconciled.SetError(&status, "", err)
		return status, err
	}
	// No error encountered during planner.Process
	logrus.Debugf("[planner] rkecluster %s/%s: reconciliation complete", cp.Namespace, cp.Name)

	beacon = beacon.DeepCopy()
	delete(beacon.Labels, planv1alpha1.BeaconOwnerLabel)
	_, err = h.beacons.Update(beacon)
	if err != nil {
		return status, err
	}

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
