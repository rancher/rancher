package certificaterotation

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
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

// ControllerOwnerKey is the shared operation-type key for certificate rotation coordination.
const ControllerOwnerKey = "certificate-rotation"

// OperationKind is this operation's kind, as it appears in the beacon claims the controller writes.
// See ops.BeaconOwnerKey.
const OperationKind = "CertificateRotation"

const Finalizer = "certificaterotation.operation.cattle.io"

// RotateStepHookLabelPrefix gates the Rotate step, before reconcileRotate pauses the cluster and
// assigns the rotation plan to each node in turn. It fires before PauseCluster so a delegate
// observes the cluster in its pre-pause state.
const RotateStepHookLabelPrefix = "rotate.step.hook.operation.cattle.io/"

// stepHookPrefixFor returns the step-hook label prefix for the given rotation step, or "" for an
// unknown / empty step. Used by handleInProgress to decide whether beacon-authorization loss is
// explained by an active step-scoped delegation vs a genuine loss.
func stepHookPrefixFor(step opv1alpha1.CertificateRotationStep) string {
	if step == opv1alpha1.CertificateRotationStepRotate {
		return RotateStepHookLabelPrefix
	}
	return ""
}

// dynamicResolver is the subset of *dynamic.Controller this handler needs: Get for resolving
// cluster refs and Enqueue for nudging the backing cluster after terminal beacon transitions.
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

// scope bundles the per-reconcile values derived from the operation, parent cluster, and beacon. It
// is built fresh on every invocation and threaded through the phase handlers so they don't each
// have to re-derive the same data.
type scope struct {
	ownerKey string

	op        *opv1alpha1.CertificateRotation
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

// Register wires the CertificateRotation status handler.
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
func (h *handler) OnChange(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	if op == nil {
		return status, nil
	}

	// Pausing an operation stops the controller touching it at all: no plans are dispatched, no
	// beacon is acquired or released, and no finalizer is taken. That holds for a deleting operation
	// too — releasing its beacon is reconciliation like any other — so an operation already carrying
	// the finalizer when it was paused will not finish deleting until it is resumed.
	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[certificaterotation] %s/%s: skipping paused operation", op.Namespace, op.Name)

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
func (h *handler) reconcileActive(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
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
		logrus.Infof("[certificaterotation] %s/%s: marking operation as canceled: cancellation requested in phase [%s] step [%s]", op.Namespace, op.Name, previous, status.Step)
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
		if err := h.certificateRotations.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{}); err != nil {
			return status, err
		}

		// The operation is on its way out, so the status computed for it is moot.
		return status, generic.ErrSkip
	}

	// Nothing moved, so poll: plan secret state, beacon transitions and the TTL falling due are all
	// changes this controller will not otherwise be told about.
	h.certificateRotations.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)

	return status, nil
}

// reconcileDeleting drives an operation which is being deleted. The deletion is held up by our
// finalizer until terminal handling has been recorded as complete, which keeps the operation — and
// with it any beacon delegation made on its behalf — alive while a terminal phase hook delegate
// finishes its work. The terminal status is also persisted before the finalizer is dropped, so an
// observer waiting on the final phase gets to see it rather than the object simply vanishing.
func (h *handler) reconcileDeleting(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	// Teardown is driven off our finalizer. Without it the operation has either already been torn
	// down or was never ours to begin with, and is on its way out under someone else's control.
	if !hasFinalizer(op) {
		return updateStatus(op, status), nil
	}

	// CancelForDeletion always leaves a phase behind, so unlike reconcileActive there is no unset
	// phase to default here.
	if previous, canceled := ops.CancelForDeletion(&status.OperationStatus); canceled {
		logrus.Infof("[certificaterotation] %s/%s: marking operation as canceled: deleted in phase [%s] step [%s] before terminal handling completed", op.Namespace, op.Name, previous, status.Step)
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
		logrus.Debugf("[certificaterotation] %s/%s: deferring deletion, terminal handling has not completed", op.Namespace, op.Name)
		h.certificateRotations.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)

		return status, nil
	}

	logrus.Infof("[certificaterotation] %s/%s: terminal handling complete, releasing operation for deletion", op.Namespace, op.Name)

	return status, h.removeFinalizer(op)
}

// advance resolves everything the phase handlers work from and runs the handler for the operation's
// current phase.
//
// A nil scope from resolveScope means the reconcile has already settled for this tick — the cluster
// or the beacon is gone, or a deleting operation has nothing left to release — and the status it
// returned is what should be reported.
func (h *handler) advance(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	s, status, err := h.resolveScope(op, status)
	if err != nil || s == nil {
		return status, err
	}

	return h.dispatchPhase(s, status)
}

// hasFinalizer reports whether the operation still carries our finalizer, i.e. whether its teardown
// is ours to drive.
func hasFinalizer(op *opv1alpha1.CertificateRotation) bool {
	return slices.Contains(op.Finalizers, Finalizer)
}

// ensureFinalizer adds our finalizer to the operation if it is not already present. The status
// handler only ever persists status, so the finalizer has to be written with an explicit Update;
// the updated object is copied back over op so the resource version the status handler goes on to
// use for its own UpdateStatus is not stale.
func (h *handler) ensureFinalizer(op *opv1alpha1.CertificateRotation) error {
	if hasFinalizer(op) {
		return nil
	}

	logrus.Debugf("[certificaterotation] %s/%s: adding finalizer", op.Namespace, op.Name)

	updated := op.DeepCopy()
	updated.Finalizers = append(updated.Finalizers, Finalizer)

	updated, err := h.certificateRotations.Update(updated)
	if err != nil {
		return err
	}

	*op = *updated

	return nil
}

// removeFinalizer drops our finalizer from the operation, which lets the API server complete the
// deletion. A NotFound is treated as success: something else (another finalizer holder finishing
// last, or a previous attempt whose response was lost) already let the object go.
func (h *handler) removeFinalizer(op *opv1alpha1.CertificateRotation) error {
	if !hasFinalizer(op) {
		return nil
	}

	updated := op.DeepCopy()
	updated.Finalizers = slices.DeleteFunc(updated.Finalizers, func(f string) bool {
		return f == Finalizer
	})

	updated, err := h.certificateRotations.Update(updated)
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
func (h *handler) resolveScope(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) (*scope, opv1alpha1.CertificateRotationStatus, error) {
	deleting := op.DeletionTimestamp != nil

	gvk := schema.FromAPIVersionAndKind(op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
	ref, err := h.dynamic.Get(gvk, op.Spec.ClusterRef.Namespace, op.Spec.ClusterRef.Name)
	if apierrors.IsNotFound(err) {
		key := opv1alpha1.ClusterRefKey(op.Spec.ClusterRef)

		// A missing cluster is only a failure for an operation which still has work to dispatch. One
		// which is deleting, or which has already concluded, has none: with no cluster there is no
		// adapter, and so no way to reach a beacon to release. Failing it here would overwrite the
		// outcome the operation ended with — the phase a cancellation just recorded, or a success
		// from an earlier reconcile — and, for a cluster deleted mid-operation, wedge the deletion
		// behind our finalizer.
		if deleting || ops.IsTerminal(status.Phase) {
			logrus.Infof("[certificaterotation] %s/%s: cluster %s is gone, nothing to release", op.Namespace, op.Name, key)

			// A terminal phase hook may still be owed, in which case the operation is left
			// un-terminated: its delegate has not had its turn, and recording termination would
			// claim that it had. UpdateStatus reports that wait, and deleting the operation is the
			// remedy if the delegate never clears its label.
			ops.TerminateUnlessHookOwed(op, &status.OperationStatus)

			return nil, status, nil
		}

		logrus.Errorf("[certificaterotation]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

		status.MarkFailed(opv1alpha1.ClusterNotFoundReason, fmt.Sprintf("cluster %s not found", key))

		// This failure is terminated here rather than by handleFailed: the beacon is resolved
		// through the cluster's adapter, so with no cluster there is no beacon to release and every
		// subsequent reconcile would return from this branch without ever reaching a terminal
		// handler, leaving the operation ineligible for TTL garbage collection forever.
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
	if apierrors.IsNotFound(err) {
		switch {
		// Nothing is owed on the beacon. A deleting operation is discarded along with its hooks;
		// an Aborted one called its own work off; a Canceled one was called off from outside, often
		// by whoever wanted the beacon next, so the beacon was never guaranteed to still be this
		// operation's. None of them are worse off for it being gone.
		case deleting,
			status.Phase == opv1alpha1.OperationPhaseAborted,
			status.Phase == opv1alpha1.OperationPhaseCanceled:
			logrus.Infof("[certificaterotation] %s/%s: beacon %s/%s is gone, nothing to release", op.Namespace, op.Name, namespace, beaconName)

			ops.TerminateUnlessHookOwed(op, &status.OperationStatus)

			return nil, status, nil

		// Already released whatever it held, so a beacon collected afterwards is no concern of this
		// operation's. Let the reconcile settle so TTL collection can take it.
		case ops.IsTerminated(&status.OperationStatus):
			return nil, status, nil

		// Succeeded and Failed both dispatched work to the cluster under the beacon's authority and
		// have not yet handed that authority back. A beacon which has gone missing in that window
		// means the state serializing writes to this cluster was destroyed while an operation still
		// had a claim on it, so complain, and keep complaining, rather than quietly recording the
		// operation as wrapped up. It stays stuck until an administrator looks at it; deleting the
		// operation is the way out, and cancels it on the way.
		case ops.IsTerminal(status.Phase):
			return nil, status, fmt.Errorf("beacon %s/%s is gone while %s/%s has yet to release it: %w",
				namespace, beaconName, op.Namespace, op.Name, err)

		// Pending has not acquired the beacon yet, so its absence is "not created" rather than
		// "lost": the system-agent controller creates one once the cluster can take operations.
		case status.Phase == opv1alpha1.OperationPhasePending, status.Phase == "":
			logrus.Warnf("[certificaterotation]: %s/%s failed to find beacon %s/%s (clusterRef apiVersion=%s kind=%s name=%s)",
				op.Namespace, op.Name, namespace, beaconName, ustr.GetAPIVersion(), ustr.GetKind(), ustr.GetName())

			opv1alpha1.PendingCondition.True(&status)
			opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
			opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")

			return nil, status, nil

		// Anything still in flight has had the beacon taken out from under it, which is the fault
		// handleInProgress reports when it finds the beacon reassigned. There is nothing left to
		// release, so the failure is terminated here for the same reason the missing-cluster failure
		// above is: no later reconcile would reach a terminal handler to do it.
		default:
			logrus.Errorf("[certificaterotation] %s/%s: beacon %s/%s is gone mid-operation, failing", op.Namespace, op.Name, namespace, beaconName)

			status.MarkFailed(opv1alpha1.BeaconLostReason, fmt.Sprintf("beacon %s/%s not found", namespace, beaconName))
			ops.TerminateUnlessHookOwed(op, &status.OperationStatus)

			return nil, status, nil
		}
	}
	if err != nil {
		return nil, status, err
	}

	return &scope{
		ownerKey:   ops.BeaconOwnerKey(OperationKind, op),
		op:         op,
		namespace:  namespace,
		beacon:     beacon,
		clusterObj: clusterObj,
		adapter:    adapter,
	}, status, nil
}

// dispatchPhase routes the operation to the handler for its current phase. An unrecognized phase is
// itself terminal: the controller cannot know what the operation was doing, so it fails it. This
// should be prevented by validation, but is handled just in case.
func (h *handler) dispatchPhase(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
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

// handleHook pushes the delegate named by the operation's hook label for prefix onto the beacon, and
// reports whether there was one — in which case the caller stops where it is and waits.
func (h *handler) handleHook(s *scope, prefix string) (bool, error) {
	logrus.Tracef("[certificaterotation] %s/%s: checking lifecycle hook for prefix %q", s.op.Namespace, s.op.Name, prefix)

	delegated, beacon, err := ops.DelegateForHook(s.op, s.beacon, h.beacons, prefix)
	s.beacon = beacon

	return delegated, err
}

// handlePending acquires or follows beacon ownership and waits for agent registration.
func (h *handler) handlePending(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Tracef("[certificaterotation] %s/%s: handling pending", s.op.Namespace, s.op.Name)

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

	delegated, err := h.handleHook(s, opv1alpha1.PendingPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		ops.SetWaitingForDelegate(opv1alpha1.PendingCondition, &status.OperationStatus, s.beacon)
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

	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.CertificateRotationStepRotate)

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

// handleInProgress enforces beacon authorization and executes the current step.
func (h *handler) handleInProgress(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Tracef("[certificaterotation] %s/%s: handling in-progress", s.op.Namespace, s.op.Name)

	stepPrefix := stepHookPrefixFor(s.op.Status.Step)

	// Stage 1 (loose): the op must appear SOMEWHERE in the ownership chain (owner or any
	// delegate). Being absent entirely means the beacon was reassigned to another controller and
	// we can't recover. If a step hook is currently active on the op, treat the absence as a
	// step-scoped delegation and surface WaitingForDelegate instead of failing — the delegate may
	// have popped us in service of the hook and will restore ownership when the hook clears.
	if !plan.IsOwningBeaconHolder(s.beacon, s.ownerKey) && !plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		if ops.HasStepHookLabel(s.op, stepPrefix) {
			ops.SetWaitingForDelegate(opv1alpha1.InProgressCondition, &status.OperationStatus, s.beacon)
			return status, nil
		}
		logrus.Errorf("[certificaterotation] %s/%s: beacon reassigned, aborting", s.op.Namespace, s.op.Name)
		status.MarkFailed(opv1alpha1.BeaconLostReason, "beacon reassigned, aborting")

		return status, nil
	}

	var err error
	s.beacon, err = plan.ToggleBeacon(s.beacon, true, h.beacons)
	if err != nil {
		return status, err
	}

	// InProgress-phase hook fires on every InProgress reconcile, ahead of step dispatch.
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
		logrus.Errorf("[certificaterotation] %s/%s: beacon lost, aborting", s.op.Namespace, s.op.Name)
		status.MarkFailed(opv1alpha1.BeaconLostReason, "beacon acquired by another controller, aborting")

		return status, nil
	}

	if s.op.Status.Step == opv1alpha1.CertificateRotationStepRotate {
		return h.reconcileRotate(s, status)
	}

	status.MarkFailed(opv1alpha1.UnknownStepReason, fmt.Sprintf("current step [%q] is unknown, expected: [%q]", status.Step, opv1alpha1.CertificateRotationStepRotate))

	return status, nil
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
		ops.SetWaitingForDelegate(opv1alpha1.InProgressCondition, &status.OperationStatus, s.beacon)
		return status, nil
	}

	// Collect every registered machine-plan secret in the collector's safe role order. The
	// requested-service filter runs afterwards so the whole node set is available to decide
	// which services the cluster's distro can actually rotate.
	candidates, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[certificaterotation] %s/%s: aborting operation: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)
		status.MarkAborted(opv1alpha1.PreflightCheckFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	if len(candidates) == 0 {
		logrus.Errorf("[certificaterotation] %s/%s: aborting operation: no eligible machine-plan secrets found", s.op.Namespace, s.op.Name)
		status.MarkAborted(opv1alpha1.PreflightCheckFailedReason, "no eligible machine-plan secrets found")
		return status, nil
	}

	// A service no node's distro exposes can never be rotated. Reject the request here, before
	// any node receives a plan, instead of returning an error that would retry forever.
	requested := s.op.Spec.Args.Services
	if unsupported := unsupportedServices(s.adapter, requested, candidates); len(unsupported) > 0 {
		logrus.Errorf("[certificaterotation] %s/%s: requested services are not available on this %s cluster: %s",
			s.op.Namespace, s.op.Name, s.adapter.RuntimeCommand(), strings.Join(unsupported, ", "))
		status.MarkAborted(opv1alpha1.PreflightCheckFailedReason,
			fmt.Sprintf("requested services are not available on this %s cluster: %s", s.adapter.RuntimeCommand(), strings.Join(unsupported, ", ")))
		return status, nil
	}

	// Narrow the requested services to each candidate's own node once, keeping only the nodes
	// that have at least one applicable service. An empty request applies to every node and
	// keeps the "rotate everything" runtime behavior via a nil per-node service slice.
	targets := make([]rotationTarget, 0, len(candidates))
	for _, secret := range candidates {
		nodeServices := servicesForNode(s.adapter, requested, secret)
		if len(requested) > 0 && len(nodeServices) == 0 {
			continue
		}
		targets = append(targets, rotationTarget{secret: secret, nodeServices: nodeServices})
	}

	// Everything above can reject the request without having touched the cluster, so the pause
	// waits until the operation is committed to dispatching work: a rotation turned away for a
	// service this distro does not have must not leave the cluster paused behind it. PauseCluster is
	// idempotent, so the reconciles that follow re-assert it.
	if err := s.adapter.PauseCluster(true); err != nil {
		return status, err
	}

	// Tie plan content to this operation and step so the system-agent reruns rotated plans
	// instead of reusing stale applied output. Applied only when a plan is assigned, below.
	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)

	for _, target := range targets {
		secret := target.secret

		var dataDir string
		if ops.IsControlPlane(secret) || ops.IsEtcd(secret) {
			dataDir, err = s.adapter.DistroDataDirectory(secret)
			if err != nil {
				return status, err
			}
		}

		// Plans are processed serially. Returning while one plan is waiting ensures
		// the next node is not disrupted until this node has applied and passed probes.
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		runtime := s.adapter.RuntimeCommand()
		runtimeService := s.adapter.RuntimeService(secret)

		var nodePlan plan.Plan

		if ops.IsControlPlane(secret) || ops.IsEtcd(secret) {
			// Server nodes own control-plane or etcd certificates. Stop the server
			// before rotating them, then restart it after all required cleanup.
			provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
			manifestPaths := s.adapter.DistroManifestPaths(dataDir)
			files := []plan.File{ops.IdempotentScriptFile(provisioningDir)}
			// Keep stop and rotate as separate idempotent instructions. A retry can
			// resume safely without rerunning an instruction already applied by the agent.
			oneTime := []plan.OneTimeInstruction{
				ops.IdempotentInstruction(provisioningDir, "certificate-rotation/stop", string(s.op.UID), "systemctl", []string{"stop", runtimeService}, nil),
			}
			oneTime = append(oneTime, certificateRotationRuntimeInstructions(s, secret, dataDir, target.nodeServices)...)

			if ops.IsControlPlane(secret) {
				cleanupInstructions, err := componentCertificateCleanupInstructions(s, secret, target.nodeServices, dataDir, manifestPaths)
				if err != nil {
					return status, err
				}
				oneTime = append(oneTime, cleanupInstructions...)
			}

			if manifestPaths.GeneratedManifestDirectory != "" {
				// Remove generated manifests only after rotation so replacements refer to
				// rotated files when the runtime starts.
				oneTime = append(oneTime, manifestRemovalInstructions(provisioningDir, string(s.op.UID), manifestPaths)...)
			}

			// Restarting the server activates the rotated certificates and lets the
			// probes verify that its local control-plane components recovered.
			oneTime = append(oneTime, linuxIdempotentRestartInstructions(provisioningDir, "certificate-rotation", string(s.op.UID), runtimeService)...)

			nodePlan = plan.Plan{
				Files:               files,
				OneTimeInstructions: oneTime,
				Probes:              probes,
			}
		} else {
			// Workers do not rotate server certificates, but their runtime agent must
			// restart so it reconnects using the updated cluster certificates.
			if ops.IsWindows(secret) {
				// Windows uses a service restart instruction rather than Linux systemctl.
				files := []plan.File{windowsIdempotentScriptFile()}
				oneTime := windowsIdempotentRestartInstructions("certificate-rotation/restart", string(s.op.UID), runtime)
				nodePlan = plan.Plan{
					Files:               files,
					OneTimeInstructions: oneTime,
					Probes:              probes,
				}
			} else {
				provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
				files := []plan.File{ops.IdempotentScriptFile(provisioningDir)}
				oneTime := linuxIdempotentRestartInstructions(provisioningDir, "certificate-rotation", string(s.op.UID), runtimeService)
				nodePlan = plan.Plan{
					Files:               files,
					OneTimeInstructions: oneTime,
					Probes:              probes,
				}
			}
		}

		// AssignPlan updates this machine-plan secret and returns the agent's latest
		// applied status for the same plan. A later reconcile continues from that status.
		planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(&nodePlan, opEnv), 1, 1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			message := fmt.Sprintf(
				"certificate rotation plan failed for %s/%s; verify the runtime service is healthy before starting another disruptive operation: %s",
				secret.Namespace, secret.Name, plan.Message([]plan.PlanStatus{*planStatus}),
			)

			logrus.Errorf("[certificaterotation] %s/%s: %s", s.op.Namespace, s.op.Name, message)
			status.MarkFailed(opv1alpha1.PlanFailedReason, message)
			return status, nil
		}

		if planStatus.Waiting() {
			// Do not assign a plan to another target until the current target reports
			// both instruction completion and successful probes.
			logrus.Debugf("[certificaterotation] %s/%s: waiting for certificate rotation plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)
			ops.SetWaitingForSinglePlan(&status.OperationStatus, planStatus)
			return status, nil
		}
	}

	return h.finishRotation(s, status)
}

// finishRotation hands the cluster back to the provisioner and records the rotation as successful.
// Both belong to the same moment and are done together so that neither can happen without the
// other.
//
// It is reached only from the end of reconcileRotate, once every target has rotated and passed its
// probes, which makes it the one place a rotation unpauses. A rotation which does not get this far
// has left the cluster mid-rotation, with some nodes on new certificates and some on old, and
// letting the provisioner change node plans in that state would be unsafe — so it stays paused,
// deliberately, for an administrator to resolve. That is why terminal handling does not unpause
// either.
func (h *handler) finishRotation(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	// Unpause before asserting the outcome: a rotation reported successful while its cluster is
	// still paused would look finished and leave the cluster frozen.
	if err := s.adapter.PauseCluster(false); err != nil {
		return status, err
	}

	logrus.Infof("[certificaterotation] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.MarkSucceeded()

	return status, nil
}

// terminalPhase describes what is specific to one terminal phase: the lifecycle hook that can defer
// its completion, whether the beacon is guaranteed to still be the operation's, and any work to run
// once the beacon is back.
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
// may hand the beacon to a delegate first, and the beacon has to be released afterwards so the next
// operation in line can acquire it. Recording termination in this one place — after the hook is
// satisfied, after the release succeeded — is what keeps the marker honest, since that marker is
// what makes the operation eligible for TTL collection and lets a deleted operation finish
// deleting. A terminal phase handler that returns early therefore cannot forget to withhold it.
//
// An operation which no longer holds the beacon has none of that left to do: see beaconOptional. It
// terminates without the beacon being written to at all, which is what keeps an operation that lost
// its claim from reaching into whichever one holds it now.
//
// Unpausing the cluster is deliberately not part of this. Only a rotation which rotated every node
// has left the cluster in a state fit to hand back to the provisioner, so reconcileRotate does it
// and no terminal path undoes it — see finishRotation.
func (h *handler) handleTerminal(s *scope, status opv1alpha1.CertificateRotationStatus, phase terminalPhase) (opv1alpha1.CertificateRotationStatus, error) {
	logrus.Debugf("[certificaterotation] %s/%s: handling operation %s", s.op.Namespace, s.op.Name, status.Phase)

	// A phase whose beacon claim is optional passes over its hook once that claim is gone: there is
	// no authority left to delegate, and pushing a delegate onto a beacon another controller now
	// holds would be reaching into its operation. Everything after the hook is either a no-op
	// without a claim (ReleaseBeaconIfHeld) or owed regardless of one, so the operation still
	// terminates — which is what lets it be collected, or lets a deleted one retire its finalizer.
	honorHook := !phase.beaconOptional || plan.HoldsBeacon(s.beacon, s.ownerKey)

	if honorHook {
		delegated, err := h.handleHook(s, phase.hook)
		if err != nil {
			return status, err
		} else if delegated {
			// The delegate drives the beacon on this operation's behalf from here, and the cluster
			// stays paused for it. Nothing is written to the outcome condition: it already reports
			// the outcome with the reason the phase handler gave it, and that reason must survive
			// the delegation. updateStatus reports the delegate on Finalized for as long as the hook
			// label is present, so the wait resolves on its own once the delegate clears the label
			// rather than being left behind on a condition.
			return status, nil
		}
	} else {
		logrus.Debugf("[certificaterotation] %s/%s: %s with no claim on the beacon, leaving it untouched", s.op.Namespace, s.op.Name, status.Phase)
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
func (h *handler) handleAborted(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           opv1alpha1.AbortedPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleCanceled handles the Canceled terminal phase, which is reached when spec.Cancel is set, when
// another controller cancels the operation, or when it is deleted before its terminal handling
// completed. What separates cancellation from the other outcomes is that it comes from outside the
// operation, where Failed means the work was attempted and lost and Aborted means the operation
// called it off itself — none implies another.
func (h *handler) handleCanceled(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           opv1alpha1.CanceledPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleFailed handles the Failed terminal phase. Its hook fires before the beacon is released so a
// delegate can inspect failure state (status conditions, plan-secret applied output, residual
// node-side scripts) before the next operation is allowed to acquire the beacon.
func (h *handler) handleFailed(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           opv1alpha1.FailedPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleSucceeded handles the Succeeded terminal phase. On the owner path it also nudges the parent
// cluster controller by enqueueing the cluster object, so any post-operation reconciliation runs
// promptly rather than waiting for the next periodic resync. Only the owner does so, since only the
// owner terminating implies downstream work.
func (h *handler) handleSucceeded(s *scope, status opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook: opv1alpha1.SucceededPhaseHookLabelPrefix,
		onRelease: func(s *scope, owning bool) {
			if !owning {
				return
			}
			gvk := schema.FromAPIVersionAndKind(s.clusterObj.GetAPIVersion(), s.clusterObj.GetKind())
			_ = h.dynamic.Enqueue(gvk, s.clusterObj.GetNamespace(), s.clusterObj.GetName())
		},
	})
}

// updateStatus refreshes ObservedGeneration and every condition that is not the one the current
// phase handler owns. Every operation type reports its progress identically, so the work itself is
// shared — see ops.UpdateStatus.
func updateStatus(op *opv1alpha1.CertificateRotation, status opv1alpha1.CertificateRotationStatus) opv1alpha1.CertificateRotationStatus {
	logrus.Tracef("[certificaterotation] %s/%s: updating conditions", op.Namespace, op.Name)

	ops.UpdateStatus(op, &op.Spec.OperationSpec, &status.OperationStatus)

	return status
}

// rotationTarget pairs a selected machine-plan secret with the requested services already
// narrowed to the ones that apply to its node, so reconcileRotate computes that narrowing once
// per node instead of once for selection and again for plan building.
type rotationTarget struct {
	secret       *corev1.Secret
	nodeServices []string
}

// servicesForNode narrows a cluster-wide service request down to the services that apply to the
// node described by secret. A request can be valid across the whole cluster while spanning
// multiple node roles — for example "etcd" and "scheduler" together — so each server must only be
// told to rotate the services it actually owns.
//
// An empty requested slice already means "rotate every service the runtime supports" as far as
// the runtime command is concerned, so it is returned unchanged rather than expanded into
// adapter.DistroServices(secret).
func servicesForNode(adapter ops.Adapter, requested []string, secret *corev1.Secret) []string {
	if len(requested) == 0 {
		return nil
	}

	available := adapter.DistroServices(secret)
	var nodeServices []string
	for _, service := range requested {
		if slices.Contains(available, service) {
			nodeServices = append(nodeServices, service)
		}
	}
	return nodeServices
}

// unsupportedServices returns the requested services that no target node's distro provides.
// A service belonging to the other distro — k3s-server on an RKE2 cluster, for example — can
// never be rotated, so the operation must reject it instead of quietly selecting no targets.
func unsupportedServices(adapter ops.Adapter, requested []string, targets []*corev1.Secret) []string {
	if len(requested) == 0 {
		return nil
	}

	supported := map[string]struct{}{}
	for _, secret := range targets {
		for _, service := range adapter.DistroServices(secret) {
			supported[service] = struct{}{}
		}
	}

	var unsupported []string
	for _, service := range requested {
		if _, ok := supported[service]; !ok {
			unsupported = append(unsupported, service)
		}
	}
	return unsupported
}

// componentCertificateCleanupInstructions builds default certificate/key cleanup
// instructions for controller-manager and scheduler on one node. services must already be
// narrowed to the ones that apply to secret's node.
func componentCertificateCleanupInstructions(s *scope, secret *corev1.Secret, services []string, dataDir string, manifestPaths ops.ManifestPaths) ([]plan.OneTimeInstruction, error) {
	provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
	operationID := string(s.op.UID)

	components := []struct {
		service        string
		probeName      string
		certificate    string
		certificateDir string
		manifest       string
	}{
		{
			service:        "controller-manager",
			probeName:      ops.KubeControllerManagerProbeName,
			certificate:    ops.DefaultKubeControllerManagerCert,
			certificateDir: ops.DefaultKubeControllerManagerCertDir,
			manifest:       "kube-controller-manager.yaml",
		},
		{
			service:        "scheduler",
			probeName:      ops.KubeSchedulerProbeName,
			certificate:    ops.DefaultKubeSchedulerCert,
			certificateDir: ops.DefaultKubeSchedulerCertDir,
			manifest:       "kube-scheduler.yaml",
		},
	}

	instructions := []plan.OneTimeInstruction{}
	for _, component := range components {
		// A service-filtered operation must not restart or remove certificates for
		// components that were not selected by the caller. Check this before reading the
		// component's TLS settings, since an unselected component's settings are irrelevant.
		if len(services) > 0 && !slices.Contains(services, component.service) {
			continue
		}

		settings, err := s.adapter.ComponentTLSSettings(secret, component.probeName)
		if err != nil {
			return nil, err
		}
		// An explicit TLS pair is the component's active serving certificate. The
		// default generated paths are not used in that configuration.
		if settings.HasCompleteTLSConfig() {
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

		if manifestPaths.StaticPodManifestDirectory != "" {
			// The runtime regenerates the static-pod manifest when it is absent. Removing
			// it makes the restarted server use the newly generated component certificate.
			instructions = append(instructions,
				ops.IdempotentInstruction(provisioningDir, "certificate-rotation/rm-"+component.service+"-spm", operationID, "rm", []string{"-f", path.Join(manifestPaths.StaticPodManifestDirectory, component.manifest)}, nil),
			)
		}
	}

	return instructions, nil
}

// certificateRotationRuntimeInstructions invokes the runtime's certificate rotation command.
// An empty services slice deliberately rotates every service supported by the runtime. This
// helper takes scope directly because it needs several scope-derived values (provisioning
// directory, operation UID, runtime command) alongside the node-specific arguments.
func certificateRotationRuntimeInstructions(s *scope, secret *corev1.Secret, dataDir string, services []string) []plan.OneTimeInstruction {
	provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
	operationID := string(s.op.UID)
	runtimeCommand := s.adapter.RuntimeCommand()

	args := []string{"certificate", "rotate", "--data-dir", dataDir}
	for _, service := range services {
		args = append(args, "-s", service)
	}

	return []plan.OneTimeInstruction{
		ops.IdempotentInstruction(provisioningDir, "certificate-rotation/rotate", operationID, runtimeCommand, args, nil),
	}
}

// manifestRemovalInstructions removes runtime-owned generated manifests so the server recreates
// them using the rotated certificates when it starts again.
func manifestRemovalInstructions(provisioningDir, operationID string, manifestPaths ops.ManifestPaths) []plan.OneTimeInstruction {
	instructions := make([]plan.OneTimeInstruction, 0, len(manifestPaths.GeneratedManifestPatterns))
	for _, pattern := range manifestPaths.GeneratedManifestPatterns {
		instructions = append(instructions,
			ops.IdempotentInstruction(provisioningDir, "certificate-rotation/manifest-removal", operationID, "/bin/sh",
				[]string{"-c", `rm -f -- "$1"/` + pattern, "--", manifestPaths.GeneratedManifestDirectory}, nil),
		)
	}
	return instructions
}

// linuxIdempotentRestartInstructions resets a failed systemd unit when needed, then restarts it.
// Keeping the systemctl commands together gives each restart the same retry-safe behavior.
func linuxIdempotentRestartInstructions(provisioningDir, identifier, value, service string) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		ops.IdempotentInstruction(provisioningDir, identifier+"-reset-failed", value, "/bin/sh", []string{"-c", fmt.Sprintf("if [ $(systemctl is-failed %s) = failed ]; then systemctl reset-failed %s; fi", service, service)}, nil),
		ops.IdempotentInstruction(provisioningDir, identifier+"-restart", value, "systemctl", []string{"restart", service}, nil),
	}
}
