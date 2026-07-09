package encryptionkeyrotation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// ControllerOwnerKey is the shared operation-type key for encryption key rotation coordination.
// Beacon ownership uses a per-operation key derived from the operation UID.
const ControllerOwnerKey = "encryption-key-rotation"
const beaconOwnerRefAnnotation = "rke.cattle.io/operation-owner-ref"

// Step hook label prefixes for the encryptionkeyrotation operation. Each prefix gates a single
// rotation step and follows the shared label semantics documented on planv1alpha1's phase-hook
// label constants.
const (
	// RotateStepHookLabelPrefix gates the Rotate step, before reconcileRotate pauses the CAPI
	// cluster and assigns the rotate-keys plan to the elected control-plane leader. Fires before
	// PauseCluster so a delegate observes the cluster in its pre-pause state.
	RotateStepHookLabelPrefix = "rotate.step.hook.operation.cattle.io/"

	// RestartStepHookLabelPrefix gates the Restart step, before reconcileRestart begins walking
	// the server pool and issuing the systemctl-restart plan to each node.
	RestartStepHookLabelPrefix = "restart.step.hook.operation.cattle.io/"
)

// dynamicResolver is the subset of *dynamic.Controller this handler needs:
// Get for resolving cluster refs and Enqueue for nudging the backing cluster
// after terminal beacon transitions.
type dynamicResolver interface {
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
	Enqueue(gvk schema.GroupVersionKind, namespace, name string) error
}

type handler struct {
	encryptionkeyrotations operationcontrollers.EncryptionKeyRotationController

	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache

	secrets corecontrollers.SecretClient

	store *plan.Store

	dynamic dynamicResolver

	clients *wrangler.CAPIContext
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		encryptionkeyrotations: clients.Operation.EncryptionKeyRotation(),
		beacons:                clients.Plan.Beacon(),
		beaconCache:            clients.Plan.Beacon().Cache(),
		secrets:                clients.Core.Secret(),
		dynamic:                clients.Dynamic,
		store:                  plan.NewStore(clients.Core.Secret()),
		clients:                clients,
	}

	operationcontrollers.RegisterEncryptionKeyRotationStatusHandler(ctx, clients.Operation.EncryptionKeyRotation(), "", "encryption-key-rotation-handler", h.OnChange)
}

func (h *handler) OnChange(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
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

	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	if equality.Semantic.DeepEqual(op.Status, status) {
		// handle after normal processing to allow for proper phase-related cleanup (freeing beacon)
		//
		// See the equivalent guard in etcdsnapshotsave's OnChange for the rationale: while any
		// lifecycle-hook label is still on the op, TTL garbage collection must be deferred so the
		// delegate has a chance to observe the terminal phase and pop itself from the beacon.
		// EKR is particularly exposed to this without the guard because the operation defaults
		// to TTL=0 (immediate expiry) in some code paths.
		if ops.IsTerminal(status.Phase) &&
			ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) &&
			!planv1alpha1.HasActiveLifecycleHook(op) {
			err = h.encryptionkeyrotations.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
			if err != nil {
				return status, err
			}
			return status, generic.ErrSkip
		}
		h.encryptionkeyrotations.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)
	}
	return status, nil
}

func (h *handler) onChange(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
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

	adapter, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return status, err
	}

	// Resolve the beacon via the adapter, not op.Spec.ClusterRef. See
	// etcdsnapshotrestore/controller.go for the full rationale.
	namespace, beaconName := adapter.BeaconRef()

	beacon, err := h.beacons.Get(namespace, beaconName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		logrus.Warnf("[encryptionkeyrotation]: %s/%s failed to find beacon %s/%s (clusterRef apiVersion=%s kind=%s name=%s)",
			op.Namespace, op.Name, namespace, beaconName, ustr.GetAPIVersion(), ustr.GetKind(), ustr.GetName())

		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")

		return status, nil
	} else if err != nil {
		return status, err
	}

	s := &scope{
		op:         op,
		beacon:     beacon,
		namespace:  namespace,
		clusterObj: &ustr,
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
		// Should be prevented via validation, but just in case
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))
	}

	return status, nil
}

type scope struct {
	op        *opv1alpha1.EncryptionKeyRotation
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
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
	logrus.Tracef("[encryptionkeyrotation] %s/%s: delegating ownership of beacon to %s on behalf of %s", s.op.Namespace, s.op.Name, delegate, name)

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
// circuit. To advance past the hook the operator must clear the label AND pop the delegate (see
// AdvancePastEncryptionKeyRotationHook in the test helpers for the rationale on ordering).
func (h *handler) handleHook(s *scope, prefix string) (bool, error) {
	logrus.Tracef("[encryptionkeyrotation] %s/%s: checking lifecycle hook for prefix %q", s.op.Namespace, s.op.Name, prefix)

	if name, delegate := h.lifecycleHookDelegate(s, prefix); delegate != "" {
		err := h.delegate(s, name, delegate)
		return true, err
	}
	return false, nil
}

func (h *handler) handlePending(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	ownerKey := beaconOwnerKey(s.op)
	if err := h.reclaimStaleBeaconOwnerIfNeeded(s); err != nil {
		return status, err
	}

	beacon, err := plan.AcquireBeacon(s.beacon, h.beacons, ownerKey)
	if err != nil {
		return status, err
	}
	if beacon == nil {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon acquisition")
		return status, nil
	}

	desiredOwnerRef := fmt.Sprintf("%s/%s/%s", s.op.Namespace, s.op.Name, s.op.UID)
	if beacon.Annotations == nil || beacon.Annotations[beaconOwnerRefAnnotation] != desiredOwnerRef {
		beacon = beacon.DeepCopy()
		if beacon.Annotations == nil {
			beacon.Annotations = map[string]string{}
		}
		beacon.Annotations[beaconOwnerRefAnnotation] = desiredOwnerRef
		beacon, err = h.beacons.Update(beacon)
		if err != nil {
			return status, err
		}
	}
	s.beacon = beacon

	// Pending-phase hook fires after beacon acquisition + owner-ref tagging so a delegate can
	// inspect the recorded ownership before the controller starts driving the rotation.
	delegated, err := h.handleHook(s, planv1alpha1.PendingPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.PendingCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
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
	if !plan.AuthorizedForBeacon(s.beacon, beaconOwnerKey(s.op)) {
		status.Phase = opv1alpha1.OperationPhaseFailed
		status.LastUpdated = metav1.Now()

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "beacon acquired by another controller, aborting")

		return status, nil
	}

	var err error
	s.beacon, err = plan.ToggleBeacon(s.beacon, true, h.beacons)
	if err != nil {
		return status, err
	}

	// InProgress-phase hook fires on every InProgress reconcile, ahead of step dispatch — useful
	// for delegates that need to gate ALL step work uniformly without subscribing to each
	// individual step prefix.
	delegated, err := h.handleHook(s, planv1alpha1.InProgressPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
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

// reconcileRotate runs `secrets-encrypt rotate-keys` on the elected leader and
// stays in Rotate until status reports `reencrypt_finished` on that node.
func (h *handler) reconcileRotate(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling secrets-encrypt rotate-keys", s.op.Namespace, s.op.Name)

	// Hook check before PauseCluster so a delegate can inspect or modify the cluster's pre-pause
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

	leader, err := s.adapter.FindOrElectLeader(ControllerOwnerKey, ops.IsControlPlane)
	if err != nil {
		return status, err
	}

	if leader == nil {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: no suitable control-plane leader found yet, will retry", s.op.Namespace, s.op.Name)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForSuitableLeaderReason)
		opv1alpha1.InProgressCondition.Message(&status, "waiting for a suitable control-plane leader for encryption key rotation")
		return status, nil
	}

	// Pause the CAPI cluster while rotate-keys is active so unrelated activity
	// does not race with the encryption-key rotation plan.
	if err := s.adapter.PauseCluster(true); err != nil {
		return status, err
	}

	probes, err := s.adapter.RenderProbes(leader, true)
	if err != nil {
		return status, err
	}

	env := operationEnv(s.op, status.Step)
	runtime := s.adapter.RuntimeCommand()

	nodePlan := &plan.Plan{
		OneTimeInstructions: []plan.OneTimeInstruction{
			// 1. Run rotate-keys via wrapper that always exits 0; captures real exit code in output.
			{
				CommonInstruction: plan.CommonInstruction{
					Name:    rotateKeysInstructionName,
					Command: "/bin/sh",
					Args:    []string{"-c", rotateKeysScript(runtime)},
					Env:     env,
				},
				SaveOutput: true,
			},
			// 2. Poll until secrets-encrypt status responds; gates planStatus.Applied until
			// the encryption server is reachable after key reload.
			{
				CommonInstruction: plan.CommonInstruction{
					Name:    waitForStatusInstructionName,
					Command: "/bin/sh",
					Args:    []string{"-c", waitForStatusScript(runtime)},
					Env:     env,
				},
			},
			// 3. One-time status snapshot captured when the plan is applied; provides an
			// observability anchor and confirms the endpoint is stable.
			{
				CommonInstruction: plan.CommonInstruction{
					Name:    statusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     env,
				},
				SaveOutput: true,
			},
		},
		PeriodicInstructions: []plan.PeriodicInstruction{
			// Runs every 5s independently; used for stage/hash convergence checking.
			{
				CommonInstruction: plan.CommonInstruction{
					Name:    statusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     env,
				},
				PeriodSeconds: 5,
			},
		},
		Probes: probes,
	}

	// Use finite failure threshold so a plan that can't execute
	// is marked Failed rather than retried forever. The wrapper always exits 0, so a
	// real apply failure here means the wrapper itself couldn't run.
	planStatus, err := h.store.AssignPlan(leader, nodePlan, 1, 1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: rotate-keys plan failed to execute on leader %s", s.op.Namespace, s.op.Name, leader.Name)
		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encryption key rotation plan failed for leader %s/%s", leader.Namespace, leader.Name))
		return status, nil
	}

	if planStatus.Waiting() {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for rotate-keys plan for %s/%s", s.op.Namespace, s.op.Name, leader.Namespace, leader.Name)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, plan.Message([]plan.PlanStatus{*planStatus}))

		return status, nil
	}

	// Plan applied and probes passed. Read the one-time output to check the rotate-keys exit code.
	appliedOutput, err := plan.ReadAppliedOutput(leader)
	if err != nil {
		return status, err
	}
	if appliedOutput == nil {
		// Output not yet in cache; wait for the next reconcile.
		logrus.Debugf("[encryptionkeyrotation] %s/%s: rotate-keys applied-output not yet available on leader %s", s.op.Namespace, s.op.Name, leader.Name)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, "waiting for rotate-keys output")
		return status, nil
	}

	result, err := readRotateKeysResult(appliedOutput)
	if errors.Is(err, errRotateKeysOutputNotYet) {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: rotate-keys exit code not yet available on leader %s", s.op.Namespace, s.op.Name, leader.Name)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, "waiting for rotate-keys exit code")
		return status, nil
	}
	if err != nil {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: corrupt rotate-keys output on leader %s: %v", s.op.Namespace, s.op.Name, leader.Name, err)
		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("corrupt rotate-keys output on leader %s", leader.Name))
		return status, nil
	}

	if result.exitCode != 0 {
		if rotateKeysCommandTimedOut(result.output) {
			// CLI timed out but rotation may still be running in the background;
			// keep watching periodic status before deciding.
			logrus.Warnf("[encryptionkeyrotation] %s/%s: rotate-keys CLI timed out on leader %s; continuing to observe periodic status", s.op.Namespace, s.op.Name, leader.Name)
		} else {
			logrus.Errorf("[encryptionkeyrotation] %s/%s: rotate-keys failed on leader %s with exit code %d", s.op.Namespace, s.op.Name, leader.Name, result.exitCode)
			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("secrets-encrypt rotate-keys failed on leader %s (exit code %d); please perform an etcd restore", leader.Name, result.exitCode))
			return status, nil
		}
	}

	// Check periodic secrets-encrypt status. Stay in Rotate until reencrypt_finished.
	waitMsg, err := convergenceWaitMessage(leader, false)
	if err != nil {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: convergence check failed on leader %s: %v", s.op.Namespace, s.op.Name, leader.Name, err)
		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("corrupt encryption key rotation state on leader %s; please perform an etcd restore", leader.Name))
		return status, nil
	}
	if waitMsg != "" {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for convergence on leader %s: %s", s.op.Namespace, s.op.Name, leader.Name, waitMsg)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForEncryptionKeyRotationReason)
		opv1alpha1.InProgressCondition.Message(&status, waitMsg)
		return status, nil
	}

	logrus.Infof("[encryptionkeyrotation] %s/%s: rotate-keys reencrypt_finished on leader %s, transitioning to restart", s.op.Namespace, s.op.Name, leader.Name)
	status.Step = opv1alpha1.EncryptionKeyRotationStepRestart
	return status, nil
}

// reconcileRestart walks the server nodes in sorted order, restarting each one
// and requiring strict hash convergence only on the final control-plane node.
func (h *handler) reconcileRestart(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling service restart", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, RestartStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// Restart order comes from plan.DefaultSorter(): init+etcd first, then
	// etcd-only, then mixed etcd/control-plane, then control-plane-only. That
	// keeps etcd nodes ahead of pure control-plane nodes.
	secrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithLabels(
			plan.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			plan.Or(
				plan.Label(capr.EtcdRoleLabel, "true"),
				plan.Label(capr.ControlPlaneRoleLabel, "true"),
			)).
		WithSorter(plan.DefaultSorter()).
		WithValidator(plan.AtLeast(1, "")).
		Collect()

	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: no control-plane nodes found at restart step", s.op.Namespace, s.op.Name)
		markFailed(&status, opv1alpha1.UnknownStepReason, "no control-plane nodes found; cannot verify post-restart encryption status")
		return status, nil
	}
	if !ops.IsControlPlane(secrets[len(secrets)-1]) {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: nodes are not correctly ordered at restart step", s.op.Namespace, s.op.Name)
		markFailed(&status, opv1alpha1.UnknownStepReason, "last control plane node not found; cannot verify hash convergence after restart")
		return status, nil
	}

	env := operationEnv(s.op, status.Step)
	serverUnit := s.adapter.ServerUnit()
	runtime := s.adapter.RuntimeCommand()

	// pool is ordered with plan.DefaultSorter(), so the final element is the
	// last control-plane node. For the new k3s rotate-keys flow, strict "All
	// hashes match" validation only makes sense after every control-plane node
	// has restarted; before that, k3s may legitimately report
	// reencrypt_finished while hashes still differ across servers.
	for i, secret := range secrets {
		requireHashMatch := i == len(secrets)-1
		status, done, err := h.reconcileRestartNode(s, status, secret, env, serverUnit, runtime, requireHashMatch)
		if err != nil {
			return status, err
		}
		if !done {
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

// reconcileRestartNode assigns and tracks the restart plan for one node. For
// control-plane nodes it also waits for post-restart secrets-encrypt status,
// and on the final control-plane node it enforces the cluster-wide hash check
// required by the k3s rotate-keys flow.
func (h *handler) reconcileRestartNode(
	s *scope,
	status opv1alpha1.EncryptionKeyRotationStatus,
	secret *corev1.Secret,
	env []string,
	serverUnit string,
	runtime string,
	requireHashMatch bool,
) (opv1alpha1.EncryptionKeyRotationStatus, bool, error) {
	probes, err := s.adapter.RenderProbes(secret, true)
	if err != nil {
		return status, false, err
	}

	oneTimeInstructions := []plan.OneTimeInstruction{
		{
			CommonInstruction: plan.CommonInstruction{
				Name:    "restart",
				Command: "systemctl",
				Args:    []string{"restart", serverUnit},
				Env:     env,
			},
		},
		{
			CommonInstruction: plan.CommonInstruction{
				Name:    "wait-for-systemctl-status",
				Command: "/bin/sh",
				Args:    []string{"-c", waitForSystemctlStatusScript(serverUnit)},
				Env:     env,
			},
		},
	}

	nodePlan := &plan.Plan{
		OneTimeInstructions: oneTimeInstructions,
		Probes:              probes,
	}
	if ops.IsControlPlane(secret) {
		nodePlan.OneTimeInstructions = append(nodePlan.OneTimeInstructions,
			plan.OneTimeInstruction{
				CommonInstruction: plan.CommonInstruction{
					Name:    waitForStatusInstructionName,
					Command: "/bin/sh",
					Args:    []string{"-c", waitForStatusScript(runtime)},
					Env:     env,
				},
			},
			plan.OneTimeInstruction{
				CommonInstruction: plan.CommonInstruction{
					Name:    statusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     env,
				},
				SaveOutput: true,
			},
		)
		nodePlan.PeriodicInstructions = []plan.PeriodicInstruction{
			{
				CommonInstruction: plan.CommonInstruction{
					Name:    statusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     env,
				},
				PeriodSeconds: 5,
			},
		}
	}

	planStatus, err := h.store.AssignPlan(secret, nodePlan, 5, 5)
	if err != nil {
		return status, false, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: restart plan failed for %s", s.op.Namespace, s.op.Name, secret.Name)
		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("restart failed for %s; please perform an etcd restore", secret.Name))
		return status, false, nil
	}

	if planStatus.Waiting() {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for restart for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, plan.Message([]plan.PlanStatus{*planStatus}))
		return status, false, nil
	}

	if ops.IsControlPlane(secret) {
		waitMsg, err := convergenceWaitMessage(secret, requireHashMatch)
		if err != nil {
			logrus.Errorf("[encryptionkeyrotation] %s/%s: convergence check failed on %s: %v", s.op.Namespace, s.op.Name, secret.Name, err)
			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("corrupt encryption key rotation state on %s; please perform an etcd restore", secret.Name))
			return status, false, nil
		}
		if waitMsg != "" {
			logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for convergence on %s: %s", s.op.Namespace, s.op.Name, secret.Name, waitMsg)
			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForEncryptionKeyRotationReason)
			opv1alpha1.InProgressCondition.Message(&status, waitMsg)
			return status, false, nil
		}
	}
	return status, true, nil
}

// handleCanceled is the terminal handler for the Canceled phase. Like handleFailed/handleSucceeded
// it runs its phase hook first so a delegate can observe cancellation, then unpauses the cluster
// and releases the beacon if we still own it. The cancel-vs-fail distinction is that an external
// party cancels whereas the operation fails itself — neither implies the other.
func (h *handler) handleCanceled(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation canceled", s.op.Namespace, s.op.Name)

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

	if plan.IsOwningBeaconHolder(s.beacon, beaconOwnerKey(s.op)) {
		if err := plan.ReleaseBeacon(s.beacon, h.beacons, beaconOwnerKey(s.op)); err != nil {
			return status, err
		}
	}
	return status, nil
}

// handleFailed releases the beacon and unpauses cluster activity after the
// operation has already been marked terminal.
func (h *handler) handleFailed(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	// Failed-phase hook fires before cleanup so a delegate can inspect failure state (status
	// conditions, plan-secret applied output, residual rotate-keys process) before the cluster is
	// unpaused and the beacon released.
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

	if err := plan.ReleaseBeacon(s.beacon, h.beacons, beaconOwnerKey(s.op)); err != nil {
		return status, err
	}
	return status, nil
}

// handleSucceeded clears the active beacon state, releases ownership, unpauses
// cluster activity, and re-enqueues the backing cluster so downstream
// controllers observe the final beacon transition.
func (h *handler) handleSucceeded(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

	// Succeeded-phase hook fires before unpausing + releasing — delegates use this to chain
	// follow-up work (e.g. a verifier that re-runs `secrets-encrypt status` from outside the
	// operation) before the cluster goes back to accepting new operations.
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

	if plan.AuthorizedForBeacon(s.beacon, beaconOwnerKey(s.op)) {
		s.beacon, err = plan.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		if err := plan.ReleaseBeacon(s.beacon, h.beacons, beaconOwnerKey(s.op)); err != nil {
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

// markFailed transitions status to the Failed phase with the given reason and condition message.
// Callers are responsible for logging before calling.
func markFailed(status *opv1alpha1.EncryptionKeyRotationStatus, reason, condMsg string) {
	status.Phase = opv1alpha1.OperationPhaseFailed
	status.LastUpdated = metav1.Now()
	opv1alpha1.FailedCondition.True(status)
	opv1alpha1.FailedCondition.Reason(status, reason)
	opv1alpha1.FailedCondition.Message(status, condMsg)
}

// beaconOwnerKey returns the per-operation beacon owner key used for
// beacon ownership checks and lifecycle cleanup.
func beaconOwnerKey(op *opv1alpha1.EncryptionKeyRotation) string {
	if op == nil {
		return ControllerOwnerKey
	}
	if op.UID != "" {
		return fmt.Sprintf("%s-%s", ControllerOwnerKey, op.UID)
	}
	return fmt.Sprintf("%s-%s-%s", ControllerOwnerKey, op.Namespace, op.Name)
}

// reclaimStaleBeaconOwnerIfNeeded clears stale beacon ownership when the
// recorded owner reference is invalid, missing, deleted, or terminal.
// Non-matching owners are left untouched so this controller only reclaims its own
// operation type.
func (h *handler) reclaimStaleBeaconOwnerIfNeeded(s *scope) error {
	if s.beacon == nil || s.beacon.Labels == nil {
		return nil
	}

	currentOwnerKey := s.beacon.Status.Owner
	newOwnerKey := beaconOwnerKey(s.op)
	// No owner, or we already own it
	if currentOwnerKey == "" || currentOwnerKey == newOwnerKey {
		return nil
	}
	// Another controller type owns it
	if currentOwnerKey != ControllerOwnerKey && !strings.HasPrefix(currentOwnerKey, ControllerOwnerKey+"-") {
		return nil
	}

	reclaim := false

	ownerRef := ""
	if s.beacon.Annotations != nil {
		ownerRef = s.beacon.Annotations[beaconOwnerRefAnnotation]
	}
	parts := strings.SplitN(ownerRef, "/", 3)
	// Missing or broken owner ref means we cannot trust this owner
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		reclaim = true
	} else {
		currentOp, err := h.encryptionkeyrotations.Get(parts[0], parts[1], metav1.GetOptions{})
		// Owner object is gone
		if apierrors.IsNotFound(err) {
			reclaim = true
		} else if err != nil {
			return err
			// UID changed or owner finished
		} else if string(currentOp.UID) != parts[2] || ops.IsTerminal(currentOp.Status.Phase) {
			reclaim = true
		}
	}
	if !reclaim {
		return nil
	}

	beacon := s.beacon.DeepCopy()
	beacon.Status.Owner = ""
	updated, err := h.beacons.Update(beacon)
	if err != nil {
		return err
	}
	s.beacon = updated

	beacon = s.beacon.DeepCopy()
	if beacon.Annotations != nil {
		delete(beacon.Annotations, beaconOwnerRefAnnotation)
	}
	updated, err = h.beacons.Update(beacon)
	if err != nil {
		return err
	}
	s.beacon = updated

	return nil
}

// operationEnv returns env vars that tie plan content to the operation UID and
// current step. This keeps rotate and restart plans byte-distinct so
// system-agent reruns them instead of reusing stale applied output.
func operationEnv(op *opv1alpha1.EncryptionKeyRotation, step opv1alpha1.EncryptionKeyRotationStep) []string {
	return []string{
		fmt.Sprintf("ENCRYPTION_KEY_ROTATION_OPERATION_UID=%s", op.UID),
		fmt.Sprintf("ENCRYPTION_KEY_ROTATION_STEP=%s", step),
	}
}

// errRotateKeysOutputNotYet is returned by readRotateKeysResult when the rotate-keys output
// or exit-code line is not yet written to the plan secret. Callers should wait and retry.
// Corrupt/unparse-able exit codes return a different error so callers can fail the operation.
var errRotateKeysOutputNotYet = errors.New("rotate-keys output not yet available")

// errStatusTimeout is returned by statusFromOutput when a transient CLI timeout is
// detected in the secrets-encrypt status output. Callers should wait for the next periodic
// run rather than failing the operation.
var errStatusTimeout = errors.New("secrets-encrypt status timed out")

const (
	rotateKeysInstructionName    = "rotate-keys"
	statusPeriodicName           = "secrets-encrypt-status"
	waitForStatusInstructionName = "wait-for-secrets-encrypt-status"
	stageReencryptFinished       = "reencrypt_finished"
	hashesMatchMessage           = "All hashes match"
	exitCodePrefix               = "rancher-rotate-keys-exit-code="

	// rotateKeysTimeoutMessage and rotateKeysTimeoutEndpoint are combined with
	// timeoutMarkers to identify CLI timeouts from the rotate-keys wrapper output.
	rotateKeysTimeoutMessage  = "see server log for details"
	rotateKeysTimeoutEndpoint = "/encrypt/config"

	// statusTimeoutEndpoint identifies a transient timeout from secrets-encrypt status.
	statusTimeoutEndpoint = "/encrypt/status"
)

// timeoutMarkers are the known CLI timeout signatures from secrets-encrypt calls.
var timeoutMarkers = []string{
	"Client.Timeout exceeded while awaiting headers",
	"timeout awaiting response headers",
	"context deadline exceeded",
}

type commandResult struct {
	output   string
	exitCode int
}

type runtimeStatus struct {
	stage         string
	hashesMatch   bool
	hashesPresent bool // false when the output omits the "Server Encryption Hashes:" line
}

// waitForSystemctlStatusScript returns a shell one-liner that polls systemctl is-active
// up to 30 times at 10s intervals and exits 1 on timeout. Used after service restart to
// confirm the unit came back before checking encryption status.
func waitForSystemctlStatusScript(serverUnit string) string {
	return fmt.Sprintf(
		`i=0; while [ $i -lt 30 ]; do systemctl is-active %s && exit 0; sleep 10; i=$((i+1)); done; exit 1`,
		serverUnit)
}

// waitForStatusScript returns a shell one-liner that polls secrets-encrypt status until it
// exits 0, giving the encryption server time to come back after rotate-keys. Up to 10
// retries at 10s intervals (100s total); exits 1 on timeout.
func waitForStatusScript(runtime string) string {
	return fmt.Sprintf(
		`i=0; while [ $i -lt 10 ]; do %s secrets-encrypt status && exit 0; sleep 10; i=$((i+1)); done; exit 1`,
		runtime)
}

// rotateKeysScript returns the shell wrapper command that captures secrets-encrypt rotate-keys'
// exit code in stdout and always exits 0, so system-agent never marks the plan failed on a
// CLI timeout. The controller classifies timeout vs real failure itself.
func rotateKeysScript(runtime string) string {
	return strings.Join([]string{
		fmt.Sprintf(`output="$(%s secrets-encrypt rotate-keys 2>&1)"`, runtime),
		"exitCode=$?",
		`printf '%s\n' "$output"`,
		fmt.Sprintf(`printf '%s%%s\n' "$exitCode"`, exitCodePrefix),
		"exit 0",
	}, "\n")
}

// readRotateKeysResult extracts the embedded exit code from the applied one-time instruction
// output. Returns errRotateKeysOutputNotYet when the key or exit-code line is not yet written
// (callers should wait). Returns a non-sentinel error when the exit code is present but
// corrupt (callers should fail the operation).
func readRotateKeysResult(appliedOutput map[string][]byte) (commandResult, error) {
	raw, ok := appliedOutput[rotateKeysInstructionName]
	if !ok {
		return commandResult{}, errRotateKeysOutputNotYet
	}
	message := string(raw)
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, exitCodePrefix) {
			continue
		}
		exitCode, err := strconv.Atoi(strings.TrimPrefix(line, exitCodePrefix))
		if err != nil {
			return commandResult{}, fmt.Errorf("corrupt rotate-keys exit code in output: %w", err)
		}
		return commandResult{output: message, exitCode: exitCode}, nil
	}
	return commandResult{}, errRotateKeysOutputNotYet
}

// rotateKeysCommandTimedOut returns true when the rotate-keys output matches the known CLI
// timeout signature for the encrypt/config endpoint.
func rotateKeysCommandTimedOut(output string) bool {
	return commandTimedOut(output, rotateKeysTimeoutEndpoint)
}

// commandTimedOut returns true when the output contains a known CLI timeout signature
// for the given endpoint.
func commandTimedOut(output, endpoint string) bool {
	if output == "" || endpoint == "" {
		return false
	}
	if !strings.Contains(output, rotateKeysTimeoutMessage) || !strings.Contains(output, endpoint) {
		return false
	}
	for _, marker := range timeoutMarkers {
		if strings.Contains(output, marker) {
			return true
		}
	}
	return false
}

// convergenceWaitMessage checks whether the periodic secrets-encrypt status on secret has
// reached reencrypt_finished (and, when requireHashMatch is true, hash convergence).
//
// Return values:
//   - ("", nil)    — all criteria met, caller may advance
//   - (msg, nil)   — still waiting, msg describes the reason
//   - ("", err)    — hard error: corrupt payload, plan instruction missing, parse failure,
//     or hash field absent when requireHashMatch is true at reencrypt_finished
func convergenceWaitMessage(secret *corev1.Secret, requireHashMatch bool) (string, error) {
	periodicOutput, err := plan.ReadAppliedPeriodicOutput(secret)
	if err != nil {
		// gzip decode or JSON unmarshal failure — corrupt payload, not a normal wait.
		return "", fmt.Errorf("corrupt applied-periodic-output on %s: %w", secret.Name, err)
	}

	var entry plan.PeriodicInstructionOutput
	var entryOK bool
	if periodicOutput != nil {
		entry, entryOK = periodicOutput[statusPeriodicName]
	}

	if !entryOK {
		// No output yet, only a wait if the instruction is actually in the assigned plan.
		// Missing instruction means a malformed/regressed plan so fail immediately.
		if err := validatePlanHasPeriodicStatus(secret); err != nil {
			return "", fmt.Errorf("failed reading assigned plan from %s: %w", secret.Name, err)
		}
		return fmt.Sprintf("waiting for secrets-encrypt status on %s", secret.Name), nil
	}

	stdout := strings.TrimSpace(string(entry.Stdout))
	if stdout == "" {
		return fmt.Sprintf("waiting for secrets-encrypt status output on %s", secret.Name), nil
	}

	rotStatus, err := statusFromOutput(stdout)
	if err != nil {
		if errors.Is(err, errStatusTimeout) {
			// Transient CLI timeout: wait for the next periodic run.
			return fmt.Sprintf("secrets-encrypt status timed out on %s; waiting for next run", secret.Name), nil
		}
		// Durable parse failure (e.g. missing rotation stage): surface as a hard error
		// so the caller can fail the operation cleanly rather than looping indefinitely.
		return "", fmt.Errorf("unable to parse secrets-encrypt status on %s: %w", secret.Name, err)
	}

	if rotStatus.stage != stageReencryptFinished {
		return fmt.Sprintf("waiting for reencrypt_finished on %s, current stage: %s", secret.Name, rotStatus.stage), nil
	}
	if requireHashMatch {
		if !rotStatus.hashesPresent {
			// Hash field absent at reencrypt_finished: the runtime output does not satisfy the
			// convergence contract. Fail rather than waiting indefinitely.
			return "", fmt.Errorf("secrets-encrypt status on %s reached %s but hash field is absent; runtime output may be incompatible", secret.Name, stageReencryptFinished)
		}
		if !rotStatus.hashesMatch {
			return fmt.Sprintf("waiting for encryption key rotation hashes to converge on %s", secret.Name), nil
		}
	}
	return "", nil
}

// validatePlanHasPeriodicStatus reports whether the plan currently assigned to secret includes a
// periodic instruction named statusPeriodicName. Returns an error when the plan data is
// absent or not decodable.
func validatePlanHasPeriodicStatus(secret *corev1.Secret) error {
	raw := secret.Data["plan"]
	if len(raw) == 0 {
		return fmt.Errorf("plan data absent from %s", secret.Name)
	}
	var p plan.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("decoding plan from %s: %w", secret.Name, err)
	}
	for _, inst := range p.PeriodicInstructions {
		if inst.Name == statusPeriodicName {
			return nil
		}
	}
	return fmt.Errorf("periodic instruction %s not present in plan for %s", statusPeriodicName, secret.Name)
}

// statusFromOutput parses the human-readable secrets-encrypt status output.
// Returns errStatusTimeout for transient CLI timeouts.
// Returns a non-sentinel error when the rotation stage is missing.
func statusFromOutput(output string) (runtimeStatus, error) {
	// A timed-out status call is transient; wait for the next periodic run.
	if commandTimedOut(output, statusTimeoutEndpoint) {
		return runtimeStatus{}, fmt.Errorf("%w; waiting for next run", errStatusTimeout)
	}

	var result runtimeStatus
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Current Rotation Stage":
			result.stage = strings.TrimSpace(value)
		case "Server Encryption Hashes":
			result.hashesPresent = true
			result.hashesMatch = strings.TrimSpace(value) == hashesMatchMessage
		}
	}

	if result.stage == "" {
		return runtimeStatus{}, fmt.Errorf("unable to parse rotation stage from secrets-encrypt status output")
	}
	return result, nil
}
