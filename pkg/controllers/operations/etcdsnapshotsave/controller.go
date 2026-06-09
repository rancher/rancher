package etcdsnapshotsave

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ControllerOwnerKey is the value used to identify the etcd-snapshot-save handler currently owns the beacon.
const ControllerOwnerKey = "etcd-snapshot-save"

type handler struct {
	etcdsnapshotsaves operationcontrollers.ETCDSnapshotSaveController

	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache

	secrets     corecontrollers.SecretClient
	secretCache corecontrollers.SecretCache

	store *planapi.Store

	dynamic *dynamic.Controller

	clients *wrangler.CAPIContext
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		etcdsnapshotsaves: clients.Operation.ETCDSnapshotSave(),
		beacons:           clients.Plan.Beacon(),
		beaconCache:       clients.Plan.Beacon().Cache(),
		secrets:           clients.Core.Secret(),
		secretCache:       clients.Core.Secret().Cache(),
		dynamic:           clients.Dynamic,
		store:             planapi.NewStore(clients.Core.Secret()),
		clients:           clients,
	}

	operationcontrollers.RegisterETCDSnapshotSaveStatusHandler(ctx, clients.Operation.ETCDSnapshotSave(), "", "etcd-snapshot-create-handler", h.OnChange)
}

func (h *handler) OnChange(op *opv1alpha1.ETCDSnapshotSave, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	if reflect.DeepEqual(op.Status, status) {
		// handle after normal processing to allow for proper phase-related cleanup (freeing beacon)
		if ops.IsTerminal(status.Phase) && ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) {
			err = h.etcdsnapshotsaves.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
			if err != nil {
				return status, err
			}
			return status, generic.ErrSkip
		}

		h.etcdsnapshotsaves.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)
	}
	return status, nil
}

func (h *handler) onChange(op *opv1alpha1.ETCDSnapshotSave, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	if op == nil {
		return status, nil
	}

	if op.DeletionTimestamp != nil {
		return status, nil
	}

	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[etcdsnapshotsave] %s/%s: skipping paused operation", op.Namespace, op.Name)
		return status, nil
	}

	if status.Phase == "" {
		status.SetPhase(opv1alpha1.OperationPhasePending)
	}

	gvk := schema.FromAPIVersionAndKind(op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
	ref, err := h.dynamic.Get(gvk, op.Spec.ClusterRef.Namespace, op.Spec.ClusterRef.Name)
	if apierrors.IsNotFound(err) {
		key := fmt.Sprintf("apiVersion=%s, kind=%s", op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
		if op.Spec.ClusterRef.Namespace != "" {
			key += fmt.Sprintf(", namespace=%s", op.Spec.ClusterRef.Namespace)
		}
		key += fmt.Sprintf(", name=%s", op.Spec.ClusterRef.Name)
		logrus.Errorf("[etcdsnapshotsave]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.ClusterNotFoundReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("cluster %s not found", key))

		status.SetPhase(opv1alpha1.OperationPhaseFailed)
		return status, nil
	}
	if err != nil {
		return status, err
	}

	ustrMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ref)
	if err != nil {
		return status, err
	}

	ustr := unstructured.Unstructured{Object: ustrMap}

	namespace := op.Spec.ClusterRef.Namespace
	if namespace == "" {
		mapping, err := h.clients.RESTMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return status, err
		}
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// For namespace-scoped objects, assume namespace is the same as the snapshot object
			namespace = op.Namespace
		} else {
			// For cluster-scoped objects, assume namespace is the name of the object
			namespace = op.Spec.ClusterRef.Name
		}
	}

	beacon, err := h.beacons.Get(namespace, ustr.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		// If the beacon is not found during pending phase, allow time for the beacon to be created.
		key := fmt.Sprintf("apiVersion=%s, kind=%s", ustr.GetAPIVersion(), ustr.GetKind())
		if ustr.GetNamespace() != "" {
			key += fmt.Sprintf(", namespace=%s", ustr.GetNamespace())
		}
		key += fmt.Sprintf(", name=%s", ustr.GetName())
		logrus.Warnf("[etcdsnapshotsave]: %s/%s failed to find beacon for %s", op.Namespace, op.Name, key)

		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")

		return status, nil
	} else if err != nil {
		return status, err
	}

	a, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return status, err
	}

	s := &scope{
		op:         op,
		beacon:     beacon,
		namespace:  namespace,
		clusterObj: &ustr,
		adapter:    a,
	}

	switch status.Phase {
	case opv1alpha1.OperationPhasePending:
		status, err = h.handlePending(s, status)
	case opv1alpha1.OperationPhaseInProgress:
		status, err = h.handleInProgress(s, status)
	case opv1alpha1.OperationPhaseCanceled:
		// canceled assumes that the beacon is released, so we can safely skip the rest of the processing
		return status, nil
	case opv1alpha1.OperationPhaseFailed:
		status, err = h.handleFailed(s, status)
	case opv1alpha1.OperationPhaseSucceeded:
		status, err = h.handleSucceeded(s, status)
	default:
		// Should be prevented via validation, but just in case
		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))
	}
	if err != nil {
		return status, err
	}

	return status, nil
}

type scope struct {
	op        *opv1alpha1.ETCDSnapshotSave
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

func (h *handler) handlePending(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling pending", s.op.Namespace, s.op.Name)

	beacon, err := planapi.AcquireBeacon(s.beacon, h.beacons, ControllerOwnerKey)
	if err != nil {
		return status, err
	}
	if beacon == nil {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")
		return status, nil
	}

	logrus.Debugf("[etcdsnapshotsave] %s/%s: acquired beacon, waiting for agents to register", s.op.Namespace, s.op.Name)

	if ok, err := s.adapter.WaitForRegister(); err != nil {
		return status, err
	} else if !ok {
		logrus.Infof("[etcdsnapshotsave] %s/%s: waiting for system-agents to connect", s.op.Namespace, s.op.Name)
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForRegistrationReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for system-agents to connect")
		return status, nil
	}

	logrus.Infof("[etcdsnapshotsave] %s/%s: transitioning to save", s.op.Namespace, s.op.Name)

	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.ETCDSnapshotSaveStepSave)

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

func (h *handler) handleInProgress(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling in-progress", s.op.Namespace, s.op.Name)

	if !planapi.HoldingBeacon(s.beacon, ControllerOwnerKey) {
		logrus.Errorf("[etcdsnapshotsave] %s/%s: beacon lost, aborting", s.op.Namespace, s.op.Name)
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "Beacon acquired by another controller, aborting")

		return status, nil
	}

	var err error
	s.beacon, err = planapi.ToggleBeacon(s.beacon, true, h.beacons)
	if err != nil {
		return status, err
	}

	switch s.op.Status.Step {
	case opv1alpha1.ETCDSnapshotSaveStepSave:
		status, err = h.reconcileSave(s, status)
	case opv1alpha1.ETCDSnapshotSaveStepRestart:
		status, err = h.reconcileRestart(s, status)
	default:
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownStepReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf(
			"current step [\"%s\"] is unknown, expected one of: [\"%s\", \"%s\"]",
			status.Step,
			opv1alpha1.ETCDSnapshotSaveStepSave,
			opv1alpha1.ETCDSnapshotSaveStepRestart))

	}

	return status, nil
}

func (h *handler) reconcileSave(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Debugf("[etcdsnapshotsave] %s/%s: handling snapshot save", s.op.Namespace, s.op.Name)

	// collect etcd nodes belonging to cluster
	secrets, err := planapi.NewLabeler().
		And(
			planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.EtcdRoleLabel, "true")).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}
	if len(secrets) == 0 {
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "failed to find etcd node to perform etcd snapshot")

		return status, nil
	}

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		saveInstruction := planapi.OneTimeInstruction{
			CommonInstruction: planapi.CommonInstruction{
				Name:    "snapshot",
				Command: s.adapter.RuntimeCommand(),
				Args: []string{
					"etcd-snapshot",
					"save",
				},
			},
		}

		if s.op.Spec.Args.Name != "" {
			saveInstruction.CommonInstruction.Args = append(saveInstruction.CommonInstruction.Args, "--name", s.op.Spec.Args.Name)
		}
		if s.op.Spec.Args.ETCDSnapshotCompress {
			saveInstruction.CommonInstruction.Args = append(saveInstruction.CommonInstruction.Args, "--compress")
		}
		if s.op.Spec.Args.ETCDSnapshotDir != "" {
			saveInstruction.CommonInstruction.Args = append(saveInstruction.CommonInstruction.Args, "--dir", s.op.Spec.Args.ETCDSnapshotDir)
		}

		nodePlan := &planapi.Plan{
			OneTimeInstructions: []planapi.OneTimeInstruction{
				saveInstruction,
			},
			Probes: probes,
		}

		planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotsave] %s/%s: marking operation as failed: failed to apply plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("etcd snapshot save failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[etcdsnapshotsave] %s/%s: waiting for snapshot save: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
	}

	logrus.Infof("[etcdsnapshotsave] %s/%s: transitioning to restart", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotSaveStepRestart)
	return status, nil
}

func (h *handler) reconcileRestart(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Debugf("[etcdsnapshotsave] %s/%s: handling service restart", s.op.Namespace, s.op.Name)

	// collect etcd nodes belonging to cluster
	secrets, err := planapi.NewLabeler().
		And(
			planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.EtcdRoleLabel, "true")).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		nodePlan := &planapi.Plan{
			OneTimeInstructions: []planapi.OneTimeInstruction{
				{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "restart",
						Command: "systemctl",
						Args: []string{
							"restart",
							s.adapter.ServerUnit(),
						},
					},
				},
			},
			Probes: probes,
		}

		planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotsave] %s/%s: marking operation as failed: failed to apply plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("restart failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[etcdsnapshotsave] %s/%s: waiting for systemctl restart: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
	}

	logrus.Infof("[etcdsnapshotsave] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.SetPhase(opv1alpha1.OperationPhaseSucceeded)

	opv1alpha1.SucceededCondition.True(&status)
	opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.FinishedReason)
	opv1alpha1.SucceededCondition.Message(&status, "Operation completed successfully")

	return status, nil
}

func (h *handler) handleFailed(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	if planapi.HoldingBeacon(s.beacon, ControllerOwnerKey) {
		var err error
		s.beacon, err = planapi.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		err = planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
		if err != nil {
			return status, err
		}
	}

	return status, nil
}

func (h *handler) handleSucceeded(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

	if planapi.HoldingBeacon(s.beacon, ControllerOwnerKey) {
		var err error
		s.beacon, err = planapi.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		err = planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
		if err != nil {
			return status, err
		}

		// enqueue original object to ensure it is processed by requisite controllers
		gvk := schema.FromAPIVersionAndKind(s.clusterObj.GetAPIVersion(), s.clusterObj.GetKind())
		_ = h.dynamic.Enqueue(gvk, s.clusterObj.GetNamespace(), s.clusterObj.GetName())
	}
	return status, nil
}

// updateStatus updates the conditions of the operation based on the current status.
// This function also updates the ObservedGeneration.
// The handler is responsible for updating the condition relevant to the current phase, but this function updates the
// remaining conditions.
func updateStatus(op *opv1alpha1.ETCDSnapshotSave, status opv1alpha1.ETCDSnapshotSaveStatus) opv1alpha1.ETCDSnapshotSaveStatus {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: updating conditions", op.Namespace, op.Name)

	status.ObservedGeneration = op.Generation
	if op.Spec.Paused {
		opv1alpha1.PausedCondition.True(&status)
		opv1alpha1.PausedCondition.Reason(&status, opv1alpha1.PausedReason)
		opv1alpha1.PausedCondition.Message(&status, "Operation is paused")
	} else {
		opv1alpha1.PausedCondition.False(&status)
		opv1alpha1.PausedCondition.Reason(&status, opv1alpha1.NotPausedReason)
		opv1alpha1.PausedCondition.Message(&status, "")
	}

	if status.Phase == opv1alpha1.OperationPhasePending {
		opv1alpha1.PendingCondition.True(&status)
	} else if status.Phase == opv1alpha1.OperationPhaseInProgress {
		opv1alpha1.PendingCondition.False(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.InProgressReason)
		opv1alpha1.PendingCondition.Message(&status, "Operation now in progress")
	} else if status.Phase == opv1alpha1.OperationPhaseSucceeded {
		opv1alpha1.PendingCondition.False(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.PendingCondition.Message(&status, "Operation completed successfully")
		opv1alpha1.InProgressCondition.False(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.InProgressCondition.Message(&status, "Operation completed successfully")
		opv1alpha1.FailedCondition.False(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.NotFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "Operation completed successfully")
	} else if status.Phase == opv1alpha1.OperationPhaseFailed {
		opv1alpha1.PendingCondition.False(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.PendingCondition.Message(&status, "Operation failed")
		opv1alpha1.InProgressCondition.False(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.InProgressCondition.Message(&status, "Operation failed")
		opv1alpha1.SucceededCondition.False(&status)
		opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.NotSuccessfulReason)
		opv1alpha1.SucceededCondition.Message(&status, "Operation failed")
	}

	return status
}
