package encryptionkeyrotation

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

// ControllerOwnerKey is the value used to identify that the encryption-key-rotation handler currently owns the beacon.
const ControllerOwnerKey = "encryption-key-rotation"

type handler struct {
	encryptionkeyrotations operationcontrollers.EncryptionKeyRotationController

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
		encryptionkeyrotations: clients.Operation.EncryptionKeyRotation(),
		beacons:                clients.Plan.Beacon(),
		beaconCache:            clients.Plan.Beacon().Cache(),
		secrets:                clients.Core.Secret(),
		secretCache:            clients.Core.Secret().Cache(),
		dynamic:                clients.Dynamic,
		store:                  planapi.NewStore(clients.Core.Secret()),
		clients:                clients,
	}

	operationcontrollers.RegisterEncryptionKeyRotationStatusHandler(ctx, clients.Operation.EncryptionKeyRotation(), "", "encryption-key-rotation-handler", h.OnChange)
}

func (h *handler) OnChange(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	if reflect.DeepEqual(op.Status, status) {
		h.encryptionkeyrotations.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)
	}
	return status, nil
}

func (h *handler) onChange(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	if op == nil {
		return status, nil
	}

	if op.DeletionTimestamp != nil {
		return status, nil
	}

	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: skipping paused operation", op.Namespace, op.Name)
		return status, nil
	}

	if status.Phase == "" {
		status.Phase = opv1alpha1.OperationPhasePending
		status.LastUpdated = metav1.Now()
	}

	gvk := schema.FromAPIVersionAndKind(op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
	ref, err := h.dynamic.Get(gvk, op.Spec.ClusterRef.Namespace, op.Spec.ClusterRef.Name)
	if apierrors.IsNotFound(err) {
		key := fmt.Sprintf("apiVersion=%s, kind=%s", op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
		if op.Spec.ClusterRef.Namespace != "" {
			key += fmt.Sprintf(", namespace=%s", op.Spec.ClusterRef.Namespace)
		}
		key += fmt.Sprintf(", name=%s", op.Spec.ClusterRef.Name)
		logrus.Errorf("[encryptionkeyrotation]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.ClusterNotFoundReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("cluster %s not found", key))

		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()
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
			namespace = op.Namespace
		} else {
			// cluster-scoped objects: beacon namespace is the name of the object
			namespace = op.Spec.ClusterRef.Name
		}
	}

	beacon, err := h.beacons.Get(namespace, ustr.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		key := fmt.Sprintf("apiVersion=%s, kind=%s", ustr.GetAPIVersion(), ustr.GetKind())
		if ustr.GetNamespace() != "" {
			key += fmt.Sprintf(", namespace=%s", ustr.GetNamespace())
		}
		key += fmt.Sprintf(", name=%s", ustr.GetName())
		logrus.Warnf("[encryptionkeyrotation]: %s/%s failed to find beacon for %s", op.Namespace, op.Name, key)

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

	if status.Phase == opv1alpha1.OperationPhasePending {
		return h.handlePending(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseInProgress {
		return h.handleInProgress(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseCanceled {
		return status, nil
	}
	if status.Phase == opv1alpha1.OperationPhaseFailed {
		return h.handleFailed(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseSucceeded {
		return h.handleSucceeded(s, status)
	}

	// handle after normal processing to allow for proper phase-related cleanup (freeing beacon)
	if ops.IsTerminal(status.Phase) && ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) {
		err = h.encryptionkeyrotations.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
		if err != nil {
			return status, err
		}
		return status, generic.ErrSkip
	}

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))

	return status, nil
}

type scope struct {
	op        *opv1alpha1.EncryptionKeyRotation
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

func (h *handler) handlePending(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	beacon, err := planapi.AcquireBeacon(s.beacon, h.beacons, ControllerOwnerKey)
	if err != nil {
		return status, err
	}
	if beacon == nil {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon acquisition")
		return status, nil
	}
	logrus.Infof("[encryptionkeyrotation] %s/%s: acquired beacon, waiting for agents to register", s.op.Namespace, s.op.Name)

	if ok, err := s.adapter.WaitForRegister(); err != nil {
		return status, err
	} else if !ok {
		logrus.Infof("[encryptionkeyrotation] %s/%s: waiting for system-agents to connect", s.op.Namespace, s.op.Name)
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForRegistrationReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for system-agents to connect")
		return status, nil
	}

	logrus.Infof("[encryptionkeyrotation] %s/%s: transitioning to rotate", s.op.Namespace, s.op.Name)

	status.Phase = opv1alpha1.OperationPhaseInProgress
	status.LastUpdated = metav1.Now()
	status.Step = opv1alpha1.EncryptionKeyRotationStepRotate

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

func (h *handler) handleInProgress(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	if !planapi.HoldingBeacon(s.beacon, ControllerOwnerKey) {
		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "beacon acquired by another controller, aborting")

		return status, nil
	}

	var err error
	s.beacon, err = planapi.ToggleBeacon(s.beacon, true, h.beacons)
	if err != nil {
		return status, err
	}

	switch s.op.Status.Step {
	case opv1alpha1.EncryptionKeyRotationStepRotate:
		return h.reconcileRotate(s, status)
	case opv1alpha1.EncryptionKeyRotationStepRestart:
		return h.reconcileRestart(s, status)
	}

	status.Phase = opv1alpha1.OperationPhaseFailed
	status.LastUpdated = metav1.Now()

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownStepReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("current step [%q] is unknown, expected one of: [%q, %q]",
		status.Step, opv1alpha1.EncryptionKeyRotationStepRotate, opv1alpha1.EncryptionKeyRotationStepRestart))

	return status, nil
}

func (h *handler) reconcileRotate(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling secrets-encrypt rotate-keys", s.op.Namespace, s.op.Name)

	secrets, err := planapi.NewLabeler().
		And(
			planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.ControlPlaneRoleLabel, "true")).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret)
		if err != nil {
			return status, err
		}

		nodePlan := &planapi.Plan{
			OneTimeInstructions: []planapi.OneTimeInstruction{
				{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "rotate-keys",
						Command: s.adapter.RuntimeCommand(),
						Args:    []string{"secrets-encrypt", "rotate-keys"},
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
			logrus.Errorf("[encryptionkeyrotation] %s/%s: marking operation as failed: failed to apply plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.Phase = opv1alpha1.OperationPhaseFailed
			status.LastUpdated = metav1.Now()

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("secrets-encrypt rotate-keys failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[encryptionkeyrotation] %s/%s: waiting for secrets-encrypt rotate-keys: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
	}

	logrus.Infof("[encryptionkeyrotation] %s/%s: transitioning to restart", s.op.Namespace, s.op.Name)

	status.Step = opv1alpha1.EncryptionKeyRotationStepRestart
	return status, nil
}

func (h *handler) reconcileRestart(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling service restart", s.op.Namespace, s.op.Name)

	secrets, err := planapi.NewLabeler().
		And(
			planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Label(capr.ControlPlaneRoleLabel, "true")).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret)
		if err != nil {
			return status, err
		}

		nodePlan := &planapi.Plan{
			OneTimeInstructions: []planapi.OneTimeInstruction{
				{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "restart",
						Command: "systemctl",
						Args:    []string{"restart", s.adapter.ServerUnit()},
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
			logrus.Errorf("[encryptionkeyrotation] %s/%s: marking operation as failed: failed to apply plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.Phase = opv1alpha1.OperationPhaseFailed
			status.LastUpdated = metav1.Now()

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("restart failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[encryptionkeyrotation] %s/%s: waiting for service restart: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
	}

	logrus.Infof("[encryptionkeyrotation] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.Phase = opv1alpha1.OperationPhaseSucceeded
	status.LastUpdated = metav1.Now()

	opv1alpha1.SucceededCondition.True(&status)
	opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.FinishedReason)
	opv1alpha1.SucceededCondition.Message(&status, "Operation completed successfully")

	return status, nil
}

func (h *handler) handleFailed(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	err := planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
	if err != nil {
		return status, err
	}
	return status, nil
}

func (h *handler) handleSucceeded(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

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
func updateStatus(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) opv1alpha1.EncryptionKeyRotationStatus {
	logrus.Tracef("[encryptionkeyrotation] %s/%s: updating conditions", op.Namespace, op.Name)

	status.ObservedGeneration = op.Generation

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
		opv1alpha1.PendingCondition.Message(&status, "Operation completed successfully")
		opv1alpha1.InProgressCondition.False(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.FinishedReason)
		opv1alpha1.InProgressCondition.Message(&status, "Operation failed")
		opv1alpha1.SucceededCondition.False(&status)
		opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.NotSuccessfulReason)
		opv1alpha1.SucceededCondition.Message(&status, "Operation failed")
	}

	return status
}
