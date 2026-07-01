package etcdsnapshotsave

import (
	"context"
	"fmt"
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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ControllerOwnerKey is the value used to identify the etcd-snapshot-save handler currently owns the beacon.
const ControllerOwnerKey = "etcd-snapshot-save"

// Step hook label prefixes for the etcdsnapshotsave operation. They follow the shared label
// semantics documented on planv1alpha1's phase-hook label constants, but each prefix only fires
// when the operation enters the matching step.
const (
	// SaveStepHookLabelPrefix gates the Save step before reconcileSave assigns the
	// `<runtime> etcd-snapshot save` plan to any etcd-labeled machine-plan secret.
	SaveStepHookLabelPrefix = "save.step.hook.operation.cattle.io/"

	// RestartStepHookLabelPrefix gates the Restart step before reconcileRestart assigns the
	// `systemctl restart <server-unit>` plan to any etcd-labeled machine-plan secret.
	RestartStepHookLabelPrefix = "restart.step.hook.operation.cattle.io/"
)

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
// handler either deletes the operation (terminal phase past its TTL — frees the beacon as a side
// effect of the watcher seeing the deletion) or re-enqueues itself after 5 seconds so the next
// poll can pick up any out-of-band changes (plan secret state, beacon transitions, etc.).
func (h *handler) OnChange(op *opv1alpha1.ETCDSnapshotSave, status opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error) {
	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	if equality.Semantic.DeepEqual(op.Status, status) {
		// handle after normal processing to allow for proper phase-related cleanup (freeing beacon)
		//
		// The HasActiveLifecycleHook guard defers TTL garbage collection while any lifecycle-hook
		// label is still on the op. Without it, an op that has reached a terminal phase but is
		// waiting on a delegate (handleSucceeded/handleFailed/handleCanceled returned early with
		// WaitingForDelegate) would be deleted on the very next reconcile as soon as the TTL is
		// past, stranding the beacon delegate and any observer polling for the terminal phase.
		if ops.IsTerminal(status.Phase) &&
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

// scope bundles the per-reconcile values derived from the operation, parent cluster, and beacon.
// It is built fresh on every invocation and threaded through the phase handlers so they don't
// each have to re-derive the same data.
type scope struct {
	op        *opv1alpha1.ETCDSnapshotSave
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

// onChange resolves the parent cluster reference, locates the cluster's beacon, builds an Adapter
// for the cluster kind, and dispatches to the phase-specific handler. Returns the unmodified
// status when:
//
//   - op is nil, being deleted, or paused;
//   - the beacon has not yet been created during the Pending phase (allows the system-agent watcher
//     to create it before we fail the operation);
//   - the operation has reached a terminal phase (handleSucceeded/handleFailed perform their own
//     beacon cleanup).
//
// Marks the operation Failed when the parent cluster is missing or the current phase is unknown.
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

	beacon, err := plan.AcquireBeacon(s.beacon, h.beacons, ControllerOwnerKey)
	if err != nil {
		return status, err
	}
	if beacon == nil {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")
		return status, nil
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

	logrus.Infof("[etcdsnapshotsave] %s/%s: transitioning to save", s.op.Namespace, s.op.Name)

	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.ETCDSnapshotSaveStepSave)

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

	if !plan.IsOwningBeaconHolder(s.beacon, ControllerOwnerKey) {
		logrus.Errorf("[etcdsnapshotsave] %s/%s: beacon lost, aborting", s.op.Namespace, s.op.Name)
		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.BeaconLostReason)
		opv1alpha1.FailedCondition.Message(&status, "Beacon acquired by another controller, aborting")

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

	switch s.op.Status.Step {
	case opv1alpha1.ETCDSnapshotSaveStepSave:
		return h.reconcileSave(s, status)
	case opv1alpha1.ETCDSnapshotSaveStepRestart:
		return h.reconcileRestart(s, status)
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

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		saveInstruction := plan.OneTimeInstruction{
			CommonInstruction: plan.CommonInstruction{
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

		nodePlan := &plan.Plan{
			OneTimeInstructions: []plan.OneTimeInstruction{
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
			opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for %s/%s in step %s: %s", secret.Namespace, secret.Name, status.Step, msg))

			return status, nil
		}
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

	for _, secret := range secrets {
		probes, err := s.adapter.RenderProbes(secret, true)
		if err != nil {
			return status, err
		}

		nodePlan := &plan.Plan{
			OneTimeInstructions: []plan.OneTimeInstruction{
				{
					CommonInstruction: plan.CommonInstruction{
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
			opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for %s/%s in step %s: %s", secret.Namespace, secret.Name, status.Step, msg))

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

// handleCanceled (unlike other handler functions) will attempt to release the beacon (if it is
// held by the current object).
// The difference between canceled and failed operations is that an operation fails itself, whereas another controller
// cancels an operation.
// This function mostly serves to execute the planv1alpha1.CanceledPhaseHookLabelPrefix, if it exists
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

	if plan.IsOwningBeaconHolder(s.beacon, ControllerOwnerKey) {
		var err error
		s.beacon, err = plan.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		err = plan.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
		if err != nil {
			return status, err
		}
	}

	return status, nil
}

// handleFailed releases the beacon when the operation has reached the Failed terminal phase, so
// the next operation in line can acquire it. The toggle-off step pairs with handleSucceeded's
// behaviour so the beacon's Active flag accurately reflects whether any operation is currently
// running.
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

	if plan.IsOwningBeaconHolder(s.beacon, ControllerOwnerKey) {
		var err error
		s.beacon, err = plan.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		err = plan.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
		if err != nil {
			return status, err
		}
	}

	return status, nil
}

// handleSucceeded mirrors handleFailed for the Succeeded terminal phase: toggles the beacon off,
// releases ownership, and then nudges the parent cluster controller (snapshotbackpopulate, RKE
// controlplane, etc.) by enqueueing the cluster object so any post-operation reconciliation runs
// promptly rather than waiting for the next periodic resync.
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

	if plan.IsOwningBeaconHolder(s.beacon, ControllerOwnerKey) {
		var err error
		s.beacon, err = plan.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		err = plan.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
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
