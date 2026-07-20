package certificaterotation

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// ControllerOwnerKey identifies certificate rotation beacon ownership.
	ControllerOwnerKey = "certificate-rotation"

	// RotateStepHookLabelPrefix gates the Rotate step so delegates can short-circuit the
	// step work while still observing pre-pause state.
	RotateStepHookLabelPrefix = "rotate.step.hook.operation.cattle.io/"
)

// dynamicResolver is the subset of the dynamic controller used by operations handlers.
type dynamicResolver interface {
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
	Enqueue(gvk schema.GroupVersionKind, namespace, name string) error
}

type handler struct {
	certificateRotations operationcontrollers.CertificateRotationController

	beacons plancontrollers.BeaconClient

	secrets corecontrollers.SecretClient

	store *plan.Store

	dynamic dynamicResolver

	clients *wrangler.CAPIContext
}

type scope struct {
	ownerKey string

	op        *opv1alpha1.CertificateRotation
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		certificateRotations: clients.Operation.CertificateRotation(),
		beacons:              clients.Plan.Beacon(),
		secrets:              clients.Core.Secret(),
		store:                plan.NewStore(clients.Core.Secret()),
		dynamic:              clients.Dynamic,
		clients:              clients,
	}

	operationcontrollers.RegisterCertificateRotationStatusHandler(ctx, clients.Operation.CertificateRotation(), "", "certificate-rotation-handler", h.OnChange)
}

func (h *handler) OnChange(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	if equality.Semantic.DeepEqual(op.Status, status) {
		if ops.IsTerminal(status.Phase) &&
			ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) &&
			!planv1alpha1.HasActiveLifecycleHook(op) {
			err = h.certificateRotations.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
			if err != nil {
				return status, err
			}
			return status, generic.ErrSkip
		}

		h.certificateRotations.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)
	}

	return status, nil
}

func (h *handler) onChange(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	if op == nil {
		return status, nil
	}

	if op.DeletionTimestamp != nil {
		return status, nil
	}

	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[certificaterotation] %s/%s: skipping paused operation", op.Namespace, op.Name)
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
		logrus.Errorf("[certificaterotation]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

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

	adapter, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return status, err
	}

	clusterObj, err := adapter.ClusterObject()
	if err != nil {
		return status, err
	}

	namespace, beaconName := adapter.BeaconRef()
	beacon, err := h.beacons.Get(namespace, beaconName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		logrus.Warnf("[certificaterotation]: %s/%s failed to find beacon %s/%s (clusterRef apiVersion=%s kind=%s name=%s)",
			op.Namespace, op.Name, namespace, beaconName, ustr.GetAPIVersion(), ustr.GetKind(), ustr.GetName())

		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")
		return status, nil
	} else if err != nil {
		return status, err
	}

	s := &scope{
		ownerKey:   plan.ControllerOwnerKey(op, ControllerOwnerKey),
		op:         op,
		namespace:  namespace,
		beacon:     beacon,
		clusterObj: clusterObj,
		adapter:    adapter,
	}

	switch status.Phase {
	case opv1alpha1.OperationPhasePending:
		return h.handlePending(s, status)
	case opv1alpha1.OperationPhaseInProgress:
		return h.handleInProgress(s, status)
	case opv1alpha1.OperationPhaseCanceled:
		return h.handleCanceled(s, status)
	case opv1alpha1.OperationPhaseFailed:
		return h.handleFailed(s, status)
	case opv1alpha1.OperationPhaseSucceeded:
		return h.handleSucceeded(s, status)
	default:
		status.SetPhase(opv1alpha1.OperationPhaseFailed)
		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))
		return status, nil
	}
}

func (h *handler) handlePending(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Tracef("[certificaterotation] %s/%s: handling pending", s.op.Namespace, s.op.Name)

	// Pending waits until this op is either the primary owner OR anywhere in the delegate chain.
	// If we're already in the chain, the primary owner is driving the beacon on our behalf — skip
	// AcquireBeacon entirely and continue with hook + WaitForRegister. Otherwise attempt to acquire;
	// a nil return means another controller currently owns it and we must keep waiting.
	if !plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		acquired, err := plan.AcquireBeacon(s.beacon, h.beacons, s.ownerKey)
		if err != nil {
			return status, err
		}
		if acquired == nil {
			opv1alpha1.PendingCondition.True(&status)
			opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
			opv1alpha1.PendingCondition.Message(&status, "waiting for beacon acquisition")
			return status, nil
		}
		s.beacon = acquired
	}

	delegated, err := h.handleHook(s, planv1alpha1.PendingPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.PendingCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	logrus.Infof("[certificaterotation] %s/%s: acquired beacon, waiting for agents to register", s.op.Namespace, s.op.Name)

	if ok, err := s.adapter.WaitForRegister(); err != nil {
		return status, err
	} else if !ok {
		logrus.Infof("[certificaterotation] %s/%s: waiting for system-agents to connect", s.op.Namespace, s.op.Name)
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForRegistrationReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for system-agents to connect")
		return status, nil
	}

	logrus.Infof("[certificaterotation] %s/%s: transitioning to rotate", s.op.Namespace, s.op.Name)

	status.Phase = opv1alpha1.OperationPhaseInProgress
	status.LastUpdated = metav1.Now()
	status.Step = opv1alpha1.CertificateRotationStepRotate

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

func (h *handler) handleInProgress(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	ownerKey := s.ownerKey
	stepPrefix := RotateStepHookLabelPrefix

	// Stage 1 (loose): the op must appear SOMEWHERE in the ownership chain (owner or any
	// delegate). Being absent entirely means the beacon was reassigned to another controller and
	// we can't recover. If a step hook is currently active on the op, treat the absence as a
	// step-scoped delegation and surface WaitingForDelegate instead of failing — the delegate may
	// have popped us in service of the hook and will restore ownership when the hook clears.
	if !plan.IsOwningBeaconHolder(s.beacon, ownerKey) && !plan.IsInDelegateChain(s.beacon, ownerKey) {
		if planv1alpha1.HasStepHookLabel(s.op, stepPrefix) {
			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
			opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
			return status, nil
		}
		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "beacon reassigned, aborting")

		return status, nil
	}

	var err error
	s.beacon, err = plan.ToggleBeacon(s.beacon, true, h.beacons)
	if err != nil {
		return status, err
	}

	// InProgress-phase hook fires on every InProgress reconcile, ahead of step dispatch.
	delegated, err := h.handleHook(s, planv1alpha1.InProgressPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// Stage 2 (strict): after the InProgress-phase hook has been handled, the op must be the
	// primary owner or the most-recent delegate on the chain to drive step work. If a step hook
	// is still active on the op, treat the missing-top state as an intentional delegation and
	// wait; otherwise this is a genuine beacon loss and we fail.
	if !plan.AuthorizedForBeacon(s.beacon, ownerKey) {
		if planv1alpha1.HasStepHookLabel(s.op, stepPrefix) {
			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
			opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
			return status, nil
		}
		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "beacon acquired by another controller, aborting")

		return status, nil
	}

	switch s.op.Status.Step {
	case opv1alpha1.CertificateRotationStepRotate:
		return h.reconcileRotate(s, status)
	}

	status.Phase = opv1alpha1.OperationPhaseFailed
	status.LastUpdated = metav1.Now()

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownStepReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("current step [%q] is unknown, expected: [%q]", status.Step, opv1alpha1.CertificateRotationStepRotate))

	return status, nil
}

func (h *handler) handleCanceled(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Debugf("[certificaterotation] %s/%s: handling operation canceled", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, planv1alpha1.CanceledPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.CanceledCondition.True(&status)
		opv1alpha1.CanceledCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.CanceledCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	if err := s.adapter.PauseCluster(false); err != nil {
		return status, err
	}

	ownerKey := s.ownerKey
	if plan.IsOwningBeaconHolder(s.beacon, ownerKey) || plan.IsInDelegateChain(s.beacon, ownerKey) {
		if err := plan.ReleaseBeacon(s.beacon, h.beacons, ownerKey); err != nil {
			return status, err
		}
	}
	return status, nil
}

func (h *handler) handleFailed(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Debugf("[certificaterotation] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, planv1alpha1.FailedPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	if err := s.adapter.PauseCluster(false); err != nil {
		return status, err
	}

	ownerKey := s.ownerKey
	if plan.IsOwningBeaconHolder(s.beacon, ownerKey) || plan.IsInDelegateChain(s.beacon, ownerKey) {
		if err := plan.ReleaseBeacon(s.beacon, h.beacons, ownerKey); err != nil {
			return status, err
		}
	}
	return status, nil
}

func (h *handler) handleSucceeded(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Debugf("[certificaterotation] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, planv1alpha1.SucceededPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.SucceededCondition.True(&status)
		opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.SucceededCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	if err := s.adapter.PauseCluster(false); err != nil {
		return status, err
	}

	ownerKey := s.ownerKey
	owning := plan.IsOwningBeaconHolder(s.beacon, ownerKey)
	if owning || plan.IsInDelegateChain(s.beacon, ownerKey) {
		if err := plan.ReleaseBeacon(s.beacon, h.beacons, ownerKey); err != nil {
			return status, err
		}
	}
	if owning {
		gvk := schema.FromAPIVersionAndKind(s.clusterObj.GetAPIVersion(), s.clusterObj.GetKind())
		_ = h.dynamic.Enqueue(gvk, s.clusterObj.GetNamespace(), s.clusterObj.GetName())
	}
	return status, nil
}

func updateStatus(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) opv1alpha1.CertificateRotationStatus {
	logrus.Tracef("[certificaterotation] %s/%s: updating conditions", op.Namespace, op.Name)

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

// lifecycleHookDelegate returns (suffix, delegate) for the first label on the operation whose key
// starts with prefix. Returns ("", "") when no such label is set.
func (h *handler) lifecycleHookDelegate(s *scope, prefix string) (string, string) {
	if s.op.Labels == nil {
		return "", ""
	}
	for k, v := range s.op.Labels {
		if strings.HasPrefix(k, prefix) {
			return strings.TrimPrefix(k, prefix), v
		}
	}
	return "", ""
}

// delegate pushes delegate onto the beacon's delegate chain if it is not already there. Idempotent
// across the reconciles that may occur while a hook is held.
func (h *handler) delegate(s *scope, name, delegate string) error {
	logrus.Tracef("[certificaterotation] %s/%s: delegating ownership of beacon to %s on behalf of %s", s.op.Namespace, s.op.Name, delegate, name)

	if plan.IsInDelegateChain(s.beacon, delegate) {
		return nil
	}

	beacon, err := plan.PushDelegate(s.beacon, delegate, h.beacons)
	if err != nil {
		return err
	}
	s.beacon = beacon
	return nil
}

// handleHook is the per-handler entry point for the lifecycle-hook mechanism. Returns (true, nil)
// while a label with the given prefix exists on the operation, signalling the caller to short
// circuit. To advance past the hook the operator must clear the label AND pop the delegate.
func (h *handler) handleHook(s *scope, prefix string) (bool, error) {
	logrus.Tracef("[certificaterotation] %s/%s: checking lifecycle hook for prefix %q", s.op.Namespace, s.op.Name, prefix)

	if name, delegate := h.lifecycleHookDelegate(s, prefix); delegate != "" {
		err := h.delegate(s, name, delegate)
		return true, err
	}
	return false, nil
}

// markFailed transitions status to the Failed phase with the given reason and condition message.
// Callers are responsible for logging before calling.
func markFailed(status *opv1alpha1.CertificateRotationStatus, reason, condMsg string) {
	status.Phase = opv1alpha1.OperationPhaseFailed
	status.LastUpdated = metav1.Now()
	opv1alpha1.FailedCondition.True(status)
	opv1alpha1.FailedCondition.Reason(status, reason)
	opv1alpha1.FailedCondition.Message(status, condMsg)
}

// operationEnv returns env vars that tie plan content to the operation UID and
// current step. This keeps rotate plans distinct so system-agent reruns them instead
// of reusing stale applied output.
func operationEnv(op *opv1alpha1.CertificateRotation, step opv1alpha1.CertificateRotationStep) []string {
	return []string{
		fmt.Sprintf("CERTIFICATE_ROTATION_OPERATION_UID=%s", op.UID),
		fmt.Sprintf("CERTIFICATE_ROTATION_STEP=%s", step),
	}
}

// runtimeAgentUnit returns the expected agent unit name for a given runtime command.
func runtimeAgentUnit(runtime string) string {
	return runtime + "-agent"
}

// servicesApply reports whether at least one of the requested services applies to the node described by secret.
func servicesApply(requested []string, secret *corev1.Secret) bool {
	if len(requested) == 0 {
		return true
	}
	relevant := map[string]struct{}{}
	if ops.IsWorker(secret) {
		// worker
		relevant["rke2-server"] = struct{}{}
		relevant["k3s-server"] = struct{}{}
		relevant["api-server"] = struct{}{}
		relevant["kubelet"] = struct{}{}
		relevant["kube-proxy"] = struct{}{}
		relevant["auth-proxy"] = struct{}{}
	}
	if ops.IsControlPlane(secret) {
		relevant["rke2-server"] = struct{}{}
		relevant["k3s-server"] = struct{}{}
		relevant["api-server"] = struct{}{}
		relevant["kubelet"] = struct{}{}
		relevant["kube-proxy"] = struct{}{}
		relevant["auth-proxy"] = struct{}{}
		relevant["controller-manager"] = struct{}{}
		relevant["scheduler"] = struct{}{}
		relevant["rke2-controller"] = struct{}{}
		relevant["k3s-controller"] = struct{}{}
		relevant["admin"] = struct{}{}
		relevant["cloud-controller"] = struct{}{}
	}
	if ops.IsEtcd(secret) {
		relevant["etcd"] = struct{}{}
		relevant["kubelet"] = struct{}{}
		relevant["k3s-server"] = struct{}{}
		relevant["rke2-server"] = struct{}{}
	}
	for _, s := range requested {
		if _, ok := relevant[s]; ok {
			return true
		}
	}
	return false
}

// reconcileRotate implements the rotate step: mark the beacon active, pause the cluster, and
// walk nodes in disruption-safe order assigning per-node certificate rotation plans and waiting
// for each to finish before proceeding.
func (h *handler) reconcileRotate(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Debugf("[certificaterotation] %s/%s: handling certificate rotation", s.op.Namespace, s.op.Name)

	// Step hook before PauseCluster so a delegate can inspect or modify the cluster's pre-pause
	// state. PauseCluster is idempotent so re-entering after the hook clears just no-ops.
	delegated, err := h.handleHook(s, RotateStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// Pause cluster reconciliation to avoid races during certificate replacement
	if err := s.adapter.PauseCluster(true); err != nil {
		return status, err
	}

	// Collect all targets once and deterministically sort them.
	targets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[certificaterotation] %s/%s: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)
		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	// Filter targets by whether the requested services apply to them.
	targetsFiltered := []*corev1.Secret{}
	for _, secret := range targets {
		if !servicesApply(s.op.Spec.Args.Services, secret) {
			continue
		}
		targetsFiltered = append(targetsFiltered, secret)
	}
	if len(targetsFiltered) == 0 {
		logrus.Errorf("[certificaterotation] %s/%s: no eligible machine-plan secrets found", s.op.Namespace, s.op.Name)
		markFailed(&status, opv1alpha1.PlanFailedReason, "no eligible machine-plan secrets found")
		return status, nil
	}

	env := operationEnv(s.op, status.Step)

	for _, secret := range targetsFiltered {
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		runtime := s.adapter.RuntimeCommand()
		serverUnit := s.adapter.ServerUnit()

		var nodePlan plan.Plan

		if ops.IsControlPlane(secret) || ops.IsEtcd(secret) {
			// server node: stop, rotate, restart
			args := []string{"certificate", "rotate"}
			if len(s.op.Spec.Args.Services) > 0 {
				for _, svc := range s.op.Spec.Args.Services {
					args = append(args, "-s", svc)
				}
			}

			provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
			dataDir := s.adapter.DistroDataDirectory(secret)
			files := []plan.File{ops.IdempotentScriptFile(provisioningDir)}
			oneTime := []plan.OneTimeInstruction{}
			// stop then rotate then cleanup manifests then restart. Do not clear prior rotate idempotency state.
			oneTime = append(oneTime, ops.IdempotentInstruction(provisioningDir, "certificate-rotation/stop", fmt.Sprintf("%s", s.op.UID), "systemctl", []string{"stop", serverUnit}, env))
			rotateInst := ops.IdempotentInstruction(provisioningDir, "certificate-rotation/rotate", fmt.Sprintf("%s", s.op.UID), runtime, args, env)
			oneTime = append(oneTime, rotateInst)

			// CAPR requires explicit component cert-dir and absence of tls-cert-file to safely remove
			// component certs. CertificateRotation lacks parsed per-node component config, so defer.

			if runtime == capr.RuntimeRKE2 {
				rmCmd := fmt.Sprintf("rm -rf %s/%s-*.yaml", path.Join(dataDir, "server/manifests"), runtime)
				oneTime = append(oneTime, ops.IdempotentInstruction(provisioningDir, "certificate-rotation/manifest-removal", fmt.Sprintf("%s", s.op.UID), "/bin/sh", []string{"-c", rmCmd}, []string{}))
			}

			// restart server unit idempotently
			oneTime = append(oneTime, ops.IdempotentInstruction(provisioningDir, "certificate-rotation/restart-reset-failed", fmt.Sprintf("%s", s.op.UID), "/bin/sh", []string{"-c", fmt.Sprintf("if [ $(systemctl is-failed %s) = failed ]; then systemctl reset-failed %s; fi", serverUnit, serverUnit)}, []string{}))
			oneTime = append(oneTime, ops.IdempotentInstruction(provisioningDir, "certificate-rotation/restart", fmt.Sprintf("%s", s.op.UID), "systemctl", []string{"restart", serverUnit}, []string{}))

			nodePlan = plan.Plan{
				Files:               files,
				OneTimeInstructions: oneTime,
				Probes:              probes,
			}
		} else {
			// worker: restart the runtime agent
			agentUnit := runtimeAgentUnit(runtime)
			if ops.IsWindows(secret) {
				// Windows idempotent restart using local idempotency helper; do not use linux systemctl logic.
				files := []plan.File{windowsIdempotentScriptFile()}
				oneTime := windowsIdempotentRestartInstructions("certificate-rotation/restart", fmt.Sprintf("%s", s.op.UID), capr.RuntimeRKE2)
				nodePlan = plan.Plan{
					Files:               files,
					OneTimeInstructions: oneTime,
					Probes:              probes,
				}
			} else {
				provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
				files := []plan.File{ops.IdempotentScriptFile(provisioningDir)}
				oneTime := []plan.OneTimeInstruction{}
				oneTime = append(oneTime, ops.IdempotentInstruction(provisioningDir, "certificate-rotation/restart-reset-failed", fmt.Sprintf("%s", s.op.UID), "/bin/sh", []string{"-c", fmt.Sprintf("if [ $(systemctl is-failed %s) = failed ]; then systemctl reset-failed %s; fi", agentUnit, agentUnit)}, []string{}))
				oneTime = append(oneTime, ops.IdempotentInstruction(provisioningDir, "certificate-rotation/restart", fmt.Sprintf("%s", s.op.UID), "systemctl", []string{"restart", agentUnit}, []string{}))
				nodePlan = plan.Plan{
					Files:               files,
					OneTimeInstructions: oneTime,
					Probes:              probes,
				}
			}
		}

		planStatus, err := h.store.AssignPlan(secret, &nodePlan, 0, 0)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[certificaterotation] %s/%s: certificate rotation plan failed for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)
			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("certificate rotation plan failed for %s/%s", secret.Namespace, secret.Name))
			return status, nil
		}

		if planStatus.Waiting() {
			logrus.Debugf("[certificaterotation] %s/%s: waiting for certificate rotation plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)
			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, plan.Message([]plan.PlanStatus{*planStatus}))
			return status, nil
		}
	}

	logrus.Infof("[certificaterotation] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.Phase = opv1alpha1.OperationPhaseSucceeded
	status.LastUpdated = metav1.Now()

	opv1alpha1.SucceededCondition.True(&status)
	opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.FinishedReason)
	opv1alpha1.SucceededCondition.Message(&status, "Operation completed successfully")

	return status, nil
}
