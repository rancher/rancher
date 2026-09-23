package etcdsnapshotrestore

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/restoremode"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/condition"
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
	ControllerOwnerKey = "etcd-snapshot-restore"

	Finalizer = "etcdsnapshotrestore.operation.cattle.io"

	// Step hook label prefixes for the etcdsnapshotrestore operation. Each prefix gates a single
	// restore step and follows the shared label semantics documented on planv1alpha1's phase-hook
	// label constants. The shutdown / restore / pod-cleanup / restart / node-cleanup / final-restart
	// sequence is unique to restore — there is no analogue on save / encryption-key-rotation.

	// PreflightStepHookLabelPrefix gates the Preflight step, before the controller performs
	// the necessary preflight checks to determine whether the operation can proceed.
	PreflightStepHookLabelPrefix = "preflight.step.hook.operation.cattle.io/"

	// RestoreClusterConfigStepHookLabelPrefix gates the RestoreClusterConfig step, before the
	// controller writes the cluster configuration captured in the snapshot back onto the object that
	// owns the cluster. Gating here lets a delegate inspect or adjust the configuration a restore is
	// about to apply, while the cluster is still running.
	RestoreClusterConfigStepHookLabelPrefix = "restore-cluster-config.step.hook.operation.cattle.io/"

	// ShutdownStepHookLabelPrefix gates the Shutdown step, before the controller assigns the
	// killall + tombstone-touch + tls/cred-directory cleanup plan to every non-Windows secret.
	ShutdownStepHookLabelPrefix = "shutdown.step.hook.operation.cattle.io/"

	// RestoreStepHookLabelPrefix gates the Restore step, before the controller assigns the
	// `<runtime> server --cluster-reset --cluster-reset-restore-path=...` plan to the elected
	// etcd leader.
	RestoreStepHookLabelPrefix = "restore.step.hook.operation.cattle.io/"

	// PostRestorePodCleanupStepHookLabelPrefix gates the PostRestorePodCleanup step, before the
	// controller starts the server unit on the elected etcd leader and deletes the well-known
	// system pods (kube-dns, CNI, ingress, etc.) that need to be re-created after the restore.
	PostRestorePodCleanupStepHookLabelPrefix = "post-restore-pod-cleanup.step.hook.operation.cattle.io/"

	// InitialRestartClusterStepHookLabelPrefix gates the first cluster restart pass — the one
	// that points every node at the restored leader's server URL before any cluster-wide
	// reconciliation has run. Distinct from the final restart so a delegate can target either
	// pass without gating the other.
	InitialRestartClusterStepHookLabelPrefix = "initial-restart-cluster.step.hook.operation.cattle.io/"

	// PostRestoreNodeCleanupStepHookLabelPrefix gates the PostRestoreNodeCleanup step, before the
	// controller runs the node-pruning script that deletes Node objects that no longer
	// correspond to a machine in the cluster.
	PostRestoreNodeCleanupStepHookLabelPrefix = "post-restore-node-cleanup.step.hook.operation.cattle.io/"

	// RestartClusterStepHookLabelPrefix gates the final restart pass, after node cleanup. This removes the temporary
	// server-URL override and lets each node return to its normal reconciliation.
	RestartClusterStepHookLabelPrefix = "restart-cluster.step.hook.operation.cattle.io/"

	// preflightInstructionName is the name of the Preflight step's instruction, and therefore the key its output is
	// saved under on the machine-plan secret.
	preflightInstructionName = "preflight"

	// TokenHashCommandFormat is the shell command the Preflight step runs on every etcd node to hash the server token
	// persisted on disk, so it can be compared against the hash the distro stamped on the snapshot.
	// The single format argument is the distro data directory, where the token file is persisted.
	//
	// It has to reproduce in a shell what the distro's do which respects colons as part of the password, treating
	// everything after "server:" as the password.
	//
	// So two anchored, non-greedy substitutions recover the password: the first drops the `K10<CA-hash>::` prefix, the
	// second drops the `server:` username. `[^:]*` cannot cross a colon, which is what makes them non-greedy; a greedy
	// `.*:` would strip through the *last* colon. This also respects passwords ending in a colon.
	//
	// Whitespace is only trimmed at the edges, matching the bytes.TrimSpace the distro applies when it
	// reads the same file; deleting all whitespace would corrupt a password that contains a space.
	//
	// The missing/empty guards are not incidental. Every element of the derivation is a pipeline, so without them a
	// missing token file leaves the shell exiting 0 having printed sha256 of the empty string which is a valid hash.
	//
	// Exported so e2e tests can derive the same value the check derives.
	TokenHashCommandFormat = `f=%[1]s/server/token; [ -s "$f" ] || { echo "server token file $f not found or empty" >&2; exit 1; }; p=$(head -n 1 "$f" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^K10[^:]*:://' -e 's/^[^:]*://'); [ -n "$p" ] || { echo "no server token password found in $f" >&2; exit 1; }; printf '%%s' "$p" | sha256sum | cut -c1-12`

	// idempotencyKey is the top-level key used to scope idempotency tracking for this controller.
	// It is also used by the cleanup instruction issued during shutdown to clear prior tracking.
	idempotencyKey = "etcd-restore"

	// etcdRestoreBinSubdir is the relative path (under the distro data directory) that holds the
	// helper scripts the controller writes to nodes during the restore.
	etcdRestoreBinSubdir = "etcd-restore/bin"

	// waitForPodListScriptName is the name of the script file containing the instruction to list pods in all namespaces.
	waitForPodListScriptName = "wait_for_pod_list.sh"

	// waitForPodListScript is the bash script for listing pods in all namespaces.
	// Although the script itself accepts a template to execute, it is only used for listing pods.
	waitForPodListScript = `#!/bin/sh

i=0

while [ $i -lt 30 ]; do
	if $@ >/dev/null 2>&1; then
		exit 0
	fi
	sleep 10
	i=$((i + 1))
done
exit 1
`

	// nodeCleanupScriptName is the name of the script file containing the instruction to clean up nodes no longer
	// present after restoring.
	nodeCleanupScriptName = "clean_up_nodes.sh"

	// nodeCleanupScript is the bash script for cleaning up nodes no longer present after restoring.
	nodeCleanupScript = `#!/bin/sh

if [ -z "$KUBECTL" ]; then
        echo "Must define KUBECTL environment variable"
        exit 1
fi

if [ -z "$KUBECONFIG" ]; then
        echo "Must define KUBECONFIG environment variable"
        exit 1
fi

NODENAMESFILE="$1"

if [ -z "$NODENAMESFILE" ]; then
        echo "Must define nodenames file"
        exit 1
fi

TMPALLNODES=$(mktemp)

if ! ${KUBECTL} get nodes --no-headers -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' > "$TMPALLNODES"; then
        echo "Error listing all nodes"
        rm "$TMPALLNODES"
        exit 1
fi

echo "Saving nodes:"
cat "$NODENAMESFILE"

while IFS='' read -r NODE; do
        if [ "${NODE}" = "" ]; then
                continue
        fi
        FOUND=false
        while IFS='' read -r KEEP; do
                if [ "${NODE}" = "${KEEP}" ]; then
                        FOUND=true
                        break
                fi
        done < "$NODENAMESFILE"
        if [ "${FOUND}" != "true" ]; then
                echo "Deleting node ${NODE}"
                ${KUBECTL} delete node "${NODE}" --wait=false
        fi
done < "$TMPALLNODES"

rm "$TMPALLNODES"
rm "$NODENAMESFILE"
`
)

// dynamicResolver is the subset of *dynamic.Controller this handler needs: Get for cluster
// lookup during phase dispatch, and Enqueue for nudging the parent cluster controller after a
// successful operation. It's an interface so tests can substitute a stub — *dynamic.Controller
// satisfies it directly.
type dynamicResolver interface {
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
	Enqueue(gvk schema.GroupVersionKind, namespace, name string) error
}

type handler struct {
	etcdsnapshotrestores operationcontrollers.ETCDSnapshotRestoreController
	etcdsnapshots        rkecontrollers.ETCDSnapshotController

	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache

	secrets     corecontrollers.SecretClient
	secretCache corecontrollers.SecretCache

	dynamic dynamicResolver

	clients *wrangler.CAPIContext

	store *plan.Store
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		etcdsnapshotrestores: clients.Operation.ETCDSnapshotRestore(),
		etcdsnapshots:        clients.RKE.ETCDSnapshot(),
		beacons:              clients.Plan.Beacon(),
		beaconCache:          clients.Plan.Beacon().Cache(),
		secrets:              clients.Core.Secret(),
		secretCache:          clients.Core.Secret().Cache(),
		dynamic:              clients.Dynamic,
		clients:              clients,
		store:                plan.NewStore(clients.Core.Secret()),
	}

	operationcontrollers.RegisterETCDSnapshotRestoreStatusHandler(ctx, clients.Operation.ETCDSnapshotRestore(), "", "etcd-snapshot-restore-handler", h.OnChange)
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
func (h *handler) OnChange(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	if op == nil {
		return status, nil
	}

	// Pausing an operation stops the controller touching it at all: no plans are dispatched, no
	// beacon is acquired or released, and no finalizer is taken. That holds for a deleting operation
	// too — releasing its beacon is reconciliation like any other — so an operation already carrying
	// the finalizer when it was paused will not finish deleting until it is resumed.
	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: skipping paused operation", op.Namespace, op.Name)

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
func (h *handler) reconcileActive(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
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

	if collectable(op, &status) {
		if err := h.etcdsnapshotrestores.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{}); err != nil {
			return status, err
		}

		// The operation is on its way out, so the status computed for it is moot.
		return status, generic.ErrSkip
	}

	// Nothing moved, so poll: plan secret state, beacon transitions and the TTL falling due are all
	// changes this controller will not otherwise be told about.
	h.etcdsnapshotrestores.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)

	return status, nil
}

// reconcileDeleting drives an operation which is being deleted. The deletion is held up by our
// finalizer until terminal handling has been recorded as complete, which keeps the operation — and
// with it any beacon delegation made on its behalf — alive while a terminal phase hook delegate
// finishes its work. The terminal status is also persisted before the finalizer is dropped, so an
// observer waiting on the final phase gets to see it rather than the object simply vanishing.
func (h *handler) reconcileDeleting(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	// Teardown is driven off our finalizer. Without it the operation has either already been torn
	// down or was never ours to begin with, and is on its way out under someone else's control.
	if !hasFinalizer(op) {
		return updateStatus(op, status), nil
	}

	// cancelForDeletion always leaves a phase behind, so unlike reconcileActive there is no unset
	// phase to default here.
	status, err := h.advance(op, cancelForDeletion(op, status))
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
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: deferring deletion, terminal handling has not completed", op.Namespace, op.Name)
		h.etcdsnapshotrestores.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: terminal handling complete, releasing operation for deletion", op.Namespace, op.Name)

	return status, h.removeFinalizer(op)
}

// advance resolves everything the phase handlers work from and runs the handler for the operation's
// current phase.
//
// A nil scope from resolveScope means the reconcile has already settled for this tick — the cluster
// or the beacon is gone, or a deleting operation has nothing left to release — and the status it
// returned is what should be reported.
func (h *handler) advance(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	s, status, err := h.resolveScope(op, status)
	if err != nil || s == nil {
		return status, err
	}

	return h.dispatchPhase(s, status)
}

// collectable reports whether a settled operation can be garbage collected: it reached a terminal
// phase, the controller finished handling that phase, its TTL has elapsed, and no lifecycle hook
// delegate is still expected to look at it.
//
// The last two conditions are what stop an operation waiting on a terminal phase hook from being
// deleted the moment its TTL passes, which would strand the delegate holding its beacon and bury
// the phase it actually finished in behind the cancellation that deletion performs.
func collectable(op *opv1alpha1.ETCDSnapshotRestore, status *opv1alpha1.ETCDSnapshotRestoreStatus) bool {
	return ops.IsTerminal(status.Phase) &&
		ops.IsTerminated(&status.OperationStatus) &&
		ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) &&
		!planv1alpha1.HasActiveLifecycleHook(op)
}
// hasFinalizer reports whether the operation still carries our finalizer, i.e. whether its teardown
// is ours to drive.
func hasFinalizer(op *opv1alpha1.ETCDSnapshotRestore) bool {
	return slices.Contains(op.Finalizers, Finalizer)
}

// ensureFinalizer adds our finalizer to the operation if it is not already present. The status
// handler only ever persists status, so the finalizer has to be written with an explicit Update;
// the updated object is copied back over op so the resource version the status handler goes on to
// use for its own UpdateStatus is not stale.
func (h *handler) ensureFinalizer(op *opv1alpha1.ETCDSnapshotRestore) error {
	if hasFinalizer(op) {
		return nil
	}

	logrus.Debugf("[etcdsnapshotrestore] %s/%s: adding finalizer", op.Namespace, op.Name)

	updated := op.DeepCopy()
	updated.Finalizers = append(updated.Finalizers, Finalizer)

	updated, err := h.etcdsnapshotrestores.Update(updated)
	if err != nil {
		return err
	}

	*op = *updated

	return nil
}

// removeFinalizer drops our finalizer from the operation, which lets the API server complete the
// deletion. A NotFound is treated as success: something else (another finalizer holder finishing
// last, or a previous attempt whose response was lost) already let the object go.
func (h *handler) removeFinalizer(op *opv1alpha1.ETCDSnapshotRestore) error {
	if !hasFinalizer(op) {
		return nil
	}

	updated := op.DeepCopy()
	updated.Finalizers = slices.DeleteFunc(updated.Finalizers, func(f string) bool {
		return f == Finalizer
	})

	updated, err := h.etcdsnapshotrestores.Update(updated)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	*op = *updated

	return nil
}
// cancelForDeletion marks an operation deleted before its terminal handling completed as Canceled:
// the work it dispatched is no longer tracked by anything, so it can neither be reported as
// succeeded nor as failed. The terminal handler for the Canceled phase then runs as usual —
// honoring any canceled phase hook, releasing the beacon — before OnChange drops the finalizer.
//
// Note that cancelling a restore does not roll it back: whatever the restore already did to the
// cluster (shut down server units, reset etcd on the leader) stays done. Cancellation only records
// that the operation stopped being driven, and frees the beacon for whoever picks up the pieces.
//
// An operation which is already terminated keeps the phase it finished in; only the window before
// that counts as racing the operation, and in that window its beacon is still held, on its own
// behalf or a delegate's. An operation already in Canceled keeps the reason it was canceled for.
func cancelForDeletion(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) opv1alpha1.ETCDSnapshotRestoreStatus {
	if ops.IsTerminated(&status.OperationStatus) || status.Phase == opv1alpha1.OperationPhaseCanceled {
		return status
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: marking operation as canceled: deleted in phase [%s] step [%s] before terminal handling completed", op.Namespace, op.Name, status.Phase, status.Step)

	markCanceled(&status, opv1alpha1.OperationDeletedReason, "operation deleted before terminal handling completed")

	return status
}

// resolveScope gathers everything the phase handlers work from: the parent cluster, the Adapter for
// its kind, and the cluster's beacon.
//
// A nil scope returned with a nil error means the reconcile has settled for this tick and the
// returned status is what should be reported — the cluster is missing, the beacon has not been
// created yet, or the operation is deleting and has nothing left to release.
func (h *handler) resolveScope(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (*scope, opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	deleting := op.DeletionTimestamp != nil

	gvk := schema.FromAPIVersionAndKind(op.Spec.ClusterRef.APIVersion, op.Spec.ClusterRef.Kind)
	ref, err := h.dynamic.Get(gvk, op.Spec.ClusterRef.Namespace, op.Spec.ClusterRef.Name)
	if apierrors.IsNotFound(err) {
		key := opv1alpha1.ClusterRefKey(op.Spec.ClusterRef)

		// The beacon lives alongside the cluster, so a deleted operation whose cluster is gone has
		// nothing left to release: terminal handling is trivially complete and the operation is
		// free to finish deleting. Failing it here instead would both overwrite the Canceled phase
		// and, for a cluster deleted mid-operation, wedge the deletion behind our finalizer.
		if deleting {
			logrus.Infof("[etcdsnapshotrestore] %s/%s: cluster %s is gone, nothing to release", op.Namespace, op.Name, key)
			status.SetTerminated()
			return nil, status, nil
		}

		logrus.Errorf("[etcdsnapshotrestore]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

		markFailed(&status, opv1alpha1.ClusterNotFoundReason, fmt.Sprintf("cluster %s not found", key))

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

	a, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return nil, status, err
	}

	clusterObj, err := a.ClusterObject()
	if err != nil {
		return nil, status, err
	}

	// Resolve the beacon (and every other cluster-scoped artifact — plan secrets, snapshot CRs)
	// via the adapter, not the op.Spec.ClusterRef. When the UI creates ops against the mgmt v3
	// Cluster, ClusterRef points there but the real state lives in the underlying provisioner's
	// namespace (fleet-default for v2prov, CAPI ns for CAPRKE2). BeaconRef() gives the correct
	// (namespace, name) for each adapter type.
	namespace, beaconName := a.BeaconRef()

	beacon, err := h.beacons.Get(namespace, beaconName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && deleting {
		// As above: no beacon means nothing to release, so let the deletion proceed rather than
		// requeueing a NotFound forever.
		logrus.Infof("[etcdsnapshotrestore] %s/%s: beacon %s/%s is gone, nothing to release", op.Namespace, op.Name, namespace, beaconName)
		status.SetTerminated()
		return nil, status, nil
	} else if apierrors.IsNotFound(err) && status.Phase == opv1alpha1.OperationPhasePending {
		logrus.Warnf("[etcdsnapshotrestore]: %s/%s failed to find beacon %s/%s (clusterRef apiVersion=%s kind=%s name=%s)",
			op.Namespace, op.Name, namespace, beaconName, ustr.GetAPIVersion(), ustr.GetKind(), ustr.GetName())

		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForBeaconReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")

		return nil, status, nil
	} else if err != nil {
		return nil, status, err
	}

	return &scope{
		ownerKey:   plan.ControllerOwnerKey(op, ControllerOwnerKey),
		op:         op,
		beacon:     beacon,
		namespace:  namespace,
		clusterObj: clusterObj,
		adapter:    a,
	}, status, nil
}

// dispatchPhase routes the operation to the handler for its current phase. An unrecognized phase is
// itself terminal: the controller cannot know what the operation was doing, so it fails it.
func (h *handler) dispatchPhase(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
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

	markFailed(&status, opv1alpha1.UnknownPhaseReason, fmt.Sprintf("unknown phase [%s]", s.op.Status.Phase))

	return status, nil
}

type scope struct {
	ownerKey string

	op        *opv1alpha1.ETCDSnapshotRestore
	namespace string

	beacon     *planv1alpha1.Beacon
	clusterObj *unstructured.Unstructured
	adapter    ops.Adapter
}

// idempotencyValue returns the value the idempotency tracker hashes to determine whether to re-run
// a given instruction. The UID of the operation never changes for a single CR, so all instructions
// associated with the same restore CR run exactly once.
func (s *scope) idempotencyValue() string {
	return string(s.op.UID)
}

// lifecycleHookDelegate returns (suffix, delegate) for the first label on the operation whose key
// starts with prefix. Returns ("", "") when no such label is set. The suffix is informational —
// only the delegate value is consulted to drive the beacon push.
func lifecycleHookDelegate(op *opv1alpha1.ETCDSnapshotRestore, prefix string) (string, string) {
	// An empty prefix would match every label, and so would report a delegate for a phase that has
	// no hook at all.
	if prefix == "" || op.Labels == nil {
		return "", ""
	}

	for k, v := range op.Labels {
		if after, ok := strings.CutPrefix(k, prefix); ok {
			return after, v
		}
	}

	return "", ""
}

// delegate pushes delegate onto the beacon's delegate chain if it is not already there. It is a
// no-op when the delegate is already present, which keeps the call idempotent across the many
// reconciles that may occur while a hook is held.
func (h *handler) delegate(s *scope, name, delegate string) error {
	logrus.Tracef("[etcdsnapshotrestore] %s/%s: delegating ownership of beacon to %s on behalf of %s", s.op.Namespace, s.op.Name, delegate, name)

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

// handleHook is the per-handler entry point for the lifecycle-hook mechanism. It returns (true, nil)
// whenever a label with the given prefix exists on the operation, signaling the caller to short
// circuit. To advance past the hook the operator must clear the label AND pop the delegate; either
// alone is insufficient because:
//
//   - Clearing the label but leaving the delegate on the chain lets the beacon's authority logic
//     still report the delegate as the holder, so the owning controller may not regain its
//     authority on the next reconcile.
//   - Popping the delegate but leaving the label present causes this function to re-push the
//     delegate on the very next reconcile (delegate() is no-op only if already in chain).
func (h *handler) handleHook(s *scope, prefix string) (bool, error) {
	logrus.Tracef("[etcdsnapshotrestore] %s/%s: checking lifecycle hook for prefix %q", s.op.Namespace, s.op.Name, prefix)

	if name, delegate := lifecycleHookDelegate(s.op, prefix); delegate != "" {
		err := h.delegate(s, name, delegate)
		return true, err
	}
	return false, nil
}

// restartClusterHookPrefix picks between the InitialRestart and Restart prefixes based on which
// step the operation is currently in. reconcileRestartCluster is reused for both restart phases,
// so we route the hook lookup with the same step-based dispatch the caller uses.
func restartClusterHookPrefix(step opv1alpha1.ETCDSnapshotRestoreStep) string {
	if step == opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster {
		return InitialRestartClusterStepHookLabelPrefix
	}
	return RestartClusterStepHookLabelPrefix
}

// stepHookPrefixFor returns the step-hook label prefix for the given restore step, or "" for an
// unknown / empty step. Used by handleInProgress to decide whether beacon-authorization loss is
// explained by an active step-scoped delegation vs a genuine loss.
func stepHookPrefixFor(step opv1alpha1.ETCDSnapshotRestoreStep) string {
	switch step {
	case opv1alpha1.ETCDSnapshotRestoreStepPreflight:
		return PreflightStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig:
		return RestoreClusterConfigStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotRestoreStepShutdown:
		return ShutdownStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotRestoreStepRestore:
		return RestoreStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup:
		return PostRestorePodCleanupStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster:
		return InitialRestartClusterStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup:
		return PostRestoreNodeCleanupStepHookLabelPrefix
	case opv1alpha1.ETCDSnapshotRestoreStepRestartCluster:
		return RestartClusterStepHookLabelPrefix
	}
	return ""
}

// etcdRestoreScriptPath returns the absolute path on the node where the named etcd-restore script lives.
func etcdRestoreScriptPath(s *scope, secret *corev1.Secret, name string) string {
	return path.Join(s.adapter.ProvisioningDataDirectory(secret), etcdRestoreBinSubdir, name)
}

// nonWindowsSecret returns true for any secret whose cattle.io/os label is not "windows". Imported
// clusters do not set this label at all; treating absent label as non-Windows keeps the shutdown
// and restart paths from no-oping on them.
func nonWindowsSecret(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels[capr.CattleOSLabel] != capr.WindowsMachineOS
}

func (h *handler) handlePending(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
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
			opv1alpha1.PendingCondition.Message(&status, "waiting for beacon creation")
			return status, nil
		}
		s.beacon = acquired
	}

	// The Pending-phase hook fires after the beacon has been acquired so external delegates can
	// inspect the cluster (machine-plan secrets, beacon ownership) before the controller starts
	// the actual restore workflow.
	delegated, err := h.handleHook(s, planv1alpha1.PendingPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.PendingCondition, &status, s.beacon)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: acquired beacon, waiting for agents to register", s.op.Namespace, s.op.Name)

	if ok, err := s.adapter.WaitForRegister(); err != nil {
		return status, err
	} else if !ok {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for system-agents to connect", s.op.Namespace, s.op.Name)

		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForRegistrationReason)
		opv1alpha1.PendingCondition.Message(&status, "waiting for system-agents to connect")

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to shutdown", s.op.Namespace, s.op.Name)

	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepPreflight)

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)

	return status, nil
}

func (h *handler) handleInProgress(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	stepPrefix := stepHookPrefixFor(s.op.Status.Step)

	// Stage 1 (loose): the op must appear SOMEWHERE in the ownership chain (owner or any
	// delegate). Being absent entirely means the beacon was reassigned to another controller, and
	// we can't recover. If a step hook is currently active on the op, treat the absence as a
	// step-scoped delegation and surface WaitingForDelegate instead of failing — the delegate may
	// have popped us in service of the hook and will restore ownership when the hook clears.
	if !plan.IsOwningBeaconHolder(s.beacon, s.ownerKey) && !plan.IsInDelegateChain(s.beacon, s.ownerKey) {
		if planv1alpha1.HasStepHookLabel(s.op, stepPrefix) {
			setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
			return status, nil
		}
		markFailed(&status, opv1alpha1.BeaconLostReason, "beacon reassigned, aborting")

		return status, nil
	}

	var err error
	s.beacon, err = plan.ToggleBeacon(s.beacon, true, h.beacons)
	if err != nil {
		return status, err
	}

	// InProgress-phase hook fires on every InProgress reconcile, ahead of step dispatch. This is
	// the broadest hook in the restore lifecycle — useful for delegates that need to gate ALL
	// step work uniformly without subscribing to each individual step prefix.
	delegated, err := h.handleHook(s, planv1alpha1.InProgressPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
		return status, nil
	}

	// Stage 2 (strict): after the InProgress-phase hook has been handled, the op must be the
	// primary owner or the most-recent delegate on the chain to drive step work. If a step hook
	// is still active on the op, treat the missing-top state as an intentional delegation and
	// wait; otherwise this is a genuine beacon loss and we fail.
	if !plan.AuthorizedForBeacon(s.beacon, s.ownerKey) {
		if planv1alpha1.HasStepHookLabel(s.op, stepPrefix) {
			setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
			return status, nil
		}
		markFailed(&status, opv1alpha1.BeaconLostReason, "Beacon acquired by another controller, aborting")

		return status, nil
	}

	switch s.op.Status.Step {
	case opv1alpha1.ETCDSnapshotRestoreStepPreflight:
		return h.reconcilePreflight(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig:
		return h.reconcileRestoreClusterConfig(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepShutdown:
		return h.reconcileShutdown(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepRestore:
		return h.reconcileRestore(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup:
		return h.reconcilePostRestorePodCleanup(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster:
		return h.reconcileRestartCluster(s, status, opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup)
	case opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup:
		return h.reconcilePostRestoreNodeCleanup(s, status)
	case opv1alpha1.ETCDSnapshotRestoreStepRestartCluster:
		return h.reconcileRestartCluster(s, status, "")
	}

	status.SetPhase(opv1alpha1.OperationPhaseFailed)

	markFailed(&status, opv1alpha1.UnknownStepReason, fmt.Sprintf("current step [\"%s\"] is unknown, expected one of: [\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]",
		status.Step,
		opv1alpha1.ETCDSnapshotRestoreStepPreflight,
		opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		opv1alpha1.ETCDSnapshotRestoreStepShutdown,
		opv1alpha1.ETCDSnapshotRestoreStepRestore,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepRestartCluster))

	return status, nil
}

func (h *handler) reconcilePreflight(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling preflight", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, PreflightStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
		return status, nil
	}

	secrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(plan.DefaultSorter()).
		WithFilter(ops.IsEtcd).
		WithValidator(plan.AtLeast(1, "")).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: aborting operation: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markAborted(&status, opv1alpha1.PreflightCheckFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	concurrency := len(secrets)
	results := make([]plan.PlanStatus, 0, concurrency)

	snapshotName := s.op.Spec.Args.Name
	snapshot, err := h.etcdsnapshots.Get(s.adapter.EtcdSnapshotNamespace(), snapshotName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: could not find associated etcdsnapshot.rke.cattle.io %s/%s, assuming snapshot file", s.op.Namespace, s.op.Name, s.adapter.EtcdSnapshotNamespace(), snapshotName)
		snapshot = nil
	} else if err != nil {
		return status, err
	}

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)

	hash := ""
	if snapshot != nil && snapshot.Annotations != nil && snapshot.Annotations[capr.SnapshotTokenHashAnnotation] != "" {
		hash = snapshot.Annotations[capr.SnapshotTokenHashAnnotation]
	}

	if hash == "" {
		logrus.Warnf("[etcdsnapshotrestore] %s/%s: could not find snapshot token hash in snapshot %s/%s, skipping preflight step", s.op.Namespace, s.op.Name, s.adapter.EtcdSnapshotNamespace(), snapshotName)
	} else {
		for _, secret := range secrets {
			nodePlan, err := buildPreflightPlan(s, secret)
			if err != nil {
				return status, err
			}
			planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
			if err != nil {
				return status, err
			}

			results = append(results, *planStatus)

			if planStatus.Failure() {
				logrus.Errorf("[etcdsnapshotrestore] %s/%s: aborting operation: preflight check failed for %s/%s",
					s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

				markAborted(&status, opv1alpha1.PreflightCheckFailedReason, fmt.Sprintf("could not find server token for %s/%s", secret.Namespace, secret.Name))

				return status, nil
			}

			// The token hash can only be validated once the node has applied the plan and reported its
			// output, so a waiting node is skipped entirely until a later reconcile.
			if planStatus.Waiting() {
				logrus.Debugf("[etcdsnapshotrestore] %s/%s: waiting for preflight check for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

				concurrency--
				if concurrency <= 0 {
					break
				}

				continue
			}

			output, err := plan.ReadAppliedOutput(secret)
			if err != nil {
				logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: could not read preflight check output for %s/%s",
					s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

				markFailed(&status, opv1alpha1.PreflightCheckFailedReason, fmt.Sprintf("could not read preflight check output for %s/%s", secret.Namespace, secret.Name))

				return status, nil
			}

			// The instruction pipes through cut, so the saved output carries a trailing newline which the annotation
			// does not.
			if b, ok := output[preflightInstructionName]; ok && strings.TrimSpace(string(b)) != hash {
				logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: preflight check output for %s/%s does not match snapshot token hash",
					s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

				markFailed(&status, opv1alpha1.PreflightCheckFailedReason, fmt.Sprintf("preflight check output for %s/%s does not match snapshot token hash", secret.Namespace, secret.Name))

				return status, nil
			}
		}
	}

	if concurrency < len(secrets) {
		setWaitingForPlan(&status, results)

		return status, nil
	}

	// Validate the requested restore mode while the cluster is still running, so an unavailable mode
	// cancels the operation before anything is shut down.
	if reason, ok := validateRestoreMode(s.op.Spec.Args.RestoreMode, snapshotName, snapshot); !ok {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as canceled: %s", s.op.Namespace, s.op.Name, reason)

		status.SetPhase(opv1alpha1.OperationPhaseCanceled)

		opv1alpha1.CanceledCondition.True(&status)
		opv1alpha1.CanceledCondition.Reason(&status, opv1alpha1.PreflightCheckFailedReason)
		opv1alpha1.CanceledCondition.Message(&status, reason)

		return status, nil
	}

	if err = s.adapter.PauseCluster(true); err != nil {
		return status, err
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to restore cluster config", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig)
	return status, nil
}

// restoresClusterConfig reports whether mode asks for any cluster configuration to be restored. An
// empty mode and "none" both mean "etcd only".
func restoresClusterConfig(mode string) bool {
	return mode != "" && mode != rkev1.RestoreRKEConfigNone
}

// validateRestoreMode validates a requested restore mode against the modes the snapshot advertises,
// returning (reason, false) when the mode cannot be honored. snapshotbackpopulate computes the
// annotation by resolving each mode's selector against the resources the snapshot captured, so a
// mode listed there is one this snapshot can actually satisfy.
func validateRestoreMode(mode, snapshotName string, snapshot *rkev1.ETCDSnapshot) (string, bool) {
	if !restoresClusterConfig(mode) {
		return "", true
	}

	// The caller tolerates a missing snapshot CR by treating Args.Name as a bare file on disk. That
	// works for restoring etcd, but there is then nothing to read a cluster configuration from.
	if snapshot == nil {
		return fmt.Sprintf("restore mode %q requires an etcdsnapshot.rke.cattle.io resource, but none exists for snapshot %q", mode, snapshotName), false
	}

	available := restoremode.AvailableModes(snapshot.Annotations[capr.RestoreModeOptionsAnnotation])
	if !slices.Contains(available, mode) {
		return fmt.Sprintf("restore mode %q is not available for snapshot %s/%s, which offers [%s]",
			mode, snapshot.Namespace, snapshot.Name, strings.Join(available, ", ")), false
	}

	return "", true
}

// reconcileRestoreClusterConfig applies the requested restore mode: it writes the fields the mode
// selects, as captured in the snapshot's metadata, back onto the object that owns the cluster.
//
// This runs before Shutdown, matching the legacy ordering where the cluster spec was updated before
// the restore began. It is also why the Restore step can install the right Kubernetes version: the
// configuration is in place, and propagated, before any node plan is built from it.
func (h *handler) reconcileRestoreClusterConfig(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling restore cluster config", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, RestoreClusterConfigStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	mode := s.op.Spec.Args.RestoreMode
	if !restoresClusterConfig(mode) {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: restore mode %q restores no cluster configuration, transitioning to shutdown", s.op.Namespace, s.op.Name, mode)

		status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepShutdown)
		return status, nil
	}

	matches, reason, err := h.resolveRestoreMode(s, mode)
	if err != nil {
		return status, err
	}
	if reason != "" {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: %s", s.op.Namespace, s.op.Name, reason)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.FailedReason)
		opv1alpha1.FailedCondition.Message(&status, reason)

		return status, nil
	}

	applied, reason, err := h.applyRestoreMode(s, matches)
	if err != nil {
		return status, err
	}
	if reason != "" {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as canceled: %s", s.op.Namespace, s.op.Name, reason)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.FailedReason)
		opv1alpha1.FailedCondition.Message(&status, reason)

		return status, nil
	}

	// An update was issued, so the caches this reconcile read are stale. Come back and re-resolve;
	// once every field already holds its restored value nothing is applied and we advance.
	if applied {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting in step %s: applying restore mode %q", status.Step, mode))

		return status, nil
	}

	// The restored configuration is on the restore target, but the objects rendered off it may not
	// have caught up. Later steps build node plans from those — the Restore step installs the
	// Kubernetes version they carry — so hold here until they are current rather than racing ahead
	// and restoring with the pre-restore configuration.
	settled, err := s.adapter.WaitForRestoreTarget()
	if err != nil {
		return status, err
	}
	if !settled {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for the cluster to observe the restored configuration", s.op.Namespace, s.op.Name)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting in step %s: waiting for the cluster to observe the restored configuration", status.Step))

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: applied restore mode %q, transitioning to shutdown", s.op.Namespace, s.op.Name, mode)

	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepShutdown)
	return status, nil
}

// resolveRestoreMode reads the snapshot's metadata and returns the fields mode selects that Rancher
// permits restoring. A non-empty reason means the operation cannot proceed.
func (h *handler) resolveRestoreMode(s *scope, mode string) ([]restoremode.Match, string, error) {
	snapshot, err := h.etcdsnapshots.Get(s.adapter.EtcdSnapshotNamespace(), s.op.Spec.Args.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, fmt.Sprintf("restore mode %q requires an etcdsnapshot.rke.cattle.io resource, but %s/%s does not exist",
			mode, s.adapter.EtcdSnapshotNamespace(), s.op.Spec.Args.Name), nil
	} else if err != nil {
		return nil, "", err
	}

	metadata, err := snapshotutil.SnapshotMetadata(snapshot)
	if err != nil {
		return nil, fmt.Sprintf("cannot apply restore mode %q: reading metadata of snapshot %s/%s: %v",
			mode, snapshot.Namespace, snapshot.Name, err), nil
	}

	modes, err := restoremode.Modes(metadata)
	if err != nil {
		return nil, fmt.Sprintf("cannot apply restore mode %q for snapshot %s/%s: %v", mode, snapshot.Namespace, snapshot.Name, err), nil
	}

	selector, ok := modes[mode]
	if !ok {
		return nil, fmt.Sprintf("snapshot %s/%s does not declare restore mode %q", snapshot.Namespace, snapshot.Name, mode), nil
	}

	resources, err := restoremode.Resources(metadata)
	if err != nil {
		return nil, fmt.Sprintf("cannot apply restore mode %q for snapshot %s/%s: %v", mode, snapshot.Namespace, snapshot.Name, err), nil
	}

	resolved, err := restoremode.Resolve(selector, resources)
	if err != nil {
		return nil, fmt.Sprintf("restore mode %q of snapshot %s/%s has an unusable selector %q: %v",
			mode, snapshot.Namespace, snapshot.Name, selector, err), nil
	}

	allowed, denied := restoremode.Writable(resolved)
	for _, d := range denied {
		logrus.Warnf("[etcdsnapshotrestore] %s/%s: restore mode %q selects %s, which Rancher does not restore; skipping it",
			s.op.Namespace, s.op.Name, mode, d)
	}

	if len(allowed) == 0 {
		return nil, fmt.Sprintf("restore mode %q of snapshot %s/%s selects no field that Rancher restores", mode, snapshot.Namespace, snapshot.Name), nil
	}

	return allowed, "", nil
}

// applyRestoreMode writes matches onto the live objects their resource keys address, returning
// whether anything was updated. A non-empty reason means the operation cannot proceed.
//
// Reporting "nothing was updated" is what lets the restore-mode step finish, so the comparisons
// below have to be able to see a field that already holds its restored value. That relies on both
// sides using the same Go types for the same JSON: the live object supplies int64 for integral
// numbers and restoremode.Match carries them the same way, because snapshotutil.DecompressInterface
// decodes the snapshot payload with the unstructured number convention.
func (h *handler) applyRestoreMode(s *scope, matches []restoremode.Match) (bool, string, error) {
	byResource := map[string][]restoremode.Match{}
	for _, m := range matches {
		byResource[m.ResourceKey] = append(byResource[m.ResourceKey], m)
	}

	applied := false
	for key, resourceMatches := range byResource {
		target, err := s.adapter.RestoreTarget(key)
		if err != nil {
			return false, "", err
		}
		if target == nil {
			return false, fmt.Sprintf("restore mode selects %s, which this cluster has no counterpart for", key), nil
		}

		updated := target.DeepCopy()
		for _, m := range resourceMatches {
			current, found, err := unstructured.NestedFieldNoCopy(updated.Object, m.Path...)
			if err != nil {
				return false, fmt.Sprintf("reading %s from %s: %v", m, key, err), nil
			}
			if found && equality.Semantic.DeepEqual(current, m.Value) {
				continue
			}
			if err := unstructured.SetNestedField(updated.Object, m.Value, m.Path...); err != nil {
				return false, fmt.Sprintf("restoring %s onto %s: %v", m, key, err), nil
			}
			logrus.Infof("[etcdsnapshotrestore] %s/%s: restoring %s on %s %s/%s",
				s.op.Namespace, s.op.Name, m, updated.GetKind(), updated.GetNamespace(), updated.GetName())
		}

		if equality.Semantic.DeepEqual(target.Object, updated.Object) {
			continue
		}

		if err := s.adapter.UpdateRestoreTarget(updated); err != nil {
			return false, "", err
		}
		applied = true
	}

	return applied, "", nil
}

func (h *handler) reconcileShutdown(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling shutdown", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, ShutdownStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
		return status, nil
	}

	secrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(plan.DefaultSorter()).
		WithFilter(nonWindowsSecret).
		WithValidator(plan.AtLeast(1, "")).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	concurrency := len(secrets)
	results := make([]plan.PlanStatus, 0, concurrency)

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)

	for _, secret := range secrets {
		nodePlan, err := buildShutdownPlan(s, secret)
		if err != nil {
			return status, err
		}
		planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
		if err != nil {
			return status, err
		}

		results = append(results, *planStatus)

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: shutdown failed for %s/%s",
				s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("shutdown failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if planStatus.Waiting() {
			logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for shutdown for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			concurrency--
			if concurrency <= 0 {
				break
			}
		}
	}

	if concurrency < len(secrets) {
		setWaitingForPlan(&status, results)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to restore", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestore)
	return status, nil
}

func (h *handler) reconcileRestore(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling etcd restore", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, RestoreStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
		return status, nil
	}

	snapshotName := s.op.Spec.Args.Name
	if snapshotName == "" {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: snapshot name is required for etcd restore", s.op.Namespace, s.op.Name)

		markFailed(&status, opv1alpha1.PlanFailedReason, "snapshot name is required for etcd restore")

		return status, nil
	}

	filter := ops.IsEtcd
	snapshot, err := h.etcdsnapshots.Get(s.adapter.EtcdSnapshotNamespace(), snapshotName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: could not find associated etcdsnapshot.rke.cattle.io %s/%s, assuming snapshot file", s.op.Namespace, s.op.Name, s.adapter.EtcdSnapshotNamespace(), snapshotName)
		snapshot = nil
	} else if err != nil {
		return status, err
	} else if snapshot != nil && snapshot.SnapshotFile.S3 == nil {
		// Prefer the snapshot's stamped MachineLifecycleNameLabel (used by CAPRKE2 where the
		// owner ref is a mgmt v3 Node but plan secrets are labeled with the CAPI Machine's
		// name). Fall back to OwnerReferences[0].Name for v2prov/imported paths that predate the
		// label.
		machineName := snapshot.Labels[planv1alpha1.MachineLifecycleNameLabel]
		if machineName == "" && len(snapshot.OwnerReferences) > 0 {
			machineName = snapshot.OwnerReferences[0].Name
		}
		if machineName == "" {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: cannot correlate machine for snapshot %s/%s (no lifecycle label, no owner reference)", s.op.Namespace, s.op.Name, snapshot.Namespace, snapshot.Name)

			markFailed(&status, opv1alpha1.PlanFailedReason, "machine correlation is required for local etcd restore")

			return status, nil
		}

		filter = func(secret *corev1.Secret) bool {
			if secret == nil || secret.Labels == nil {
				return false
			}
			return secret.Labels[planv1alpha1.MachineLifecycleNameLabel] == machineName
		}
	}

	secret, err := s.adapter.FindOrElectLeader(s.ownerKey, filter)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))

		return status, nil
	} else if secret == nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: no eligible etcd leader for restore", s.op.Namespace, s.op.Name)

		markFailed(&status, opv1alpha1.PlanFailedReason, "no eligible etcd leader for restore")

		return status, nil
	}

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)

	nodePlan, err := buildRestorePlan(s, secret, snapshot, snapshotName)
	if err != nil {
		return status, err
	}

	planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: etcd restore failed for %s/%s",
			s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("etcd restore failed for %s/%s", secret.Namespace, secret.Name))

		return status, nil
	}

	if planStatus.Waiting() {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: waiting for etcd restore for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

		setWaitingForSinglePlan(&status, planStatus)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to post-restore pod cleanup", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup)
	return status, nil
}

func (h *handler) reconcilePostRestorePodCleanup(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling post-restore pod cleanup", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, PostRestorePodCleanupStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
		return status, nil
	}

	etcdSecret, err := s.adapter.FindOrElectLeader(s.ownerKey, nil)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))

		return status, nil
	} else if etcdSecret == nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: no eligible etcd leader for restore", s.op.Namespace, s.op.Name)

		markFailed(&status, opv1alpha1.PlanFailedReason, "no eligible etcd leader for restore")

		return status, nil
	}

	var controlPlaneSecret *corev1.Secret

	if ops.IsControlPlane(etcdSecret) {
		controlPlaneSecret = etcdSecret
	} else {
		secrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
			WithLabels(plan.Label(capr.ControlPlaneRoleLabel, "true")).
			WithSorter(plan.DefaultSorter()).
			Collect()
		if plan.IsTransient(err) {
			return status, err
		} else if err != nil {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))

			return status, nil
		} else if len(secrets) == 0 {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))

			return status, nil
		}

		controlPlaneSecret = secrets[0]
	}

	kubectl, err := s.adapter.KubectlPath(etcdSecret)
	if err != nil {
		return status, err
	}
	kubeconfig := s.adapter.KubeconfigPath(etcdSecret)

	podSelectors := []string{
		"kube-system:k8s-app=kube-dns",
		"kube-system:k8s-app=kube-dns-autoscaler",
	}

	if s.adapter.RuntimeCommand() == "rke2" {
		podSelectors = append(podSelectors,
			"kube-system:app=rke2-metrics-server",
			"tigera-operator:k8s-app=tigera-operator",
			"calico-system:k8s-app=calico-node",
			"calico-system:k8s-app=calico-kube-controllers",
			"calico-system:k8s-app=calico-typha",
			"kube-system:k8s-app=canal",
			"kube-system:k8s-app=cilium",
			"kube-system:app=rke2-multus",
			"kube-system:app.kubernetes.io/name=rke2-ingress-nginx",
		)
	}

	provisioningDir := s.adapter.ProvisioningDataDirectory(etcdSecret)
	value := s.idempotencyValue()
	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)
	waitScriptPath := etcdRestoreScriptPath(s, etcdSecret, waitForPodListScriptName)

	instructions := []plan.OneTimeInstruction{
		ops.IdempotentInstruction(
			provisioningDir,
			idempotencyKey+"/post-restore-start-service",
			value,
			"systemctl",
			[]string{"start", s.adapter.ServerUnit()},
			nil),
		ops.IdempotentInstruction(
			provisioningDir,
			idempotencyKey+"/wait-for-api-server",
			value,
			"/bin/sh",
			[]string{
				"-x",
				waitScriptPath,
				kubectl,
				"--kubeconfig",
				kubeconfig,
				"get",
				"pods",
				"--all-namespaces",
			}, nil),
	}

	for i, podSelector := range podSelectors {
		if namespace, labelSelector, ok := strings.Cut(podSelector, ":"); ok {
			instructions = append(instructions, ops.IdempotentInstruction(provisioningDir, fmt.Sprintf("%s/cleanup-pods-%d", idempotencyKey, i), value, kubectl,
				[]string{
					"--kubeconfig",
					kubeconfig,
					"delete",
					"pods",
					"-n",
					namespace,
					"-l",
					labelSelector,
					"--wait=false",
				}, nil))
		}
	}

	nodePlan := &plan.Plan{
		Files: []plan.File{
			ops.IdempotentScriptFile(provisioningDir),
			{
				Content: base64.StdEncoding.EncodeToString([]byte(waitForPodListScript)),
				Path:    waitScriptPath,
				Dynamic: true,
			},
		},
		OneTimeInstructions: instructions,
	}

	if etcdSecret.Name != controlPlaneSecret.Name {
		etcdNodePlan := &plan.Plan{
			OneTimeInstructions: []plan.OneTimeInstruction{
				ops.IdempotentInstruction(
					provisioningDir,
					idempotencyKey+"/post-restore-start-service",
					value,
					"systemctl",
					[]string{"start", s.adapter.ServerUnit()},
					nil),
			},
		}

		planStatus, err := h.store.AssignPlan(etcdSecret, ops.WithOperationEnv(etcdNodePlan, opEnv), 1, 1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: pod cleanup failed for %s/%s",
				s.op.Namespace, s.op.Name, etcdSecret.Namespace, etcdSecret.Name)

			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("post-restore pod cleanup failed for %s/%s", etcdSecret.Namespace, etcdSecret.Name))

			return status, nil
		}

		if planStatus.Waiting() {
			logrus.Debugf("[etcdsnapshotrestore] %s/%s: waiting for pod cleanup for %s/%s", s.op.Namespace, s.op.Name, etcdSecret.Namespace, etcdSecret.Name)

			setWaitingForSinglePlan(&status, planStatus)

			return status, nil
		}

		nodePlan.Files = append(nodePlan.Files, plan.File{
			Content: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("server: \"https://%s:%s\"\n", s.adapter.GetServerURL(etcdSecret), s.adapter.GetSupervisorPort(etcdSecret)))),
			Path:    path.Join(s.adapter.ConfigDirectory(controlPlaneSecret), "zz_etcd-snapshot-restore.yaml"),
		})
	}

	planStatus, err := h.store.AssignPlan(controlPlaneSecret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: pod cleanup failed for %s/%s",
			s.op.Namespace, s.op.Name, etcdSecret.Namespace, etcdSecret.Name)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("post-restore pod cleanup failed for %s/%s", etcdSecret.Namespace, etcdSecret.Name))

		return status, nil
	}

	if planStatus.Waiting() {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for pod cleanup for %s/%s", s.op.Namespace, s.op.Name, controlPlaneSecret.Namespace, controlPlaneSecret.Name)

		setWaitingForSinglePlan(&status, planStatus)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to initial restart", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster)
	return status, nil
}

func (h *handler) reconcileRestartCluster(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus, nextStep opv1alpha1.ETCDSnapshotRestoreStep) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling cluster restart", s.op.Namespace, s.op.Name)

	// The same reconcile function backs both InitialRestartCluster (nextStep set) and the final
	// RestartCluster (nextStep == ""). Route the hook check to the matching prefix so a delegate
	// can subscribe to one restart pass without gating the other.
	delegated, err := h.handleHook(s, restartClusterHookPrefix(s.op.Status.Step))
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
		return status, nil
	}

	secrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithFilter(nonWindowsSecret).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	// The two restart phases must use distinct values; otherwise the second phase would skip the
	// restart as already-reconciled.
	value := s.idempotencyValue()
	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)
	if nextStep != "" {
		value = value + "/initial"
	} else {
		value = value + "/final"
	}

	if err = s.adapter.PauseCluster(false); err != nil {
		return status, err
	}

	initSecret, err := s.adapter.FindOrElectLeader(s.ownerKey, ops.IsEtcd)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	} else if initSecret == nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: no eligible etcd leader for restart", s.op.Namespace, s.op.Name)

		markFailed(&status, opv1alpha1.PlanFailedReason, "no eligible etcd leader for restart")

		return status, nil
	}

	serverURL := s.adapter.GetServerURL(initSecret)

	concurrency := 1
	results := make([]plan.PlanStatus, 0, concurrency)

	for _, secret := range secrets {
		nodePlan, err := buildRestartPlan(s, secret, initSecret, serverURL, value, nextStep != "")
		if err != nil {
			return status, err
		}

		planStatus, err := h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
		if err != nil {
			return status, err
		}

		results = append(results, *planStatus)

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: restart failed for %s/%s",
				s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("restart failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if planStatus.Waiting() {
			logrus.Debugf("[etcdsnapshotrestore] %s/%s: waiting for restart for %s/%s", s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			concurrency--
			if concurrency <= 0 {
				break
			}
		}
	}

	if concurrency < 1 {
		setWaitingForPlan(&status, results)

		return status, nil
	}

	if nextStep != "" {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to %s", s.op.Namespace, s.op.Name, nextStep)
		status.SetStep(nextStep)
		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	markSucceeded(&status)

	return status, nil
}

// buildPreflightPlan assembles the plan which hashes the server token the node currently has on
// disk, so the Preflight step can compare it against the hash stamped on the snapshot being restored.
//
// The instruction is deliberately not wrapped with ops.ConvertToIdempotentInstruction like the
// instructions of every other step: the idempotent script gates on the agent's attempt number and, on
// an attempt it considers already reconciled, prints a message instead of running the command. With
// SaveOutput that message would land in the applied output in place of the hash. A check must run on
// every attempt, so it relies on the operation environment for plan uniqueness instead.
func buildPreflightPlan(s *scope, secret *corev1.Secret) (*plan.Plan, error) {
	dataDir, err := s.adapter.DistroDataDirectory(secret)
	if err != nil {
		return nil, err
	}
	return &plan.Plan{
		OneTimeInstructions: []plan.OneTimeInstruction{
			{
				SaveOutput: true,
				Name:       preflightInstructionName,
				Command:    "/bin/sh",
				Args: []string{
					"-c",
					fmt.Sprintf(TokenHashCommandFormat, dataDir),
				},
			},
		},
	}, nil
}

// buildShutdownPlan assembles the plan which stops the distro on a node ahead of the restore: it
// clears any idempotency tracking left by a previous attempt, installs the distro version the
// cluster is now configured for, runs the distro's killall script, and on etcd and control-plane
// nodes lays down the etcd tombstone and removes the TLS directory.
//
// # Why the install lives here
//
// This is the one place in the operation that runs on every node while nothing is running, so it is
// the one place a version change can be applied uniformly. A kubernetesVersion or all restore has
// already rewritten the cluster's version by this point (the RestoreClusterConfig step did it, and
// waited for it to propagate), and that version is usually older than what the nodes have on disk.
// Two things depend on getting it onto disk before the cluster comes back:
//
//   - the Restore step's `--cluster-reset` runs on one node and must be executed by the snapshot's
//     own binary, because a newer server cannot reset onto etcd data written by an older one;
//   - every other node has to rejoin on that same version, or it would come up a minor ahead of the
//     control plane it is joining.
//
// The legacy planner does this in two separate places — an install in the restore plan for the init
// node (Planner.generateEtcdSnapshotRestorePlan) and another for every node when
// rkev1.ETCDSnapshotPhaseInitialRestartCluster runs a full reconcile, each desired plan carrying its
// own install. Doing it once here covers both, and no node installs twice.
//
// The installer is invoked with the distro's start suppressed (see installInstruction's
// INSTALL_<RUNTIME>_SKIP_START), so it lays down binaries and leaves the service alone: the
// system-agent installer image exits before its `systemctl restart` when that is set. It is also
// ordered ahead of the killall rather than after it, so that anything the installer did bring up is
// taken down again by the shutdown that follows.
func buildShutdownPlan(s *scope, secret *corev1.Secret) (*plan.Plan, error) {
	provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
	dataDir, err := s.adapter.DistroDataDirectory(secret)
	if err != nil {
		return nil, err
	}
	// Clear any prior idempotency tracking under the restore key before starting; subsequent
	// reconciles see the cleanup already applied and skip it.
	instructions := []plan.OneTimeInstruction{
		ops.GenerateIdempotencyCleanupInstruction(provisioningDir, idempotencyKey),
	}

	instructions = append(instructions,
		plan.OneTimeInstruction{
			Name:    "shutdown",
			Command: "/bin/sh",
			Env: []string{
				fmt.Sprintf("%s_DATA_DIR=%s", strings.ToUpper(s.adapter.RuntimeCommand()), dataDir),
			},
			Args: []string{
				"-c",
				fmt.Sprintf("if [ -z $(command -v %[1]s) ] && [ -z $(command -v %[2]s) ]; then echo %[1]s does not appear to be installed; exit 0; else %[2]s; fi",
					s.adapter.RuntimeCommand(),
					s.adapter.RuntimeCommand()+"-killall.sh"),
			},
		},
	)

	if secret.Labels[capr.EtcdRoleLabel] == "true" {
		instructions = append(instructions, plan.OneTimeInstruction{
			Name:    "create-etcd-tombstone",
			Command: "touch",
			Args:    []string{path.Join(dataDir, "server/db/etcd/tombstone")},
		})
	}

	if secret.Labels[capr.EtcdRoleLabel] == "true" || secret.Labels[capr.ControlPlaneRoleLabel] == "true" {
		instructions = append(instructions,
			plan.OneTimeInstruction{
				Name:    "remove-tls-directory",
				Command: "rm",
				Args:    []string{"-rf", path.Join(dataDir, "server/tls")},
			},
		)
	}

	return &plan.Plan{
		Files:               []plan.File{ops.IdempotentScriptFile(provisioningDir)},
		OneTimeInstructions: instructions,
	}, nil
}

// buildRestartPlan assembles the plan that brings a node back up after the restore: it restarts the
// service and manages the temporary `server:` drop-in that points the non-leader nodes at the node
// etcd was reset on.
//
// The node is already carrying the right distro version — the Shutdown step installed it on every
// node before anything was torn down (see buildShutdownPlan) — so this only has to start what is
// there. That is also why the restart is a plain `systemctl restart` rather than an install with a
// restart stamp, the way the legacy planner's full reconcile does it.
//
// initialPass distinguishes the two restart passes the operation makes — InitialRestartCluster, which
// stands the cluster back up around the restored etcd, and the final RestartCluster once the node
// cleanup has run. The initial pass writes the `server:` drop-in on every node other than the leader,
// so they rejoin through it rather than through whatever endpoint they were using; the final pass
// removes it again, on the leader too.
func buildRestartPlan(s *scope, secret, initSecret *corev1.Secret, serverURL, value string, initialPass bool) (*plan.Plan, error) {
	provisioningDir := s.adapter.ProvisioningDataDirectory(secret)

	probes, err := s.adapter.RenderProbes(secret, false)
	if err != nil {
		return nil, err
	}

	unit := s.adapter.ServerUnit()
	if !ops.IsEtcd(secret) && !ops.IsControlPlane(secret) {
		unit = s.adapter.RuntimeCommand() + "-agent"
	}

	instructions := []plan.OneTimeInstruction{
		ops.IdempotentInstruction(
			provisioningDir, idempotencyKey+"/restart", value, "systemctl", []string{"restart", unit}, nil),
	}

	nodePlan := &plan.Plan{
		Files:               []plan.File{ops.IdempotentScriptFile(provisioningDir)},
		OneTimeInstructions: instructions,
		Probes:              probes,
	}

	serverArgPath := path.Join(s.adapter.ConfigDirectory(secret), "zz_etcd-snapshot-restore.yaml")

	if initialPass && secret.UID != initSecret.UID {
		nodePlan.Files = append(nodePlan.Files, plan.File{
			Content: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("server: \"https://%s:%s\"\n", serverURL, s.adapter.GetSupervisorPort(secret)))),
			Path:    serverArgPath,
		})
	} else if !initialPass {
		nodePlan.OneTimeInstructions = append(nodePlan.OneTimeInstructions, plan.OneTimeInstruction{
			Name:    "remove-server-arg",
			Command: "rm",
			Args:    []string{"-rf", serverArgPath},
		})
	}

	return nodePlan, nil
}

// buildRestorePlan builds the plan that resets etcd from the snapshot on the elected leader:
// reinstall the configured distro version, wipe the etcd data directory, then `--cluster-reset`
// onto the snapshot. A nil snapshot means no ETCDSnapshot resource exists and snapshotName names a
// file already on disk.
func buildRestorePlan(s *scope, secret *corev1.Secret, snapshot *rkev1.ETCDSnapshot, snapshotName string) (*plan.Plan, error) {
	provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
	dataDir, err := s.adapter.DistroDataDirectory(secret)
	if err != nil {
		return nil, err
	}
	value := s.idempotencyValue()

	args := []string{
		"server",
		"--cluster-reset",
		fmt.Sprintf("--etcd-arg=advertise-client-urls=https://%s:2379", s.adapter.LoopbackAddress(secret)),
		"--etcd-disable-snapshots=false",
	}

	var env []string

	files := []plan.File{
		{
			Content: base64.StdEncoding.EncodeToString([]byte("server: \"\"\n")),
			Path:    path.Join(s.adapter.ConfigDirectory(secret), "zz_etcd-snapshot-restore.yaml"),
		},
		ops.IdempotentScriptFile(provisioningDir),
	}

	if snapshot == nil {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshotName), "--etcd-s3=false")
	} else if snapshot.SnapshotFile.S3 == nil {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshot.SnapshotFile.Name), "--etcd-s3=false")
	} else {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=%s", snapshot.SnapshotFile.Name))
		s3Args, s3Env, s3Files := s.adapter.ToS3ArgsEnvAndFiles(secret)
		args = append(args, s3Args...)
		env = append(env, s3Env...)
		files = append(files, s3Files...)
	}

	// Nothing is installed here: the Shutdown step put the version this snapshot needs on every node
	// while nothing was running (see buildShutdownPlan), so the reset below is already being executed
	// by the right binary.
	return &plan.Plan{
		Files: files,
		OneTimeInstructions: []plan.OneTimeInstruction{
			ops.ConvertToIdempotentInstruction(provisioningDir, idempotencyKey+"/clean-etcd-dir", value, plan.OneTimeInstruction{
				Name:    "remove-etcd-db-dir",
				Command: "rm",
				Args:    []string{"-rf", path.Join(dataDir, "server/db/etcd")},
			}),
			ops.IdempotentInstruction(provisioningDir, idempotencyKey+"/restore", value, s.adapter.RuntimeCommand(), args, env),
		},
	}, nil
}

// buildPostRestoreNodeCleanupPlan assembles the plan that runs the node-cleanup script on the init
// node. A non-empty skipReason signals that the caller should skip the cleanup phase entirely (the
// returned plan is nil in that case).
func buildPostRestoreNodeCleanupPlan(s *scope, initSecret *corev1.Secret, allSecrets []*corev1.Secret) (*plan.Plan, string, error) {
	kubectl, err := s.adapter.KubectlPath(initSecret)
	if err != nil {
		return nil, "", err
	}
	kubeconfig := s.adapter.KubeconfigPath(initSecret)
	if kubectl == "" || kubeconfig == "" {
		return nil, "adapter did not provide kubectl/kubeconfig paths", nil
	}

	var nodeNamesBuf []byte
	for _, secret := range allSecrets {
		if name := secret.Labels[capr.NodeNameLabel]; name != "" {
			nodeNamesBuf = fmt.Appendf(nodeNamesBuf, "%s\n", name)
		}
	}

	// With no node names to preserve, the cleanup script would delete every node — bail out instead
	// so we don't strand the cluster.
	if len(nodeNamesBuf) == 0 {
		return nil, "no node names available from machine-plan secrets", nil
	}

	provisioningDir := s.adapter.ProvisioningDataDirectory(initSecret)
	value := s.idempotencyValue()

	cleanupScriptPath := etcdRestoreScriptPath(s, initSecret, nodeCleanupScriptName)
	nodeNamesPath := etcdRestoreScriptPath(s, initSecret, fmt.Sprintf("node-names-%s", string(s.op.UID)))

	return &plan.Plan{
		Files: []plan.File{
			ops.IdempotentScriptFile(provisioningDir),
			{
				Content: base64.StdEncoding.EncodeToString([]byte(nodeCleanupScript)),
				Path:    cleanupScriptPath,
				Dynamic: true,
			},
			{
				Content: base64.StdEncoding.EncodeToString(nodeNamesBuf),
				Path:    nodeNamesPath,
				Dynamic: true,
			},
		},
		OneTimeInstructions: []plan.OneTimeInstruction{
			ops.IdempotentInstruction(provisioningDir, idempotencyKey+"/cleanup-nodes", value, "/bin/sh",
				[]string{cleanupScriptPath, nodeNamesPath},
				[]string{
					fmt.Sprintf("KUBECTL=%s", kubectl),
					fmt.Sprintf("KUBECONFIG=%s", kubeconfig),
				}),
		},
	}, "", nil
}

// reconcilePostRestoreNodeCleanup deletes Node objects from the restored cluster that no longer
// correspond to a machine still in the cluster. A snapshot taken before a node was removed will
// re-introduce the stale Node on restore; this step prunes those nodes so they don't block readiness.
//
// We assemble the keep-list (node names that should survive) from the currently-present machine-plan
// secrets, which carry the node name as a label. The cleanup script runs on the init node and deletes
// any Node not in the keep-list.
func (h *handler) reconcilePostRestoreNodeCleanup(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling post-restore node cleanup", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, PostRestoreNodeCleanupStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		setWaitingForDelegate(opv1alpha1.InProgressCondition, &status, s.beacon)
		return status, nil
	}

	initSecret, err := s.adapter.FindOrElectLeader(s.ownerKey, ops.IsEtcd)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	if initSecret == nil {
		logrus.Warnf("[etcdsnapshotrestore] %s/%s: no eligible etcd leader for node cleanup, skipping", s.op.Namespace, s.op.Name)
		status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestartCluster)
		return status, nil
	}

	allSecrets, err := plan.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if plan.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	nodePlan, skipReason, err := buildPostRestoreNodeCleanupPlan(s, initSecret, allSecrets)
	if err != nil {
		return status, err
	}
	if skipReason != "" {
		logrus.Warnf("[etcdsnapshotrestore] %s/%s: %s, skipping node cleanup", s.op.Namespace, s.op.Name, skipReason)
		status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestartCluster)
		return status, nil
	}

	opEnv := ops.OperationEnv(ControllerOwnerKey, s.op, status.Step)

	planStatus, err := h.store.AssignPlan(initSecret, ops.WithOperationEnv(nodePlan, opEnv), 1, 1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: node cleanup failed for %s/%s",
			s.op.Namespace, s.op.Name, initSecret.Namespace, initSecret.Name)

		markFailed(&status, opv1alpha1.PlanFailedReason, fmt.Sprintf("post-restore node cleanup failed for %s/%s", initSecret.Namespace, initSecret.Name))

		return status, nil
	}

	if planStatus.Waiting() {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for node cleanup for %s/%s", s.op.Namespace, s.op.Name, initSecret.Namespace, initSecret.Name)

		setWaitingForSinglePlan(&status, planStatus)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to final restart", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestartCluster)
	return status, nil
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
// An operation which no longer holds the beacon has none of that left to do: see
// beaconOptional. It terminates without the beacon being written to at all, which is what
// keeps an operation that lost its claim from reaching into whichever one holds it now.
func (h *handler) handleTerminal(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus, phase terminalPhase) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling operation %s", s.op.Namespace, s.op.Name, status.Phase)

	// A phase whose beacon claim is optional passes over its hook once that claim is gone: there is
	// no authority left to delegate, and pushing a delegate onto a beacon another controller now
	// holds would be reaching into its operation. Everything after the hook is either a no-op
	// without a claim (releaseBeacon) or owed regardless of one, so the operation still terminates
	// — which is what lets it be collected, or lets a deleted one retire its finalizer.
	honourHook := !phase.beaconOptional || holdsBeacon(s)

	if honourHook {
		delegated, err := h.handleHook(s, phase.hook)
		if err != nil {
			return status, err
		} else if delegated {
			// The delegate drives the beacon on this operation's behalf from here. Nothing is written
			// to the outcome condition: it already reports the outcome with the reason the phase handler
			// gave it, and that reason must survive the delegation. updateStatus reports the delegate on
			// Finalized for as long as the hook label is present, so the wait resolves on its own once
			// the delegate clears the label rather than being left behind on a condition.
			return status, nil
		}
	} else {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: %s with no claim on the beacon, leaving it untouched", s.op.Namespace, s.op.Name, status.Phase)
	}

	owning, err := h.releaseBeacon(s)
	if err != nil {
		return status, err
	}

	if phase.onRelease != nil {
		phase.onRelease(s, owning)
	}

	status.SetTerminated()

	return status, nil
}

// releaseBeacon hands the beacon back if this operation still holds it: ReleaseBeacon clears it
// outright (Active + Owner + Delegates) for the primary owner, or removes our slot from the
// delegate chain at any position otherwise. Reports whether we were the primary owner; the
// owner-vs-chain guard just avoids a no-op call.
func (h *handler) releaseBeacon(s *scope) (bool, error) {
	if !holdsBeacon(s) {
		return false, nil
	}

	owning := plan.IsOwningBeaconHolder(s.beacon, s.ownerKey)

	return owning, plan.ReleaseBeacon(s.beacon, h.beacons, s.ownerKey)
}

// holdsBeacon reports whether the operation still has a claim on the beacon, either as its primary
// owner or from anywhere in the delegate chain.
func holdsBeacon(s *scope) bool {
	return plan.IsOwningBeaconHolder(s.beacon, s.ownerKey) || plan.IsInDelegateChain(s.beacon, s.ownerKey)
}

// handleAborted handles the Aborted terminal phase, reached when the operation called its own work
// off rather than attempting it and losing — which is what separates it from Failed. Its phase hook
// runs first so a delegate can observe why the operation stopped.
func (h *handler) handleAborted(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           planv1alpha1.AbortedPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleCanceled handles the Canceled terminal phase, which is reached when an external controller
// cancels the operation or it is deleted before its terminal handling completed. The Canceled-phase
// hook runs first so delegates can react to the cancellation. Mirrors save's handleCanceled — what
// separates cancellation from the other outcomes is that it comes from outside the operation, where
// Failed means the work was attempted and lost and Aborted means the operation called it off.
func (h *handler) handleCanceled(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           planv1alpha1.CanceledPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleFailed handles the Failed terminal phase. Its hook gates beacon release on the failure
// path: a delegate that wants to inspect the failure state (op conditions, plan-secret statuses,
// leftover scripts on nodes) can hold the beacon before the next operation acquires it.
func (h *handler) handleFailed(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook:           planv1alpha1.FailedPhaseHookLabelPrefix,
		beaconOptional: true,
	})
}

// handleSucceeded handles the Succeeded terminal phase. Its hook gates the beacon release that
// signals "next operation may acquire", which delegates use to chain follow-up work (e.g.
// snapshotbackpopulate post-restore) before the cluster goes back to accepting new operations. On
// the owner path it then enqueues the cluster object so downstream reconciliation runs promptly;
// only the owner does so, since only the owner terminating implies downstream work.
func (h *handler) handleSucceeded(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	return h.handleTerminal(s, status, terminalPhase{
		hook: planv1alpha1.SucceededPhaseHookLabelPrefix,
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

// markSucceeded moves the operation into the Succeeded terminal phase, asserting the outcome: the
// work is over and the result will not change. Whether the controller is finished with the
// operation is reported separately, by the Finalized condition.
func markSucceeded(status *opv1alpha1.ETCDSnapshotRestoreStatus) {
	status.SetPhase(opv1alpha1.OperationPhaseSucceeded)

	opv1alpha1.SucceededCondition.True(status)
	opv1alpha1.SucceededCondition.Reason(status, opv1alpha1.FinishedReason)
	opv1alpha1.SucceededCondition.Message(status, "Operation completed successfully")
}

// markFailed moves the operation into the Failed terminal phase. Failed is what the operation
// reports when it gave up on its own work; the caller supplies the reason and message.
func markFailed(status *opv1alpha1.ETCDSnapshotRestoreStatus, reason, message string) {
	status.SetPhase(opv1alpha1.OperationPhaseFailed)

	opv1alpha1.FailedCondition.True(status)
	opv1alpha1.FailedCondition.Reason(status, reason)
	opv1alpha1.FailedCondition.Message(status, message)
}

// markAborted moves the operation into the Aborted terminal phase, for work the operation called off
// itself after finding a condition it cannot proceed past — a failed preflight check, say. The
// caller supplies the reason and message.
func markAborted(status *opv1alpha1.ETCDSnapshotRestoreStatus, reason, message string) {
	status.SetPhase(opv1alpha1.OperationPhaseAborted)

	opv1alpha1.AbortedCondition.True(status)
	opv1alpha1.AbortedCondition.Reason(status, reason)
	opv1alpha1.AbortedCondition.Message(status, message)
}

// markCanceled moves the operation into the Canceled terminal phase, for work that was called off
// from outside the operation — spec.Cancel being set, or a deletion that raced the operation.
func markCanceled(status *opv1alpha1.ETCDSnapshotRestoreStatus, reason, message string) {
	status.SetPhase(opv1alpha1.OperationPhaseCanceled)

	opv1alpha1.CanceledCondition.True(status)
	opv1alpha1.CanceledCondition.Reason(status, reason)
	opv1alpha1.CanceledCondition.Message(status, message)
}

// setWaitingForDelegate reports, through the condition belonging to the phase currently being
// handled, that the operation's beacon has been handed to a lifecycle-hook delegate and the
// controller is waiting for it to finish. The phase itself does not move: the operation is still
// where it was, it just isn't the one driving the beacon.
func setWaitingForDelegate(cond condition.Cond, status *opv1alpha1.ETCDSnapshotRestoreStatus, beacon *planv1alpha1.Beacon) {
	cond.True(status)
	cond.Reason(status, opv1alpha1.WaitingForDelegateReason)
	cond.Message(status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(beacon)))
}

// setWaitingForPlan reports that the current step's plans have been handed to the system-agents and
// the controller is now waiting on their feedback.
func setWaitingForPlan(status *opv1alpha1.ETCDSnapshotRestoreStatus, results []plan.PlanStatus) {
	opv1alpha1.InProgressCondition.True(status)
	opv1alpha1.InProgressCondition.Reason(status, opv1alpha1.WaitingForPlanAppliedReason)
	opv1alpha1.InProgressCondition.Message(status, fmt.Sprintf("Waiting in step %s: %s", status.Step, plan.Message(results)))
}

// setWaitingForSinglePlan is the one-node form of setWaitingForPlan, for the steps that dispatch to
// a single node at a time (restore, pod cleanup, node cleanup) and report that node's plan message
// on its own rather than a step-wide summary.
func setWaitingForSinglePlan(status *opv1alpha1.ETCDSnapshotRestoreStatus, planStatus *plan.PlanStatus) {
	opv1alpha1.InProgressCondition.True(status)
	opv1alpha1.InProgressCondition.Reason(status, opv1alpha1.WaitingForPlanAppliedReason)
	opv1alpha1.InProgressCondition.Message(status, plan.Message([]plan.PlanStatus{*planStatus}))
}

// updateStatus refreshes ObservedGeneration and every condition that is not the one the current
// phase handler owns.
//
// Division of labor for the outcome conditions (Succeeded / Failed / Canceled): a phase handler
// records *why* the operation ended, by setting the reason and message on the condition matching
// the phase it moves to — markSucceeded, markFailed and markCanceled do exactly that. This function
// asserts that outcome and denies the competing two, and owns the Finalized condition outright.
//
// The three states, in order:
//
//   - not terminal: the operation is still running. Progress conditions report where it is and
//     Finalized is False with NotFinalizedReason.
//   - terminal: the work is over and its outcome will not change, so the matching outcome condition
//     goes True (keeping the reason and message it was given at decision time) and the other two go
//     False. The progress conditions are cleared.
//   - terminal and terminated: the controller is done with the operation too — the terminal phase
//     hook was satisfied and the beacon released — so Finalized goes True. Until then it stays
//     False with FinalizingReason, which is the only difference between this state and the one
//     above.
func updateStatus(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) opv1alpha1.ETCDSnapshotRestoreStatus {
	logrus.Tracef("[etcdsnapshotrestore] %s/%s: updating conditions", op.Namespace, op.Name)

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

	if !ops.IsTerminal(status.Phase) {
		opv1alpha1.FinalizedCondition.False(&status)
		opv1alpha1.FinalizedCondition.Reason(&status, opv1alpha1.NotFinalizedReason)
		opv1alpha1.FinalizedCondition.Message(&status, "")

		if status.Phase == opv1alpha1.OperationPhasePending {
			opv1alpha1.PendingCondition.True(&status)
		} else if status.Phase == opv1alpha1.OperationPhaseInProgress {
			opv1alpha1.PendingCondition.False(&status)
			opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.InProgressReason)
			opv1alpha1.PendingCondition.Message(&status, "Operation now in progress")
		}

		return status
	}

	outcome, summary := opv1alpha1.OutcomeConditionFor(status.Phase)

	// The outcome is asserted as soon as the terminal phase is reached: the work is over and the
	// result will not change. The reason and message the phase handler recorded are left in place —
	// they are the record of why the operation ended.
	outcome.True(&status)

	for cond, reason := range map[condition.Cond]string{
		opv1alpha1.SucceededCondition: opv1alpha1.NotSuccessfulReason,
		opv1alpha1.FailedCondition:    opv1alpha1.NotFailedReason,
		opv1alpha1.AbortedCondition:   opv1alpha1.NotAbortedReason,
		opv1alpha1.CanceledCondition:  opv1alpha1.NotCanceledReason,
	} {
		if cond == outcome {
			continue
		}
		cond.False(&status)
		cond.Reason(&status, reason)
		cond.Message(&status, summary)
	}

	// Terminated is the separate question of whether the controller is done with the operation, so
	// it is the only thing the terminal marker gates. Note that an operation is still cancellable in
	// this window even though its outcome is already asserted — see cancelForDeletion.
	terminated := ops.IsTerminated(&status.OperationStatus)

	progressReason := opv1alpha1.FinalizingReason
	if terminated {
		progressReason = opv1alpha1.FinishedReason
	}

	opv1alpha1.PendingCondition.False(&status)
	opv1alpha1.PendingCondition.Reason(&status, progressReason)
	opv1alpha1.PendingCondition.Message(&status, summary)
	opv1alpha1.InProgressCondition.False(&status)
	opv1alpha1.InProgressCondition.Reason(&status, progressReason)
	opv1alpha1.InProgressCondition.Message(&status, summary)

	if !terminated {
		opv1alpha1.FinalizedCondition.False(&status)

		// Read the delegate back off the operation rather than remembering it on a condition: the
		// hook label is the source of truth, so when the delegate clears it this reverts by itself.
		if _, delegate := lifecycleHookDelegate(op, ops.TerminalPhaseHookPrefix(status.Phase)); delegate != "" {
			opv1alpha1.FinalizedCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
			opv1alpha1.FinalizedCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", delegate))
		} else {
			opv1alpha1.FinalizedCondition.Reason(&status, opv1alpha1.FinalizingReason)
			opv1alpha1.FinalizedCondition.Message(&status, "waiting for terminal handling to complete")
		}

		return status
	}

	opv1alpha1.FinalizedCondition.True(&status)
	opv1alpha1.FinalizedCondition.Reason(&status, opv1alpha1.FinishedReason)
	opv1alpha1.FinalizedCondition.Message(&status, summary)

	return status
}
