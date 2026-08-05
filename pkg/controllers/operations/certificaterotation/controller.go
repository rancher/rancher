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

var supportedCertificateRotationServices = map[string]struct{}{
	"admin":              {},
	"api-server":         {},
	"auth-proxy":         {},
	"cloud-controller":   {},
	"controller-manager": {},
	"etcd":               {},
	"k3s-controller":     {},
	"k3s-server":         {},
	"kubelet":            {},
	"kube-proxy":         {},
	"rke2-controller":    {},
	"rke2-server":        {},
	"scheduler":          {},
}

type certificateRotationComponentSettingsProvider interface {
	// CertificateRotationComponentTLSSettings returns an explicitly configured
	// controller-manager or scheduler serving certificate for this node. The
	// rotation plan uses this to avoid deleting the default generated certificate
	// when the component is configured to use a different certificate pair.
	CertificateRotationComponentTLSSettings(
		secret *corev1.Secret,
		component string,
	) ops.ComponentTLSSettings
}

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

	if status.Phase == opv1alpha1.OperationPhasePending {
		if err := validateCertificateRotationServices(op.Spec.Args.Services); err != nil {
			markFailed(&status, opv1alpha1.PlanFailedReason, err.Error())
			return status, nil
		}
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

func validateCertificateRotationServices(services []string) error {
	for _, service := range services {
		if _, ok := supportedCertificateRotationServices[service]; !ok {
			return fmt.Errorf("unsupported certificate rotation service %q", service)
		}
	}
	return nil
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
		// Workers restart their runtime agent when any of these shared runtime
		// certificates are rotated, so they can reconnect to the server.
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

func serviceRequested(requested []string, service string) bool {
	if len(requested) == 0 {
		return true
	}
	for _, requestedService := range requested {
		if requestedService == service {
			return true
		}
	}
	return false
}

// componentCertificateCleanupInstructions removes the default generated certificate/key pairs
// used by controller-manager and scheduler. Removing the pair before restarting the server lets
// the component generate a rotated replacement. Components with an explicit certificate and key
// are excluded because those paths are managed outside the runtime's default certificate directory.
func componentCertificateCleanupInstructions(provisioningDir, operationID, runtime, dataDir string, services []string, controllerManagerSettings, schedulerSettings ops.ComponentTLSSettings) []plan.OneTimeInstruction {
	components := []struct {
		service        string
		certificate    string
		certificateDir string
		manifest       string
		settings       ops.ComponentTLSSettings
	}{
		{
			service:        "controller-manager",
			certificate:    ops.DefaultKubeControllerManagerCert,
			certificateDir: ops.DefaultKubeControllerManagerCertDir,
			manifest:       "kube-controller-manager.yaml",
			settings:       controllerManagerSettings,
		},
		{
			service:        "scheduler",
			certificate:    ops.DefaultKubeSchedulerCert,
			certificateDir: ops.DefaultKubeSchedulerCertDir,
			manifest:       "kube-scheduler.yaml",
			settings:       schedulerSettings,
		},
	}

	instructions := []plan.OneTimeInstruction{}
	for _, component := range components {
		// A service-filtered operation must not restart or remove certificates for
		// components that were not selected by the caller.
		if !serviceRequested(services, component.service) {
			continue
		}
		// An explicit TLS pair is the component's active serving certificate. The
		// default generated paths are not used in that configuration.
		if component.settings.HasCompleteTLSConfig() {
			continue
		}

		// The default component certificate and key are recreated when the server
		// starts after the runtime certificate rotation command completes.
		certPath := path.Join(dataDir, component.certificateDir, component.certificate)
		keyPath := strings.TrimSuffix(certPath, ".crt") + ".key"
		instructions = append(instructions,
			ops.IdempotentInstruction(provisioningDir, "certificate-rotation/rm-"+component.service+"-cert", operationID, "rm", []string{"-f", certPath}, nil),
			ops.IdempotentInstruction(provisioningDir, "certificate-rotation/rm-"+component.service+"-key", operationID, "rm", []string{"-f", keyPath}, nil),
		)

		if runtime == capr.RuntimeRKE2 {
			// RKE2 regenerates the static-pod manifest when it is absent. Removing it
			// makes the restarted server use the newly generated component certificate.
			instructions = append(instructions,
				ops.IdempotentInstruction(provisioningDir, "certificate-rotation/rm-"+component.service+"-spm", operationID, "rm", []string{"-f", path.Join(dataDir, "agent/pod-manifests", component.manifest)}, nil),
			)
		}
	}

	return instructions
}

// certificateRotationStopInstructions stops the runtime server before its certificates are
// changed. Keeping this as a discrete idempotent action makes retries safe.
func certificateRotationStopInstructions(provisioningDir, operationID, serverUnit string, env []string) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		ops.IdempotentInstruction(provisioningDir, "certificate-rotation/stop", operationID, "systemctl", []string{"stop", serverUnit}, env),
	}
}

// certificateRotationRuntimeInstructions invokes the runtime's certificate rotation command.
// An empty services slice deliberately rotates every service supported by the runtime.
func certificateRotationRuntimeInstructions(provisioningDir, operationID, runtime string, services []string, env []string) []plan.OneTimeInstruction {
	args := []string{"certificate", "rotate"}
	for _, service := range services {
		args = append(args, "-s", service)
	}

	return []plan.OneTimeInstruction{
		ops.IdempotentInstruction(provisioningDir, "certificate-rotation/rotate", operationID, runtime, args, env),
	}
}

// rke2ManifestRemovalInstructions removes generated RKE2 manifests so the server recreates
// them using the rotated certificates when it starts again.
func rke2ManifestRemovalInstructions(provisioningDir, operationID, dataDir string) []plan.OneTimeInstruction {
	rmCmd := fmt.Sprintf("rm -rf %s/rke2-*.yaml", path.Join(dataDir, "server/manifests"))
	return []plan.OneTimeInstruction{
		ops.IdempotentInstruction(provisioningDir, "certificate-rotation/manifest-removal", operationID, "/bin/sh", []string{"-c", rmCmd}, nil),
	}
}

// linuxIdempotentRestartInstructions resets a failed systemd unit when needed, then restarts it.
// Keeping the systemctl commands together gives each restart the same retry-safe behavior.
func linuxIdempotentRestartInstructions(provisioningDir, identifier, value, service string) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		ops.IdempotentInstruction(provisioningDir, identifier+"-reset-failed", value, "/bin/sh", []string{"-c", fmt.Sprintf("if [ $(systemctl is-failed %s) = failed ]; then systemctl reset-failed %s; fi", service, service)}, nil),
		ops.IdempotentInstruction(provisioningDir, identifier+"-restart", value, "systemctl", []string{"restart", service}, nil),
	}
}

// reconcileRotate pauses normal cluster reconciliation, assigns a rotation plan to one target at
// a time in disruption-safe order, and waits for that target to recover before advancing.
func (h *handler) reconcileRotate(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Debugf("[certificaterotation] %s/%s: handling certificate rotation", s.op.Namespace, s.op.Name)

	// Run the step hook before pausing the cluster so a delegate sees the normal
	// pre-rotation state. Reconciliation resumes here after the delegate clears.
	delegated, err := h.handleHook(s, RotateStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// Prevent the cluster provisioner from changing node plans while the operation
	// replaces certificates and restarts runtimes.
	if err := s.adapter.PauseCluster(true); err != nil {
		return status, err
	}

	// Collect all registered machine-plan secrets in the collector's safe role order.
	// The service filter removes nodes that cannot run any requested service; keeping
	// it in the collector ensures sorting and empty-target handling use the same set.
	targets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithFilter(func(secret *corev1.Secret) bool {
			return servicesApply(s.op.Spec.Args.Services, secret)
		}).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[certificaterotation] %s/%s: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)
		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	if len(targets) == 0 {
		logrus.Errorf("[certificaterotation] %s/%s: no eligible machine-plan secrets found", s.op.Namespace, s.op.Name)
		markFailed(&status, opv1alpha1.PlanFailedReason, "no eligible machine-plan secrets found")
		return status, nil
	}

	// Pass the operation identity to runtime instructions so their execution is
	// associated with this rotation attempt and step.
	env := operationEnv(s.op, status.Step)

	for _, secret := range targets {
		// Plans are processed serially. Returning while one plan is waiting ensures
		// the next node is not disrupted until this node has applied and passed probes.
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		runtime := s.adapter.RuntimeCommand()
		serverUnit := s.adapter.ServerUnit()

		var nodePlan plan.Plan

		if ops.IsControlPlane(secret) || ops.IsEtcd(secret) {
			// Server nodes own control-plane or etcd certificates. Stop the server
			// before rotating them, then restart it after all required cleanup.
			provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
			dataDir := s.adapter.DistroDataDirectory(secret)
			files := []plan.File{ops.IdempotentScriptFile(provisioningDir)}
			oneTime := certificateRotationStopInstructions(provisioningDir, string(s.op.UID), serverUnit, env)
			// Keep stop and rotate as separate idempotent instructions. A retry can
			// resume safely without rerunning an instruction already applied by the agent.
			oneTime = append(oneTime, certificateRotationRuntimeInstructions(provisioningDir, string(s.op.UID), runtime, s.op.Spec.Args.Services, env)...)

			if ops.IsControlPlane(secret) {
				var controllerManagerSettings, schedulerSettings ops.ComponentTLSSettings
				if provider, ok := s.adapter.(certificateRotationComponentSettingsProvider); ok {
					// Some adapters can identify an explicit component TLS pair. Pass those
					// settings to cleanup so rotation never removes an externally managed pair.
					controllerManagerSettings = provider.CertificateRotationComponentTLSSettings(secret, ops.KubeControllerManagerProbeName)
					schedulerSettings = provider.CertificateRotationComponentTLSSettings(secret, ops.KubeSchedulerProbeName)
				}
				oneTime = append(oneTime, componentCertificateCleanupInstructions(provisioningDir, string(s.op.UID), runtime, dataDir, s.op.Spec.Args.Services, controllerManagerSettings, schedulerSettings)...)
			}

			if runtime == capr.RuntimeRKE2 {
				// RKE2 regenerates its generated manifests during server startup. Remove
				// them only after rotation so replacement manifests refer to rotated files.
				oneTime = append(oneTime, rke2ManifestRemovalInstructions(provisioningDir, string(s.op.UID), dataDir)...)
			}

			// Restarting the server activates the rotated certificates and lets the
			// probes verify that its local control-plane components recovered.
			oneTime = append(oneTime, linuxIdempotentRestartInstructions(provisioningDir, "certificate-rotation", string(s.op.UID), serverUnit)...)

			nodePlan = plan.Plan{
				Files:               files,
				OneTimeInstructions: oneTime,
				Probes:              probes,
			}
		} else {
			// Workers do not rotate server certificates, but their runtime agent must
			// restart so it reconnects using the updated cluster certificates.
			agentUnit := runtimeAgentUnit(runtime)
			if ops.IsWindows(secret) {
				// Windows uses a service restart instruction rather than Linux systemctl.
				files := []plan.File{windowsIdempotentScriptFile()}
				oneTime := windowsIdempotentRestartInstructions("certificate-rotation/restart", string(s.op.UID), capr.RuntimeRKE2)
				nodePlan = plan.Plan{
					Files:               files,
					OneTimeInstructions: oneTime,
					Probes:              probes,
				}
			} else {
				provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
				files := []plan.File{ops.IdempotentScriptFile(provisioningDir)}
				oneTime := linuxIdempotentRestartInstructions(provisioningDir, "certificate-rotation", string(s.op.UID), agentUnit)
				nodePlan = plan.Plan{
					Files:               files,
					OneTimeInstructions: oneTime,
					Probes:              probes,
				}
			}
		}

		// AssignPlan updates this machine-plan secret and returns the agent's latest
		// applied status for the same plan. A later reconcile continues from that status.
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
			// Do not assign a plan to another target until the current target reports
			// both instruction completion and successful probes.
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
