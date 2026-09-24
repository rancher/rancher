package encryptionkeyrotation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

// OperationKind is this operation's kind, as it appears in the beacon claims the controller writes.
// See ops.BeaconOwnerKey.
const OperationKind = "EncryptionKeyRotation"

const Finalizer = "encryptionkeyrotation.operation.cattle.io"

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

// stepHookPrefixFor returns the step-hook label prefix for the given rotation step, or "" for an
// unknown / empty step. Used by handleInProgress to decide whether beacon-authorization loss is
// explained by an active step-scoped delegation vs a genuine loss.
func stepHookPrefixFor(step opv1alpha1.EncryptionKeyRotationStep) string {
	switch step {
	case opv1alpha1.EncryptionKeyRotationStepRotate:
		return RotateStepHookLabelPrefix
	case opv1alpha1.EncryptionKeyRotationStepRestart:
		return RestartStepHookLabelPrefix
	}
	return ""
}

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

// OnChange is the status handler entrypoint invoked by the wrangler-registered controller, and the
// whole of one reconcile. It decides whether the operation should be reconciled at all, and if so
// hands it to whichever of the two drivers applies:
//
//   - paused: nothing is reconciled, in flight or deleting. Only the conditions are refreshed, so
//     the operation reports that it is paused and otherwise stands still.
//   - deleting: reconcileDeleting cancels an operation the deletion caught in flight, runs its
//     terminal handler, and retires the finalizer once that handling is complete.
//   - otherwise: reconcileActive advances the operation through its phases and, once it stops
//     moving, either garbage collects it or schedules the next poll.
func (h *handler) OnChange(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	if op == nil {
		return status, nil
	}

	// Pausing an operation stops the controller touching it at all: no plans are dispatched, no
	// beacon is acquired or released, and no finalizer is taken. That holds for a deleting operation
	// too — releasing its beacon is reconciliation like any other — so an operation already carrying
	// the finalizer when it was paused will not finish deleting until it is resumed.
	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: skipping paused operation", op.Namespace, op.Name)

		return updateStatus(op, status), nil
	}

	if op.DeletionTimestamp != nil {
		return h.reconcileDeleting(op, status)
	}

	return h.reconcileActive(op, status)
}

// reconcileActive drives an operation which is neither paused nor being deleted: it takes the
// finalizer, advances the operation one step, and then either garbage collects it or arranges to
// look again.
func (h *handler) reconcileActive(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	// The finalizer is what guarantees the controller observes the deletion of an operation which is
	// still in flight, so it can cancel it and release the beacon. A paused operation is turned away
	// above before reaching this: it has dispatched nothing since it was paused, and taking the
	// finalizer would only wedge its deletion until it is resumed.
	if err := h.ensureFinalizer(op); err != nil {
		return status, err
	}

	// An operation which has not been reconciled before starts out Pending.
	if status.Phase == "" {
		status.SetPhase(opv1alpha1.OperationPhasePending)
	}

	if previous, canceled := ops.CancelForRequest(&op.Spec.OperationSpec, &status.OperationStatus); canceled {
		logrus.Infof("[encryptionkeyrotation] %s/%s: marking operation as canceled: cancellation requested in phase [%s] step [%s]", op.Namespace, op.Name, previous, status.Step)
	}

	status, err := h.advance(op, status)
	if err != nil {
		return status, err
	}

	status = updateStatus(op, status)

	if !equality.Semantic.DeepEqual(op.Status, status) {
		// State moved this tick. The status handler writes it out, and that update re-enqueues the
		// operation, so there is nothing to schedule here.
		return status, nil
	}

	if ops.Collectable(op, &op.Spec.OperationSpec, &status.OperationStatus) {
		if err := h.encryptionkeyrotations.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{}); err != nil {
			return status, err
		}

		// The operation is on its way out, so the status computed for it is moot.
		return status, generic.ErrSkip
	}

	// Nothing moved, so poll: plan secret state, beacon transitions and the TTL falling due are all
	// changes this controller will not otherwise be told about.
	h.encryptionkeyrotations.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)

	return status, nil
}

// reconcileDeleting drives an operation which is being deleted. The deletion is held up by our
// finalizer until terminal handling has been recorded as complete, which keeps the operation — and
// with it any beacon delegation made on its behalf — alive while a terminal phase hook delegate
// finishes its work. The terminal status is also persisted before the finalizer is dropped, so an
// observer waiting on the final phase gets to see it rather than the object simply vanishing.
func (h *handler) reconcileDeleting(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	// Teardown is driven off our finalizer. Without it the operation has either already been torn
	// down or was never ours to begin with, and is on its way out under someone else's control.
	if !hasFinalizer(op) {
		return updateStatus(op, status), nil
	}

	// cancelForDeletion always leaves a phase behind, so unlike reconcileActive there is no unset
	// phase to default here.
	if previous, canceled := ops.CancelForDeletion(&status.OperationStatus); canceled {
		logrus.Infof("[encryptionkeyrotation] %s/%s: marking operation as canceled: deleted in phase [%s] step [%s] before terminal handling completed", op.Namespace, op.Name, previous, status.Step)
	}

	status, err := h.advance(op, status)
	if err != nil {
		return status, err
	}

	status = updateStatus(op, status)

	if !equality.Semantic.DeepEqual(op.Status, status) {
		// State moved this tick: let the status handler write it out. The resulting update
		// re-enqueues the operation, and the next pass retires the finalizer.
		return status, nil
	}

	if !ops.IsTerminated(&status.OperationStatus) {
		// Terminal handling is still in flight — typically a canceled phase hook whose delegate has
		// yet to hand the beacon back. Keep the finalizer and poll for it to finish.
		logrus.Debugf("[encryptionkeyrotation] %s/%s: deferring deletion, terminal handling has not completed", op.Namespace, op.Name)
		h.encryptionkeyrotations.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)

		return status, nil
	}

	logrus.Infof("[encryptionkeyrotation] %s/%s: terminal handling complete, releasing operation for deletion", op.Namespace, op.Name)

	return status, h.removeFinalizer(op)
}

// advance resolves everything the phase handlers work from and runs the handler for the operation's
// current phase.
//
// A nil scope from resolveScope means the reconcile has already settled for this tick — the cluster
// or the beacon is gone, or a deleting operation has nothing left to release — and the status it
// returned is what should be reported.
func (h *handler) advance(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	s, status, err := h.resolveScope(op, status)
	if err != nil || s == nil {
		return status, err
	}

	return h.dispatchPhase(s, status)
}

// hasFinalizer reports whether the operation still carries our finalizer, i.e. whether its teardown
// is ours to drive.
func hasFinalizer(op *opv1alpha1.EncryptionKeyRotation) bool {
	return slices.Contains(op.Finalizers, Finalizer)
}

// ensureFinalizer adds our finalizer to the operation if it is not already present. The status
// handler only ever persists status, so the finalizer has to be written with an explicit Update;
// the updated object is copied back over op so the resource version the status handler goes on to
// use for its own UpdateStatus is not stale.
func (h *handler) ensureFinalizer(op *opv1alpha1.EncryptionKeyRotation) error {
	if hasFinalizer(op) {
		return nil
	}

	logrus.Debugf("[encryptionkeyrotation] %s/%s: adding finalizer", op.Namespace, op.Name)

	updated := op.DeepCopy()
	updated.Finalizers = append(updated.Finalizers, Finalizer)

	updated, err := h.encryptionkeyrotations.Update(updated)
	if err != nil {
		return err
	}

	*op = *updated

	return nil
}

// removeFinalizer drops our finalizer from the operation, which lets the API server complete the
// deletion. A NotFound is treated as success: something else (another finalizer holder finishing
// last, or a previous attempt whose response was lost) already let the object go.
func (h *handler) removeFinalizer(op *opv1alpha1.EncryptionKeyRotation) error {
	if !hasFinalizer(op) {
		return nil
	}

	updated := op.DeepCopy()
	updated.Finalizers = slices.DeleteFunc(updated.Finalizers, func(f string) bool {
		return f == Finalizer
	})

	updated, err := h.encryptionkeyrotations.Update(updated)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	*op = *updated

	return nil
}

// resolveScope gathers everything the phase handlers work from: the parent cluster, the Adapter for
// its kind, and the cluster's beacon.
//
// A nil scope returned with a nil error means the reconcile has settled for this tick and the
// returned status is what should be reported — the cluster is missing, the beacon has not been
// created yet, or the operation is deleting and has nothing left to release.
func (h *handler) resolveScope(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) (*scope, opv1alpha1.EncryptionKeyRotationStatus, error) {
	deleting := op.DeletionTimestamp != nil

	gvk := schema.FromAPIVersionAndKind(op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
	ref, err := h.dynamic.Get(gvk, op.Spec.ClusterRef.Namespace, op.Spec.ClusterRef.Name)
	if apierrors.IsNotFound(err) {
		key := opv1alpha1.ClusterRefKey(op.Spec.ClusterRef)

		// The beacon lives alongside the cluster, so a deleted operation whose cluster is gone has
		// nothing left to release or unpause: terminal handling is trivially complete and the
		// operation is free to finish deleting. Failing it here instead would both overwrite the
		// Canceled phase and, for a cluster deleted mid-operation, wedge the deletion behind our
		// finalizer.
		if deleting {
			logrus.Infof("[encryptionkeyrotation] %s/%s: cluster %s is gone, nothing to release", op.Namespace, op.Name, key)
			status.SetTerminated()
			return nil, status, nil
		}

		logrus.Errorf("[encryptionkeyrotation]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

		status.MarkFailed(opv1alpha1.ClusterNotFoundReason, fmt.Sprintf("cluster %s not found", key))

		// This failure is terminated here rather than by handleFailed: the beacon is resolved
		// through the cluster's adapter, so with no cluster there is no beacon to release and every
		// subsequent reconcile would return from this branch without ever reaching a terminal
		// handler — leaving the operation ineligible for TTL garbage collection forever.
		status.SetTerminated()

		return nil, status, nil
	}
	if err != nil {
		return nil, status, err
	}

	ustrMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ref)
	if err != nil {
		return nil, status, err
	}

	ustr := unstructured.Unstructured{Object: ustrMap}

	adapter, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return nil, status, err
	}

	clusterObj, err := adapter.ClusterObject()
	if err != nil {
		return nil, status, err
	}

	// Resolve the beacon via the adapter, not op.Spec.ClusterRef. See
	// etcdsnapshotrestore/controller.go for the full rationale.
	namespace, beaconName := adapter.BeaconRef()

	beacon, err := h.beacons.Get(namespace, beaconName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && deleting {
		// As above: no beacon means nothing to release, so let the deletion proceed rather than
		// requeueing a NotFound forever.
		logrus.Infof("[encryptionkeyrotation] %s/%s: beacon %s/%s is gone, nothing to release", op.Namespace, op.Name, namespace, beaconName)
		status.SetTerminated()
		return nil, status, nil
	} else if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		logrus.Warnf("[encryptionkeyrotation]: %s/%s failed to find beacon %s/%s (clusterRef apiVersion=%s kind=%s name=%s)",
			op.Namespace, op.Name, namespace, beaconName, ustr.GetAPIVersion(), ustr.GetKind(), ustr.GetName())

		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")

		return nil, status, nil
	} else if err != nil {
		return nil, status, err
	}

	return &scope{
		ownerKey:   ops.BeaconOwnerKey(OperationKind, op),
		op:         op,
		beacon:     beacon,
		namespace:  namespace,
		clusterObj: clusterObj,
		adapter:    adapter,
	}, status, nil
}

// dispatchPhase routes the operation to the handler for its current phase. An unrecognized phase is
// itself terminal: the controller cannot know what the operation was doing, so it fails it. This
// should be prevented by validation, but is handled just in case.
func (h *handler) dispatchPhase(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	switch status.Phase {
	case opv1alpha1.OperationPhasePending:
		return h.handlePending(s, status)
	case opv1alpha1.OperationPhaseInProgress:
		return h.handleInProgress(s, status)
	case opv1alpha1.OperationPhaseAborted:
		return h.handleAborted(s, status)
	case opv1alpha1.OperationPhaseCanceled:
		return h.handleCanceled(s, status)
	case opv1alpha1.OperationPhaseFailed:
		return h.handleFailed(s, status)
	case opv1alpha1.OperationPhaseSucceeded:
		return h.handleSucceeded(s, status)
	}

	status.MarkFailed(opv1alpha1.UnknownPhaseReason, fmt.Sprintf("unknown phase [%s]", s.op.Status.Phase))

	return status, nil
}

type scope struct {
	// ownerKey is the per-operation beacon owner key, derived from the operation UID. Unlike the
	// other operation controllers, encryption key rotation never shares one key across operations.
	ownerKey string

	op        *opv1alpha1.EncryptionKeyRotation
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

// handleHook pushes the delegate named by the operation's hook label for prefix onto the beacon, and
// reports whether there was one — in which case the caller stops where it is and waits.
func (h *handler) handleHook(s *scope, prefix string) (bool, error) {
	logrus.Tracef("[encryptionkeyrotation] %s/%s: checking lifecycle hook for prefix %q", s.op.Namespace, s.op.Name, prefix)

	delegated, beacon, err := ops.DelegateForHook(s.op, s.beacon, h.beacons, prefix)
	s.beacon = beacon

	return delegated, err
}
func (h *handler) handlePending(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	// A beacon still carrying a claim from an earlier incarnation of this operation's name is
	// reclaimed before anything is attempted: that claim is provably dead, and leaving it would
	// either block this operation forever or, worse, be mistaken for its own.
	beacon, err := ops.ReclaimSupersededBeacon(s.beacon, h.beacons, s.ownerKey)
	if err != nil {
		return status, err
	}
	s.beacon = beacon

	// Pending waits until this op is either the primary owner OR anywhere in the delegate chain.
	// If we're already in the chain, the primary owner is driving the beacon on our behalf — skip
	// AcquireBeacon entirely and continue with hook + WaitForRegister. Otherwise, attempt to acquire;
	// a nil return means another controller currently owns it, and we must keep waiting.
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

	// Pending-phase hook fires after beacon acquisition so a delegate can inspect the recorded
	// ownership before the controller starts driving the rotation.
	delegated, err := h.handleHook(s, opv1alpha1.PendingPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		ops.SetWaitingForDelegate(opv1alpha1.PendingCondition, &status.OperationStatus, s.beacon)
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

	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.EncryptionKeyRotationStepRotate)

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

func (h *handler) handleInProgress(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	stepPrefix := stepHookPrefixFor(s.op.Status.Step)

	// Stage 1 (loose): the op must appear SOMEWHERE in the ownership chain (owner or any
	// delegate). Being absent entirely means the beacon was reassigned to another controller, and
	// we can't recover. If a step hook is currently active on the op, treat the absence as a
	// step-scoped delegation and surface WaitingForDelegate instead of failing — the delegate may
	// have popped us in service of the hook and will restore ownership when the hook clears.
	if !plan.IsOwningBeaconHolder(s.beacon, s.ownerKey) && !plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		if ops.HasStepHookLabel(s.op, stepPrefix) {
			ops.SetWaitingForDelegate(opv1alpha1.InProgressCondition, &status.OperationStatus, s.beacon)
			return status, nil
		}
		status.MarkFailed(opv1alpha1.BeaconLostReason, "beacon reassigned, aborting")

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
	delegated, err := h.handleHook(s, opv1alpha1.InProgressPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		ops.SetWaitingForDelegate(opv1alpha1.InProgressCondition, &status.OperationStatus, s.beacon)
		return status, nil
	}

	// Stage 2 (strict): after the InProgress-phase hook has been handled, the op must be the
	// primary owner or the most-recent delegate on the chain to drive step work. If a step hook
	// is still active on the op, treat the missing-top state as an intentional delegation and
	// wait; otherwise this is a genuine beacon loss and we fail.
	if !plan.AuthorizedForBeacon(s.beacon, s.ownerKey) {
		if ops.HasStepHookLabel(s.op, stepPrefix) {
			ops.SetWaitingForDelegate(opv1alpha1.InProgressCondition, &status.OperationStatus, s.beacon)
			return status, nil
		}
		status.MarkFailed(opv1alpha1.BeaconLostReason, "beacon acquired by another controller, aborting")

		return status, nil
	}

	switch s.op.Status.Step {
	case opv1alpha1.EncryptionKeyRotationStepRotate:
		return h.reconcileRotate(s, status)
	case opv1alpha1.EncryptionKeyRotationStepRestart:
		return h.reconcileRestart(s, status)
	}

	status.MarkFailed(opv1alpha1.UnknownStepReason, fmt.Sprintf("current step [%q] is unknown, expected one of: [%q, %q]",
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
		ops.SetWaitingForDelegate(opv1alpha1.InProgressCondition, &status.OperationStatus, s.beacon)
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

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)
	runtime := s.adapter.RuntimeCommand()

	nodePlan := &plan.Plan{
		OneTimeInstructions: []plan.OneTimeInstruction{
			// 1. Run rotate-keys via wrapper that always exits 0; captures real exit code in output.
			{
				Name:       rotateKeysInstructionName,
				Command:    "/bin/sh",
				Args:       []string{"-c", rotateKeysScript(runtime)},
				SaveOutput: true,
			},
			// 2. Poll until secrets-encrypt status responds; gates planStatus.Applied until
			// the encryption server is reachable after key reload.
			{
				Name:    waitForStatusInstructionName,
				Command: "/bin/sh",
				Args:    []string{"-c", waitForStatusScript(runtime)},
			},
			// 3. One-time status snapshot captured when the plan is applied; provides an
			// observability anchor and confirms the endpoint is stable.
			{
				Name:       statusPeriodicName,
				Command:    runtime,
				Args:       []string{"secrets-encrypt", "status"},
				SaveOutput: true,
			},
		},
		PeriodicInstructions: []plan.PeriodicInstruction{
			// Runs every 5s independently; used for stage/hash convergence checking.
			{
				Name:          statusPeriodicName,
				Command:       runtime,
				Args:          []string{"secrets-encrypt", "status"},
				PeriodSeconds: 5,
			},
		},
		Probes: probes,
	}

	// Use finite failure threshold so a plan that can't execute
	// is marked Failed rather than retried forever. The wrapper always exits 0, so a
	// real apply failure here means the wrapper itself couldn't run.
	planStatus, err := h.store.AssignPlan(leader, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: rotate-keys plan failed to execute on leader %s", s.op.Namespace, s.op.Name, leader.Name)
		status.MarkFailed(opv1alpha1.PlanFailedReason, fmt.Sprintf("encryption key rotation plan failed for leader %s/%s", leader.Namespace, leader.Name))
		return status, nil
	}

	if planStatus.Waiting() {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for rotate-keys plan for %s/%s", s.op.Namespace, s.op.Name, leader.Namespace, leader.Name)

		ops.SetWaitingForSinglePlan(&status.OperationStatus, planStatus)

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
		status.MarkFailed(opv1alpha1.PlanFailedReason, fmt.Sprintf("corrupt rotate-keys output on leader %s", leader.Name))
		return status, nil
	}

	if result.exitCode != 0 {
		if rotateKeysCommandTimedOut(result.output) {
			// CLI timed out but rotation may still be running in the background;
			// keep watching periodic status before deciding.
			logrus.Warnf("[encryptionkeyrotation] %s/%s: rotate-keys CLI timed out on leader %s; continuing to observe periodic status", s.op.Namespace, s.op.Name, leader.Name)
		} else {
			logrus.Errorf("[encryptionkeyrotation] %s/%s: rotate-keys failed on leader %s with exit code %d", s.op.Namespace, s.op.Name, leader.Name, result.exitCode)
			status.MarkFailed(opv1alpha1.PlanFailedReason, fmt.Sprintf("secrets-encrypt rotate-keys failed on leader %s (exit code %d); please perform an etcd restore", leader.Name, result.exitCode))
			return status, nil
		}
	}

	// Check periodic secrets-encrypt status. Stay in Rotate until reencrypt_finished.
	waitMsg, err := convergenceWaitMessage(leader, false)
	if err != nil {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: convergence check failed on leader %s: %v", s.op.Namespace, s.op.Name, leader.Name, err)
		status.MarkFailed(opv1alpha1.PlanFailedReason, fmt.Sprintf("corrupt encryption key rotation state on leader %s; please perform an etcd restore", leader.Name))
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
	status.SetStep(opv1alpha1.EncryptionKeyRotationStepRestart)
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
		ops.SetWaitingForDelegate(opv1alpha1.InProgressCondition, &status.OperationStatus, s.beacon)
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
		status.MarkFailed(opv1alpha1.UnknownStepReason, "no control-plane nodes found; cannot verify post-restart encryption status")
		return status, nil
	}
	if !ops.IsControlPlane(secrets[len(secrets)-1]) {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: nodes are not correctly ordered at restart step", s.op.Namespace, s.op.Name)
		status.MarkFailed(opv1alpha1.UnknownStepReason, "last control plane node not found; cannot verify hash convergence after restart")
		return status, nil
	}

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)
	serverUnit := s.adapter.ServerUnit()
	runtime := s.adapter.RuntimeCommand()

	// pool is ordered with plan.DefaultSorter(), so the final element is the
	// last control-plane node. For the new k3s rotate-keys flow, strict "All
	// hashes match" validation only makes sense after every control-plane node
	// has restarted; before that, k3s may legitimately report
	// reencrypt_finished while hashes still differ across servers.
	for i, secret := range secrets {
		requireHashMatch := i == len(secrets)-1
		status, done, err := h.reconcileRestartNode(s, status, secret, opEnv, serverUnit, runtime, requireHashMatch)
		if err != nil {
			return status, err
		}
		if !done {
			return status, nil
		}
	}

	return h.finishRotation(s, status)
}

// finishRotation hands the cluster back to the planner and records the rotation as successful. Both
// belong to the same moment and are done together so that neither can happen without the other.
//
// It is reached only from the end of reconcileRestart, once every server node has restarted and
// converged, which makes it the one place a rotation unpauses. A rotation which does not get this
// far has left the cluster mid-rotation, with some nodes on the new key and some on the old, and
// letting the planner roll nodes in that state would be unsafe — so it stays paused, deliberately,
// for an administrator to resolve. That is what its failure messages already ask for, and it is why
// terminal handling does not unpause either.
func (h *handler) finishRotation(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	// Unpause before asserting the outcome: a rotation reported successful while its cluster is
	// still paused would look finished and leave the cluster frozen.
	if err := s.adapter.PauseCluster(false); err != nil {
		return status, err
	}

	logrus.Infof("[encryptionkeyrotation] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.MarkSucceeded()

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
	opEnv []string,
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
			Name:    "restart",
			Command: "systemctl",
			Args:    []string{"restart", serverUnit},
		},
		{
			Name:    "wait-for-systemctl-status",
			Command: "/bin/sh",
			Args:    []string{"-c", waitForSystemctlStatusScript(serverUnit)},
		},
	}

	nodePlan := &plan.Plan{
		OneTimeInstructions: oneTimeInstructions,
		Probes:              probes,
	}
	if ops.IsControlPlane(secret) {
		nodePlan.OneTimeInstructions = append(nodePlan.OneTimeInstructions,
			plan.OneTimeInstruction{
				Name:    waitForStatusInstructionName,
				Command: "/bin/sh",
				Args:    []string{"-c", waitForStatusScript(runtime)},
			},
			plan.OneTimeInstruction{
				Name:       statusPeriodicName,
				Command:    runtime,
				Args:       []string{"secrets-encrypt", "status"},
				SaveOutput: true,
			},
		)
		nodePlan.PeriodicInstructions = []plan.PeriodicInstruction{
			{
				Name:          statusPeriodicName,
				Command:       runtime,
				Args:          []string{"secrets-encrypt", "status"},
				PeriodSeconds: 5,
			},
		}
	}

	planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, opEnv), 5, 5)
	if err != nil {
		return status, false, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: restart plan failed for %s", s.op.Namespace, s.op.Name, secret.Name)
		status.MarkFailed(opv1alpha1.PlanFailedReason, fmt.Sprintf("restart failed for %s; please perform an etcd restore", secret.Name))
		return status, false, nil
	}

	if planStatus.Waiting() {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for restart for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)
		ops.SetWaitingForSinglePlan(&status.OperationStatus, planStatus)
		return status, false, nil
	}

	if ops.IsControlPlane(secret) {
		waitMsg, err := convergenceWaitMessage(secret, requireHashMatch)
		if err != nil {
			logrus.Errorf("[encryptionkeyrotation] %s/%s: convergence check failed on %s: %v", s.op.Namespace, s.op.Name, secret.Name, err)
			status.MarkFailed(opv1alpha1.PlanFailedReason, fmt.Sprintf("corrupt encryption key rotation state on %s; please perform an etcd restore", secret.Name))
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

// terminalPhase describes what is specific to one terminal phase: the condition it reports through,
// the lifecycle hook that can defer its completion, and any work to run once the beacon is back.
type terminalPhase struct {
	hook string

	// beaconOptional marks a phase an operation can reach without holding the beacon, which is
	// every outcome but success:
	//
	//   - Failed, which handleInProgress reaches precisely because the beacon was lost;
	//   - Aborted, where the operation called its own work off and may since have been overtaken;
	//   - Canceled, driven from outside the operation and often by whoever wants the beacon next.
	//
	// For those a missing claim is an expected outcome rather than a failure, so the phase's hook
	// is passed over — without a claim there is no authority to delegate — and the beacon is left
	// untouched. Succeeded is deliberately not one of them: an operation cannot have finished its
	// work without holding the beacon throughout, so a missing claim there is an anomaly rather
	// than a state to paper over.
	beaconOptional bool

	// onRelease, when set, runs after the beacon has been released. owning reports whether this
	// operation was the beacon's primary owner rather than a delegate acting on its behalf.
	onRelease func(s *scope, owning bool)
}

// handleTerminal is the shared body of every terminal phase handler, and the only place an
// operation is recorded as terminated.
//
// Reaching a terminal phase is not the end of the operation's handling: the phase's lifecycle hook
// may hand the beacon to a delegate first, and the beacon has to be released afterward so the next
// operation in line can acquire it. Recording termination in this one place — after the hook is
// satisfied, after the release succeeded — is what keeps the marker honest, since that marker is
// what makes the operation eligible for TTL collection and lets a deleted operation finish
// deleting. A terminal phase handler that returns early therefore cannot forget to withhold it.
//
// Unpausing the cluster is deliberately not part of this. Only a rotation which restarted every
// node has left the cluster in a state fit to hand back to the planner, so reconcileRestart does it
// and no terminal path undoes it — see there.
//
// An operation which no longer holds the beacon has none of that left to do: see
// beaconOptional. It terminates without the beacon being written to at all, which is what
// keeps an operation that lost its claim from reaching into whichever one holds it now.
func (h *handler) handleTerminal(s *scope, status opv1alpha1.EncryptionKeyRotationStatus, phase terminalPhase) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation %s", s.op.Namespace, s.op.Name, status.Phase)

	// A phase whose beacon claim is optional passes over its hook once that claim is gone: there is
	// no authority left to delegate, and pushing a delegate onto a beacon another controller now
	// holds would be reaching into its operation. Everything after the hook is either a no-op
	// without a claim (releaseBeacon) or owed regardless of one, so the operation still terminates
	// — which is what lets it be collected, or lets a deleted one retire its finalizer.
	honorHook := !phase.beaconOptional || plan.HoldsBeacon(s.beacon, s.ownerKey)

	if honorHook {
		delegated, err := h.handleHook(s, phase.hook)
		if err != nil {
			return status, err
		} else if delegated {
			// The delegate drives the beacon on this operation's behalf from here, and the cluster stays
			// paused for it. Nothing is written to the outcome condition: it already reports the outcome
			// with the reason the phase handler gave it, and that reason must survive the delegation.
			// updateStatus reports the delegate on Finalized for as long as the hook label is present,
			// so the wait resolves on its own once the delegate clears the label rather than being left
			// behind on a condition.
			return status, nil
		}
	} else {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: %s with no claim on the beacon, leaving it untouched", s.op.Namespace, s.op.Name, status.Phase)
	}

	owning, err := plan.ReleaseBeaconIfHeld(s.beacon, h.beacons, s.ownerKey)
	if err != nil {
		return status, err
	}

	if phase.onRelease != nil {
		phase.onRelease(s, owning)
	}

	status.SetTerminated()

	return status, nil
}

// handleAborted handles the Aborted terminal phase, reached when the operation called its own work
// off rather than attempting it and losing — which is what separates it from Failed. Its phase hook
// runs first so a delegate can observe why the operation stopped.
func (h *handler) handleAborted(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           opv1alpha1.AbortedPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleCanceled handles the Canceled terminal phase, which is reached when an external controller
// cancels the operation or it is deleted before its terminal handling completed. Its phase hook
// runs first so a delegate can observe the cancellation. What separates cancellation from the other
// outcomes is that it comes from outside the operation, where Failed means the work was attempted
// and lost and Aborted means the operation called it off itself — none implies another.
func (h *handler) handleCanceled(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           opv1alpha1.CanceledPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleFailed handles the Failed terminal phase. Its hook fires before cleanup so a delegate can
// inspect failure state (status conditions, plan-secret applied output, residual rotate-keys
// process) before the cluster is unpaused and the beacon released.
func (h *handler) handleFailed(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           opv1alpha1.FailedPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleSucceeded handles the Succeeded terminal phase. Its hook fires before unpausing and
// releasing — delegates use this to chain follow-up work (e.g. a verifier that re-runs
// `secrets-encrypt status` from outside the operation) before the cluster goes back to accepting
// new operations. On the owner path it then re-enqueues the backing cluster so downstream
// controllers observe the final beacon transition; only the owner does so, since only the owner
// terminating implies downstream work.
func (h *handler) handleSucceeded(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook: opv1alpha1.SucceededPhaseHookLabelPrefix,
		onRelease: func(s *scope, owning bool) {
			if !owning {
				return
			}
			// enqueue original object to ensure it is processed by requisite controllers
			gvk := schema.FromAPIVersionAndKind(s.clusterObj.GetAPIVersion(), s.clusterObj.GetKind())
			_ = h.dynamic.Enqueue(gvk, s.clusterObj.GetNamespace(), s.clusterObj.GetName())
		},
	})
}

// updateStatus refreshes ObservedGeneration and every condition that is not the one the current
// phase handler owns. Every operation type reports its progress identically, so the work itself is
// shared — see ops.UpdateStatus.
func updateStatus(op *opv1alpha1.EncryptionKeyRotation, status opv1alpha1.EncryptionKeyRotationStatus) opv1alpha1.EncryptionKeyRotationStatus {
	logrus.Tracef("[encryptionkeyrotation] %s/%s: updating conditions", op.Namespace, op.Name)

	ops.UpdateStatus(op, &op.Spec.OperationSpec, &status.OperationStatus)

	return status
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
	for line := range strings.SplitSeq(message, "\n") {
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
	for line := range strings.SplitSeq(output, "\n") {
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
