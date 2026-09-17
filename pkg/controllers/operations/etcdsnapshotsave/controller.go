package etcdsnapshotsave

import (
	"context"
	"fmt"
	"slices"
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
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// ControllerOwnerKey is the value used to identify the etcd-snapshot-save handler currently owns the beacon.
	ControllerOwnerKey = "etcd-snapshot-save"

	Finalizer = "etcdsnapshotsave.operation.cattle.io"

	// Step hook label prefixes for the etcdsnapshotsave operation. They follow the shared label
	// semantics documented on planv1alpha1's phase-hook label constants, but each prefix only fires
	// when the operation enters the matching step.

	// PreflightStepHookLabelPrefix gates the Preflight step, before the controller performs
	// the necessary preflight checks to determine whether or not the operation can proceed.
	PreflightStepHookLabelPrefix = "preflight.step.hook.operation.cattle.io/"

	// SaveStepHookLabelPrefix gates the Save step before reconcileSave assigns the
	// `<runtime> etcd-snapshot save` plan to any etcd-labeled machine-plan secret.
	SaveStepHookLabelPrefix = "save.step.hook.operation.cattle.io/"

	// RestartStepHookLabelPrefix gates the Restart step before reconcileRestart assigns the
	// `systemctl restart <server-unit>` plan to any etcd-labeled machine-plan secret.
	RestartStepHookLabelPrefix = "restart.step.hook.operation.cattle.io/"
)

// stepHookPrefixFor returns the step-hook label prefix for the given snapshot-save step, or "" for
// an unknown / empty step. Used by handleInProgress to decide whether beacon-authorization loss is
// explained by an active step-scoped delegation vs a genuine loss.
func stepHookPrefixFor(step opv1alpha1.ETCDSnapshotSaveStep) string {
	switch step {
	case opv1alpha1.ETCDSnapshotSaveStepPreflight:
		return PreflightStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotSaveStepSave:
		return SaveStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotSaveStepRestart:
		return RestartStepHookLabelPrefix
	}
	return ""
}

// dynamicResolver is the subset of *dynamic.Controller this handler needs: Get for cluster
// lookup during onChange dispatch, and Enqueue for nudging the parent cluster controller after a
// successful operation. It's an interface so tests can substitute a stub — *dynamic.Controller
// satisfies it directly.
type dynamicResolver interface {
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
	Enqueue(gvk schema.GroupVersionKind, namespace, name string) error
}

// handler is the per-cluster reconciliation state for the ETCDSnapshotSave controller. All fields
// are populated at Register time; the handler itself is stateless across reconciles.
type handler struct {
	etcdsnapshotsaves operationcontrollers.ETCDSnapshotSaveController

	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache

	secrets corecontrollers.SecretClient

	store *plan.Store

	dynamic dynamicResolver

	clients *wrangler.CAPIContext
}

// Register wires the ETCDSnapshotSave controller into the given wrangler context. It must be
// called exactly once per process; subsequent calls would clobber the registered status handler.
func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		etcdsnapshotsaves: clients.Operation.ETCDSnapshotSave(),
		beacons:           clients.Plan.Beacon(),
		beaconCache:       clients.Plan.Beacon().Cache(),
		secrets:           clients.Core.Secret(),
		dynamic:           clients.Dynamic,
		store:             plan.NewStore(clients.Core.Secret()),
		clients:           clients,
	}

	operationcontrollers.RegisterETCDSnapshotSaveStatusHandler(ctx, clients.Operation.ETCDSnapshotSave(), "", "etcd-snapshot-create-handler", h.OnChange)
}

// OnChange is the status handler entrypoint invoked by the wrangler-registered controller. It
// delegates the phase-specific work to onChange, then runs the common condition refresh through
// updateStatus.
//
// When the resulting status is byte-identical to the prior status (no state moved this tick), the
// handler either deletes the operation (terminal phase past its TTL, terminal handling complete —
// frees the beacon as a side effect of the watcher seeing the deletion) or re-enqueues itself after
// 5 seconds so the next poll can pick up any out-of-band changes (plan secret state, beacon
// transitions, etc.).
//
// Operations which are already deleting skip both of those and go through handleDeletion instead.
func (h *handler) OnChange(op *opv1alpha1.ETCDSnapshotSave, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	// A deleted operation has no TTL to enforce and nothing left to poll for; the only remaining
	// work is retiring the finalizer once its teardown is done. This runs ahead of the paused check
	// so a paused operation which is already deleting still gets torn down.
	if op.DeletionTimestamp != nil {
		return h.handleDeletion(op, status)
	}

	// Paused operations resume on a spec change; skip TTL cleanup and polling until then.
	if ops.IsPaused(&op.Spec.OperationSpec) {
		return status, nil
	}

	if equality.Semantic.DeepEqual(op.Status, status) {
		// handle after normal processing to allow for proper phase-related cleanup (freeing beacon)
		//
		// The IsTerminated and HasActiveLifecycleHook guards defer TTL garbage collection until
		// terminal handling has actually completed. Without them, an op that has reached a terminal
		// phase but is waiting on a delegate (handleSucceeded/handleFailed/handleCanceled returned
		// early with WaitingForDelegate) would be deleted on the very next reconcile as soon as the
		// TTL is past, stranding the beacon delegate and any observer polling for the terminal
		// phase. It would also be canceled on the way out by the deletion handling above, which
		// would bury the phase it actually finished in.
		if ops.IsTerminal(status.Phase) &&
			ops.IsTerminated(&status.OperationStatus) &&
			ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) &&
			!planv1alpha1.HasActiveLifecycleHook(op) {
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

// handleDeletion drives the tail end of the deletion flow for an operation still carrying our
// finalizer. onChange has already canceled the operation if it was deleted mid-flight and run the
// terminal phase handler for it; all that is left here is deciding whether the finalizer can go.
//
// The finalizer is held until terminal handling has been recorded as complete, which keeps the
// operation — and with it any beacon delegation made on its behalf — alive while a terminal phase
// hook delegate finishes its work. The terminal status is also persisted before the finalizer is
// dropped, so an observer waiting on the final phase gets to see it rather than the object simply
// vanishing.
func (h *handler) handleDeletion(op *opv1alpha1.ETCDSnapshotSave, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	if !equality.Semantic.DeepEqual(op.Status, status) {
		// State moved this tick: let the status handler write it out. The resulting update
		// re-enqueues the operation, and the next pass retires the finalizer.
		return status, nil
	}

	if !ops.IsTerminated(&status.OperationStatus) {
		// Terminal handling is still in flight — typically a canceled phase hook whose delegate has
		// yet to hand the beacon back. Keep the finalizer and poll for it to finish.
		logrus.Debugf("[etcdsnapshotsave] %s/%s: deferring deletion, terminal handling has not completed", op.Namespace, op.Name)
		h.etcdsnapshotsaves.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)
		return status, nil
	}

	logrus.Infof("[etcdsnapshotsave] %s/%s: terminal handling complete, releasing operation for deletion", op.Namespace, op.Name)

	return status, h.removeFinalizer(op)
}

// ensureFinalizer adds our finalizer to the operation if it is not already present. The status
// handler only ever persists status, so the finalizer has to be written with an explicit Update;
// the updated object is copied back over op so the resource version the status handler goes on to
// use for its own UpdateStatus is not stale.
func (h *handler) ensureFinalizer(op *opv1alpha1.ETCDSnapshotSave) error {
	if slices.Contains(op.Finalizers, Finalizer) {
		return nil
	}

	logrus.Debugf("[etcdsnapshotsave] %s/%s: adding finalizer", op.Namespace, op.Name)

	updated := op.DeepCopy()
	updated.Finalizers = append(updated.Finalizers, Finalizer)

	updated, err := h.etcdsnapshotsaves.Update(updated)
	if err != nil {
		return err
	}

	*op = *updated

	return nil
}

// removeFinalizer drops our finalizer from the operation, which lets the API server complete the
// deletion. A NotFound is treated as success: something else (another finalizer holder finishing
// last, or a previous attempt whose response was lost) already let the object go.
func (h *handler) removeFinalizer(op *opv1alpha1.ETCDSnapshotSave) error {
	if !slices.Contains(op.Finalizers, Finalizer) {
		return nil
	}

	updated := op.DeepCopy()
	updated.Finalizers = slices.DeleteFunc(updated.Finalizers, func(f string) bool {
		return f == Finalizer
	})

	updated, err := h.etcdsnapshotsaves.Update(updated)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	*op = *updated

	return nil
}

// scope bundles the per-reconcile values derived from the operation, parent cluster, and beacon.
// It is built fresh on every invocation and threaded through the phase handlers so they don't
// each have to re-derive the same data.
type scope struct {
	ownerKey string

	op        *opv1alpha1.ETCDSnapshotSave
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

// onChange takes the finalizer (or, for an operation being deleted, cancels it when its terminal
// handling had not completed), resolves the parent cluster reference, locates the cluster's beacon,
// builds an Adapter for the cluster kind, and dispatches to the phase-specific handler. Returns the
// unmodified status when:
//
//   - op is nil, or paused and not being deleted;
//   - the beacon has not yet been created during the Pending phase (allows the system-agent watcher
//     to create it before we fail the operation);
//   - the operation has reached a terminal phase (handleSucceeded/handleFailed perform their own
//     beacon cleanup).
//
// A deleted operation whose cluster or beacon no longer exists is marked terminated on the spot:
// there is nothing left to release, and the finalizer must not outlive the cluster it refers to.
//
// Marks the operation Failed when the parent cluster is missing or the current phase is unknown.
func (h *handler) onChange(op *opv1alpha1.ETCDSnapshotSave, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	if op == nil {
		return status, nil
	}

	deleting := op.DeletionTimestamp != nil

	if deleting {
		// Teardown is driven off our finalizer. Without it the operation has either already been
		// torn down or was never ours to begin with, and there is nothing left to reconcile.
		if !slices.Contains(op.Finalizers, Finalizer) {
			return status, nil
		}

		// An operation deleted before its terminal handling completed is canceled: the work it
		// dispatched is no longer tracked by anything, so it can neither be reported as succeeded
		// nor as failed. The terminal handler for the Canceled phase then runs below (releasing the
		// beacon, honouring any canceled phase hook) before OnChange drops the finalizer.
		//
		// Note this deliberately also demotes an operation which reached Succeeded or Failed but
		// whose terminal handling had not finished yet — until that completes its beacon is still
		// held, on its own behalf or a delegate's.
		if !ops.IsTerminated(&status.OperationStatus) && status.Phase != opv1alpha1.OperationPhaseCanceled {
			logrus.Infof("[etcdsnapshotsave] %s/%s: marking operation as canceled: deleted in phase [%s] before terminal handling completed", op.Namespace, op.Name, status.Phase)

			status.SetPhase(opv1alpha1.OperationPhaseCanceled)

			opv1alpha1.CanceledCondition.True(&status)
			opv1alpha1.CanceledCondition.Reason(&status, opv1alpha1.OperationDeletedReason)
			opv1alpha1.CanceledCondition.Message(&status, "operation deleted before terminal handling completed")
		}
	} else {
		if ops.IsPaused(&op.Spec.OperationSpec) {
			logrus.Debugf("[etcdsnapshotsave] %s/%s: skipping paused operation", op.Namespace, op.Name)
			return status, nil
		}

		// The finalizer is what guarantees the controller observes the deletion of an operation
		// which is still in flight, so it can cancel it and release the beacon. Paused operations
		// are intentionally skipped above: a paused operation has dispatched nothing since it was
		// paused, and taking the finalizer would only wedge its deletion until it is resumed.
		if err := h.ensureFinalizer(op); err != nil {
			return status, err
		}
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

		// The beacon lives alongside the cluster, so a deleted operation whose cluster is gone has
		// nothing left to release: terminal handling is trivially complete and the operation is
		// free to finish deleting. Failing it here instead would both overwrite the Canceled phase
		// and, for a cluster deleted mid-operation, wedge the deletion behind our finalizer.
		if deleting {
			logrus.Infof("[etcdsnapshotsave] %s/%s: cluster %s is gone, nothing to release", op.Namespace, op.Name, key)
			status.SetTerminated()
			return status, nil
		}

		logrus.Errorf("[etcdsnapshotsave]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.ClusterNotFoundReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("cluster %s not found", key))

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		// This failure is terminated on the spot rather than by handleFailed: the beacon is
		// resolved through the cluster's adapter, so with no cluster there is no beacon to release
		// and every subsequent reconcile would return here without ever reaching the terminal
		// handler — leaving the operation ineligible for TTL garbage collection forever.
		status.SetTerminated()

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

	a, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return status, err
	}

	clusterObj, err := a.ClusterObject()
	if err != nil {
		return status, err
	}

	// Resolve the beacon (and every other cluster-scoped artifact — plan secrets, snapshot CRs)
	// via the adapter, not the op.Spec.ClusterRef. See the equivalent block in
	// etcdsnapshotrestore/controller.go for the full rationale.
	namespace, beaconName := a.BeaconRef()

	beacon, err := h.beacons.Get(namespace, beaconName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && deleting {
		// As above: no beacon means nothing to release, so let the deletion proceed rather than
		// requeueing a NotFound forever.
		logrus.Infof("[etcdsnapshotsave] %s/%s: beacon %s/%s is gone, nothing to release", op.Namespace, op.Name, namespace, beaconName)
		status.SetTerminated()
		return status, nil
	} else if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		logrus.Warnf("[etcdsnapshotsave]: %s/%s failed to find beacon %s/%s (clusterRef apiVersion=%s kind=%s name=%s)",
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
		beacon:     beacon,
		namespace:  namespace,
		clusterObj: clusterObj,
		adapter:    a,
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
	}

	status.SetPhase(opv1alpha1.OperationPhaseFailed)

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))

	return status, nil
}

func (h *handler) lifecycleHookDelegate(s *scope, prefix string) (string, string) {
	if s.op.Labels == nil {
		return "", ""
	}

	for k, v := range s.op.Labels {
		if after, ok := strings.CutPrefix(k, prefix); ok {
			return after, v
		}
	}

	return "", ""
}

func (h *handler) delegate(s *scope, name, delegate string) error {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: delegating ownership of beacon to %s on behalf of %s", s.op.Namespace, s.op.Name, delegate, name)

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

func (h *handler) handleHook(s *scope, prefix string) (bool, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling before hook", s.op.Namespace, s.op.Name)

	if name, delegate := h.lifecycleHookDelegate(s, prefix); delegate != "" {
		err := h.delegate(s, name, delegate)
		return true, err
	}

	return false, nil
}

// handlePending advances a Pending operation through the prerequisite checks: acquire the
// cluster's beacon, then wait for every expected system-agent to register a machine-plan secret.
// On success the operation transitions to InProgress at the Save step. Otherwise it remains
// Pending with a condition explaining what we're still waiting on (beacon ownership or agent
// registration).
func (h *handler) handlePending(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling pending", s.op.Namespace, s.op.Name)

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
			opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")
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

	logrus.Infof("[etcdsnapshotsave] %s/%s: transitioning to preflight", s.op.Namespace, s.op.Name)

	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.ETCDSnapshotSaveStepPreflight)

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

// handleInProgress is invoked once the operation is past Pending. It re-verifies beacon ownership
// (defends against another controller swooping in mid-operation), marks the beacon active so the
// system-agent will keep polling, and then dispatches to the step-specific reconciler. An unknown
// step marks the operation Failed.
func (h *handler) handleInProgress(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling in-progress", s.op.Namespace, s.op.Name)

	stepPrefix := stepHookPrefixFor(s.op.Status.Step)

	// Stage 1 (loose): the op must appear SOMEWHERE in the ownership chain (owner or any
	// delegate). Being absent entirely means the beacon was reassigned to another controller and
	// we can't recover. If a step hook is currently active on the op, treat the absence as a
	// step-scoped delegation and surface WaitingForDelegate instead of failing — the delegate may
	// have popped us in service of the hook and will restore ownership when the hook clears.
	if !plan.IsOwningBeaconHolder(s.beacon, s.ownerKey) && !plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		if planv1alpha1.HasStepHookLabel(s.op, stepPrefix) {
			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
			opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
			return status, nil
		}
		logrus.Errorf("[etcdsnapshotsave] %s/%s: beacon reassigned, aborting", s.op.Namespace, s.op.Name)
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

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
	if !plan.AuthorizedForBeacon(s.beacon, s.ownerKey) {
		if planv1alpha1.HasStepHookLabel(s.op, stepPrefix) {
			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
			opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
			return status, nil
		}
		logrus.Errorf("[etcdsnapshotsave] %s/%s: beacon lost, aborting", s.op.Namespace, s.op.Name)
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "Beacon acquired by another controller, aborting")

		return status, nil
	}

	switch s.op.Status.Step {
	case opv1alpha1.ETCDSnapshotSaveStepPreflight:
		return h.reconcilePreflight(s, status)
	case opv1alpha1.ETCDSnapshotSaveStepSave:
		return h.reconcileSave(s, status)
	case opv1alpha1.ETCDSnapshotSaveStepRestart:
		return h.reconcileRestart(s, status)
	default:
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownStepReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf(
			"current step [\"%s\"] is unknown, expected one of: [\"%s\", \"%s\", \"%s\"]",
			status.Step,
			opv1alpha1.ETCDSnapshotSaveStepPreflight,
			opv1alpha1.ETCDSnapshotSaveStepSave,
			opv1alpha1.ETCDSnapshotSaveStepRestart))

	}

	return status, nil
}

// reconcilePreflight does not currently run any plans, but exists to ensure that the cluster has a sufficient amount of
// etcd nodes to run the operation on.
func (h *handler) reconcilePreflight(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Debugf("[etcdsnapshotsave] %s/%s: handling preflight", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, PreflightStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	_, err = plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(plan.DefaultSorter()).
		WithFilter(ops.IsEtcd).
		WithValidator(plan.AtLeast(1, "")).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotsave] %s/%s: marking operation as canceled: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseCanceled)

		opv1alpha1.CanceledCondition.True(&status)
		opv1alpha1.CanceledCondition.Reason(&status, opv1alpha1.PreflightCheckFailedReason)
		opv1alpha1.CanceledCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	logrus.Infof("[etcdsnapshotsave] %s/%s: transitioning to save", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotSaveStepSave)
	return status, nil
}

// reconcileSave assigns the `<runtime> etcd-snapshot save` plan to every etcd-labeled
// machine-plan secret in the cluster. The snapshot Args (Name/Compress/Dir) are appended to the
// command verbatim when set.
//
// Per-secret outcomes:
//   - the plan is still applying → returns InProgress with a waiting-for-plan message and lets
//     the next reconcile poll the agent's feedback;
//   - the plan failed (system-agent saturated the retry budget) → marks the entire operation
//     Failed with the offending secret in the message;
//   - the plan applied successfully → continues to the next etcd secret.
//
// Once every etcd secret has applied the plan, transitions to the Restart step.
func (h *handler) reconcileSave(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Debugf("[etcdsnapshotsave] %s/%s: handling snapshot save", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, SaveStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// collect etcd nodes belonging to cluster
	secrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithLabels(plan.Label(capr.EtcdRoleLabel, "true")).
		WithSorter(plan.DefaultSorter()).
		WithValidator(plan.AtLeast(1, "")).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotsave] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	concurrency := len(secrets)
	results := make([]plan.PlanStatus, 0, concurrency)

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		saveInstruction := plan.OneTimeInstruction{
			Name:    "snapshot",
			Command: s.adapter.RuntimeCommand(),
			Args: []string{
				"etcd-snapshot",
				"save",
			},
		}

		if s.op.Spec.Args.Name != "" {
			saveInstruction.CommonInstruction.Args = append(saveInstruction.CommonInstruction.Args, "--name", s.op.Spec.Args.Name)
		}

		nodePlan := &plan.Plan{
			OneTimeInstructions: []plan.OneTimeInstruction{
				saveInstruction,
			},
			Probes: probes,
		}

		planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
		if err != nil {
			return status, err
		}

		results = append(results, *planStatus)

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotsave] %s/%s: marking operation as failed: failed to apply plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("etcd snapshot save failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if planStatus.Waiting() {
			logrus.Debugf("[etcdsnapshotsave] %s/%s: waiting for snapshot save for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			concurrency--
			if concurrency <= 0 {
				break
			}
		}
	}

	if concurrency < len(secrets) {
		msg := plan.Message(results)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting in step %s: %s", status.Step, msg))

		return status, nil
	}

	logrus.Infof("[etcdsnapshotsave] %s/%s: transitioning to restart", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotSaveStepRestart)
	return status, nil
}

// reconcileRestart issues `systemctl restart <server-unit>` against every etcd-labeled
// machine-plan secret. The restart is required because some snapshot configmaps need the etcd
// server to roll before they're visible (per the K3s/RKE2 snapshot bug referenced upstream).
//
// On per-secret failure marks the operation Failed; on per-secret pending returns InProgress with
// a wait message. After all secrets have applied the restart, marks the operation Succeeded.
func (h *handler) reconcileRestart(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Debugf("[etcdsnapshotsave] %s/%s: handling service restart", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, RestartStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// collect etcd nodes belonging to cluster
	secrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithLabels(plan.Label(capr.EtcdRoleLabel, "true")).
		WithSorter(plan.DefaultSorter()).
		WithValidator(plan.AtLeast(1, "")).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotsave] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	concurrency := 1
	results := make([]plan.PlanStatus, 0, concurrency)

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		nodePlan := &plan.Plan{
			OneTimeInstructions: []plan.OneTimeInstruction{
				{
					Name:    "restart",
					Command: "systemctl",
					Args: []string{
						"restart",
						s.adapter.ServerUnit(),
					},
				},
			},
			Probes: probes,
		}

		planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
		if err != nil {
			return status, err
		}

		results = append(results, *planStatus)

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotsave] %s/%s: marking operation as failed: failed to apply plan for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("restart failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if planStatus.Waiting() {
			logrus.Debugf("[etcdsnapshotsave] %s/%s: waiting for systemctl restart for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			concurrency--
			if concurrency <= 0 {
				break
			}
		}
	}

	if concurrency < 1 {
		msg := plan.Message(results)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting in step %s: %s", status.Step, msg))

		return status, nil
	}

	logrus.Infof("[etcdsnapshotsave] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.SetPhase(opv1alpha1.OperationPhaseSucceeded)

	opv1alpha1.SucceededCondition.True(&status)
	opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.FinishedReason)
	opv1alpha1.SucceededCondition.Message(&status, "Operation completed successfully")

	return status, nil
}

// handleCanceled (unlike other handler functions) will attempt to release the beacon (if it is
// held by the current object).
// The difference between canceled and failed operations is that an operation fails itself, whereas another controller
// cancels an operation, or it is deleted before its terminal handling completed.
// This function mostly serves to execute the planv1alpha1.CanceledPhaseHookLabelPrefix, if it exists.
// Once the hook has been satisfied and the beacon released, the operation is recorded as
// terminated, which is what allows a deleted operation to finish deleting.
func (h *handler) handleCanceled(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling operation canceled", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, planv1alpha1.CanceledPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.CanceledCondition.True(&status)
		opv1alpha1.CanceledCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.CanceledCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// Owner and mid-chain delegates both go through ReleaseBeacon: it clears the beacon fully
	// for the owner, or removes the delegate slot from the chain otherwise.
	if plan.IsOwningBeaconHolder(s.beacon, s.ownerKey) || plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		if err := plan.ReleaseBeacon(s.beacon, h.beacons, s.ownerKey); err != nil {
			return status, err
		}
	}

	status.SetTerminated()

	return status, nil
}

// handleFailed releases the beacon when the operation has reached the Failed terminal phase, so
// the next operation in line can acquire it, then records the operation as terminated. The
// toggle-off step pairs with handleSucceeded's behaviour so the beacon's Active flag accurately
// reflects whether any operation is currently running.
func (h *handler) handleFailed(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, planv1alpha1.FailedPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// ReleaseBeacon does the appropriate cleanup in a single UpdateStatus call: if we're the
	// owner it clears Active + Owner + Delegates; if we're in the delegate chain it removes us
	// from the chain (any position). The owner-vs-chain guard just avoids the no-op call.
	if plan.IsOwningBeaconHolder(s.beacon, s.ownerKey) || plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		if err := plan.ReleaseBeacon(s.beacon, h.beacons, s.ownerKey); err != nil {
			return status, err
		}
	}

	status.SetTerminated()

	return status, nil
}

// handleSucceeded mirrors handleFailed for the Succeeded terminal phase: releases the beacon
// (owner path clears it fully, delegate path pops us off the chain) and, on the owner path,
// nudges the parent cluster controller (snapshotbackpopulate, RKE controlplane, etc.) by
// enqueueing the cluster object so any post-operation reconciliation runs promptly rather than
// waiting for the next periodic resync. The operation is recorded as terminated once that is done.
func (h *handler) handleSucceeded(s *scope, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	logrus.Tracef("[etcdsnapshotsave] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, planv1alpha1.SucceededPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.SucceededCondition.True(&status)
		opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.SucceededCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	// Owner and mid-chain delegates both go through ReleaseBeacon; only the owner enqueues the
	// parent cluster, since only the owner terminating implies downstream reconciliation.
	owning := plan.IsOwningBeaconHolder(s.beacon, s.ownerKey)
	if owning || plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		if err := plan.ReleaseBeacon(s.beacon, h.beacons, s.ownerKey); err != nil {
			return status, err
		}
	}
	if owning {
		// enqueue original object to ensure it is processed by requisite controllers
		gvk := schema.FromAPIVersionAndKind(s.clusterObj.GetAPIVersion(), s.clusterObj.GetKind())
		_ = h.dynamic.Enqueue(gvk, s.clusterObj.GetNamespace(), s.clusterObj.GetName())
	}

	status.SetTerminated()

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
