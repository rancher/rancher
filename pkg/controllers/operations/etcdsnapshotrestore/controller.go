package etcdsnapshotrestore

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const ControllerOwnerKey = "etcd-snapshot-restore"

// Step hook label prefixes for the etcdsnapshotrestore operation. Each prefix gates a single
// restore step and follows the shared label semantics documented on planv1alpha1's phase-hook
// label constants. The shutdown / restore / pod-cleanup / restart / node-cleanup / final-restart
// sequence is unique to restore — there is no analogue on save / encryption-key-rotation.
const (
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

	// RestartClusterStepHookLabelPrefix gates the final restart pass, after node cleanup. This
	// removes the temporary server-URL override and lets each node return to its normal
	// reconciliation.
	RestartClusterStepHookLabelPrefix = "restart-cluster.step.hook.operation.cattle.io/"
)

// idempotencyKey is the top-level key used to scope idempotency tracking for this controller.
// It is also used by the cleanup instruction issued during shutdown to clear prior tracking.
const idempotencyKey = "etcd-restore"

// etcdRestoreBinSubdir is the relative path (under the distro data directory) that holds the
// helper scripts the controller writes to nodes during the restore.
const etcdRestoreBinSubdir = "etcd-restore/bin"

const (
	waitForPodListScriptName = "wait_for_pod_list.sh"
	nodeCleanupScriptName    = "clean_up_nodes.sh"
)

const waitForPodListScript = `#!/bin/sh

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

const nodeCleanupScript = `#!/bin/sh

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

type handler struct {
	etcdsnapshotrestores operationcontrollers.ETCDSnapshotRestoreController

	etcdsnapshots rkecontrollers.ETCDSnapshotController

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
		etcdsnapshotrestores: clients.Operation.ETCDSnapshotRestore(),
		etcdsnapshots:        clients.RKE.ETCDSnapshot(),
		beacons:              clients.Plan.Beacon(),
		beaconCache:          clients.Plan.Beacon().Cache(),
		secrets:              clients.Core.Secret(),
		secretCache:          clients.Core.Secret().Cache(),
		dynamic:              clients.Dynamic,
		store:                planapi.NewStore(clients.Core.Secret()),
		clients:              clients,
	}

	operationcontrollers.RegisterETCDSnapshotRestoreStatusHandler(ctx, clients.Operation.ETCDSnapshotRestore(), "", "etcd-snapshot-restore-handler", h.OnChange)
}

func (h *handler) OnChange(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	status, err := h.onChange(op, status)
	if err != nil {
		return status, err
	}
	status = updateStatus(op, status)

	if reflect.DeepEqual(op.Status, status) {
		// handle after normal processing to allow for proper phase-related cleanup (freeing beacon)
		//
		// See the equivalent guard in etcdsnapshotsave's OnChange for the rationale: while any
		// lifecycle-hook label is still on the op, TTL garbage collection must be deferred so the
		// delegate has a chance to observe the terminal phase and pop itself from the beacon.
		if ops.IsTerminal(status.Phase) &&
			ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) &&
			!planv1alpha1.HasActiveLifecycleHook(op) {
			err = h.etcdsnapshotrestores.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
			if err != nil {
				return status, err
			}
			return status, generic.ErrSkip
		}

		h.etcdsnapshotrestores.EnqueueAfter(op.Namespace, op.Name, 5*time.Second)
	}
	return status, nil
}

func (h *handler) onChange(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	if op == nil {
		return status, nil
	}

	if op.DeletionTimestamp != nil {
		return status, nil
	}

	if ops.IsPaused(&op.Spec.OperationSpec) {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: skipping paused operation", op.Namespace, op.Name)
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
		logrus.Errorf("[etcdsnapshotrestore]: %s/%s failed to find cluster for %s", op.Namespace, op.Name, key)

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
			namespace = op.Namespace
		} else {
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
		logrus.Warnf("[etcdsnapshotrestore]: %s/%s failed to find beacon for %s", op.Namespace, op.Name, key)

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
		return h.handleCanceled(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseFailed {
		return h.handleFailed(s, status)
	}
	if status.Phase == opv1alpha1.OperationPhaseSucceeded {
		return h.handleSucceeded(s, status)
	}

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))

	return status, nil
}

type scope struct {
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

// delegate pushes delegate onto the beacon's delegate chain if it is not already there. It is a
// no-op when the delegate is already present, which keeps the call idempotent across the many
// reconciles that may occur while a hook is held.
func (h *handler) delegate(s *scope, name, delegate string) error {
	logrus.Tracef("[etcdsnapshotrestore] %s/%s: delegating ownership of beacon to %s on behalf of %s", s.op.Namespace, s.op.Name, delegate, name)

	if planapi.IsInDelegateChain(s.beacon, delegate) {
		return nil
	}

	beacon, err := planapi.PushDelegate(s.beacon, delegate, h.beacons)
	if err != nil {
		return err
	}
	s.beacon = beacon
	return nil
}

// handleHook is the per-handler entry point for the lifecycle-hook mechanism. It returns (true, nil)
// whenever a label with the given prefix exists on the operation, signalling the caller to short
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

	if name, delegate := h.lifecycleHookDelegate(s, prefix); delegate != "" {
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

	// The Pending-phase hook fires after the beacon has been acquired so external delegates can
	// inspect the cluster (machine-plan secrets, beacon ownership) before the controller starts
	// the actual restore workflow.
	delegated, err := h.handleHook(s, planv1alpha1.PendingPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.PendingCondition.True(&status)
		opv1alpha1.PendingCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.PendingCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
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
	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepShutdown)

	opv1alpha1.InProgressCondition.True(&status)
	opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.InProgressReason)
	return status, nil
}

func (h *handler) handleInProgress(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	if !planapi.IsOwningBeaconHolder(s.beacon, ControllerOwnerKey) {
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

	// InProgress-phase hook fires on every InProgress reconcile, ahead of step dispatch. This is
	// the broadest hook in the restore lifecycle — useful for delegates that need to gate ALL
	// step work uniformly without subscribing to each individual step prefix.
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

	opv1alpha1.FailedCondition.True(&status)
	opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownStepReason)
	opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("current step [\"%s\"] is unknown, expected one of: [\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]",
		status.Step,
		opv1alpha1.ETCDSnapshotRestoreStepShutdown,
		opv1alpha1.ETCDSnapshotRestoreStepRestore,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepRestartCluster))

	return status, nil
}

func (h *handler) reconcileShutdown(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling shutdown", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, ShutdownStepHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	secrets, err := planapi.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(planapi.DefaultSorter()).
		WithFilter(nonWindowsSecret).
		WithValidator(planapi.AtLeast(1, "")).
		Collect()
	if planapi.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	for _, secret := range secrets {
		provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
		// Clear any prior idempotency tracking under the restore key before starting; subsequent
		// reconciles see the cleanup already applied and skip it.
		instructions := []planapi.OneTimeInstruction{
			ops.GenerateIdempotencyCleanupInstruction(provisioningDir, idempotencyKey),
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "shutdown",
					Command: "/bin/sh",
					Env: []string{
						fmt.Sprintf("%s_DATA_DIR=%s", strings.ToUpper(s.adapter.RuntimeCommand()), s.adapter.DistroDataDirectory(secret)),
					},
					Args: []string{
						"-c",
						fmt.Sprintf("if [ -z $(command -v %[1]s) ] && [ -z $(command -v %[2]s) ]; then echo %[1]s does not appear to be installed; exit 0; else %[2]s; fi",
							s.adapter.RuntimeCommand(),
							s.adapter.RuntimeCommand()+"-killall.sh"),
					},
				},
			},
		}

		if secret.Labels[capr.EtcdRoleLabel] == "true" {
			instructions = append(instructions, planapi.OneTimeInstruction{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "create-etcd-tombstone",
					Command: "touch",
					Args:    []string{path.Join(s.adapter.DistroDataDirectory(secret), "server/db/etcd/tombstone")},
				},
			})
		}

		if secret.Labels[capr.EtcdRoleLabel] == "true" || secret.Labels[capr.ControlPlaneRoleLabel] == "true" {
			instructions = append(instructions,
				planapi.OneTimeInstruction{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "remove-tls-directory",
						Command: "rm",
						Args:    []string{"-rf", path.Join(s.adapter.DistroDataDirectory(secret), "server/tls")},
					},
				},
				planapi.OneTimeInstruction{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "remove-cred-directory",
						Command: "rm",
						Args:    []string{"-rf", path.Join(s.adapter.DistroDataDirectory(secret), "server/cred")},
					},
				},
			)
		}

		nodePlan := &planapi.Plan{
			Files:               []planapi.File{ops.IdempotentScriptFile(provisioningDir)},
			OneTimeInstructions: instructions,
		}

		planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: shutdown failed for %s/%s",
				s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("shutdown failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for shutdown: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
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
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	snapshotName := s.op.Spec.Args.Name
	if snapshotName == "" {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: snapshot name is required for etcd restore", s.op.Namespace, s.op.Name)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "snapshot name is required for etcd restore")

		return status, nil
	}

	filter := ops.IsEtcd
	snapshot, err := h.etcdsnapshots.Get(s.namespace, snapshotName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[etcdsnapshotrestore] %s/%s: could not find associated etcdsnapshot.rke.cattle.io %s, assuming snapshot file", s.op.Namespace, s.op.Name, snapshotName)
		snapshot = nil
	} else if err != nil {
		return status, err
	} else if snapshot != nil && snapshot.SnapshotFile.S3 == nil {
		if len(snapshot.OwnerReferences) == 0 {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: cannot find machine by owner reference for snapshot %s/%s", s.op.Namespace, s.op.Name, snapshot.Namespace, snapshot.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, "owner reference is required for s3 etcd restore")

			return status, nil
		}

		ref := snapshot.OwnerReferences[0]

		filter = func(secret *corev1.Secret) bool {
			if secret == nil || secret.Labels == nil {
				return false
			}

			if secret.Labels[planv1alpha1.MachineLifecycleName] == ref.Name {
				return true
			}

			return false
		}
	}

	// todo: elect leader based on snapshot
	secret, err := s.adapter.FindOrElectLeader(ControllerOwnerKey, filter)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	} else if secret == nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: no eligible etcd leader for restore", s.op.Namespace, s.op.Name)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "no eligible etcd leader for restore")

		return status, nil
	}

	provisioningDir := s.adapter.ProvisioningDataDirectory(secret)
	value := s.idempotencyValue()

	args := []string{
		"server",
		"--cluster-reset",
		fmt.Sprintf("--etcd-arg=advertise-client-urls=https://%s:2379", s.adapter.LoopbackAddress(secret)),
		"--etcd-disable-snapshots=false",
	}

	var env []string

	files := []planapi.File{
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

	nodePlan := &planapi.Plan{
		Files: files,
		OneTimeInstructions: []planapi.OneTimeInstruction{
			ops.ConvertToIdempotentInstruction(provisioningDir, idempotencyKey+"/clean-etcd-dir", value, planapi.OneTimeInstruction{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "remove-etcd-db-dir",
					Command: "rm",
					Args:    []string{"-rf", path.Join(s.adapter.DistroDataDirectory(secret), "server/db/etcd")},
				},
			}),
			ops.IdempotentInstruction(provisioningDir, idempotencyKey+"/restore", value, s.adapter.RuntimeCommand(), args, env),
		},
	}

	planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: etcd restore failed for %s/%s",
			s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("etcd restore failed for %s/%s", secret.Namespace, secret.Name))

		return status, nil
	}

	if wait, msg := planStatus.Wait(); wait {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for etcd restore: %s", s.op.Namespace, s.op.Name, msg)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, msg)

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
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	etcdSecret, err := s.adapter.FindOrElectLeader(ControllerOwnerKey, nil)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	} else if etcdSecret == nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: no eligible etcd leader for restore", s.op.Namespace, s.op.Name)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, "no eligible etcd leader for restore")

		return status, nil
	}

	var controlPlaneSecret *corev1.Secret

	if ops.IsControlPlane(etcdSecret) {
		controlPlaneSecret = etcdSecret
	} else {
		secrets, err := planapi.NewCollector(h.secrets, s.clusterObj, s.namespace).
			WithLabels(planapi.Label(capr.ControlPlaneRoleLabel, "true")).
			WithSorter(planapi.DefaultSorter()).
			Collect()
		if planapi.IsTransient(err) {
			return status, err
		} else if err != nil {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
			return status, nil
		} else if len(secrets) == 0 {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		}

		controlPlaneSecret = secrets[0]
	}

	kubectl := s.adapter.KubectlPath(etcdSecret)
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
	waitScriptPath := etcdRestoreScriptPath(s, etcdSecret, waitForPodListScriptName)

	instructions := []planapi.OneTimeInstruction{
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

	nodePlan := &planapi.Plan{
		Files: []planapi.File{
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
		etcdNodePlan := &planapi.Plan{
			OneTimeInstructions: []planapi.OneTimeInstruction{
				ops.IdempotentInstruction(
					provisioningDir,
					idempotencyKey+"/post-restore-start-service",
					value,
					"systemctl",
					[]string{"start", s.adapter.ServerUnit()},
					nil),
			},
		}

		planStatus, err := h.store.AssignPlan(etcdSecret, etcdNodePlan, 1, -1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: pod cleanup failed for %s/%s",
				s.op.Namespace, s.op.Name, etcdSecret.Namespace, etcdSecret.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("post-restore pod cleanup failed for %s/%s", etcdSecret.Namespace, etcdSecret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for pod cleanup: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}

		nodePlan.Files = append(nodePlan.Files, planapi.File{
			Content: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("server: \"https://%s:%s\"\n", s.adapter.GetServerURL(etcdSecret), s.adapter.GetSupervisorPort(etcdSecret)))),
			Path:    path.Join(s.adapter.ConfigDirectory(controlPlaneSecret), "zz_etcd-snapshot-restore.yaml"),
		})
	}

	planStatus, err := h.store.AssignPlan(controlPlaneSecret, nodePlan, 1, -1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: pod cleanup failed for %s/%s",
			s.op.Namespace, s.op.Name, etcdSecret.Namespace, etcdSecret.Name)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("post-restore pod cleanup failed for %s/%s", etcdSecret.Namespace, etcdSecret.Name))

		return status, nil
	}

	if wait, msg := planStatus.Wait(); wait {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for pod cleanup: %s", s.op.Namespace, s.op.Name, msg)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, msg)

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
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	secrets, err := planapi.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithFilter(nonWindowsSecret).
		WithSorter(planapi.DefaultSorter()).
		Collect()
	if planapi.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	// The two restart phases must use distinct values; otherwise the second phase would skip the
	// restart as already-reconciled.
	value := s.idempotencyValue()
	if nextStep != "" {
		value = value + "/initial"
	} else {
		value = value + "/final"
	}

	initSecret, err := s.adapter.FindOrElectLeader(ControllerOwnerKey, ops.IsEtcd)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	serverURL := s.adapter.GetServerURL(initSecret)

	for _, secret := range secrets {
		provisioningDir := s.adapter.ProvisioningDataDirectory(secret)

		probes, err := s.adapter.RenderProbes(secret, false)
		if err != nil {
			return status, err
		}

		unit := s.adapter.ServerUnit()
		if secret.Labels[capr.EtcdRoleLabel] != "true" && secret.Labels[capr.ControlPlaneRoleLabel] != "true" {
			unit = s.adapter.RuntimeCommand() + "-agent"
		}

		nodePlan := &planapi.Plan{
			Files: []planapi.File{ops.IdempotentScriptFile(provisioningDir)},
			OneTimeInstructions: []planapi.OneTimeInstruction{
				ops.IdempotentInstruction(provisioningDir, idempotencyKey+"/restart", value, "systemctl",
					[]string{"restart", unit}, nil),
			},
			Probes: probes,
		}

		if secret.UID != initSecret.UID {
			if nextStep != "" {
				nodePlan.Files = append(nodePlan.Files, planapi.File{
					Content: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("server: \"https://%s:%s\"\n", serverURL, s.adapter.GetSupervisorPort(secret)))),
					Path:    path.Join(s.adapter.ConfigDirectory(secret), "zz_etcd-snapshot-restore.yaml"),
				})
			} else {
				nodePlan.OneTimeInstructions = append(nodePlan.OneTimeInstructions, planapi.OneTimeInstruction{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "remove-server-arg",
						Command: "rm",
						Args: []string{
							"-rf", path.Join(s.adapter.ConfigDirectory(secret), "zz_etcd-snapshot-restore.yaml"),
						},
					},
				})
			}
		} else {
			if nextStep == "" {
				nodePlan.OneTimeInstructions = append(nodePlan.OneTimeInstructions, planapi.OneTimeInstruction{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "remove-server-arg",
						Command: "rm",
						Args: []string{
							"-rf", path.Join(s.adapter.ConfigDirectory(secret), "zz_etcd-snapshot-restore.yaml"),
						},
					},
				})
			}
		}

		planStatus, err := h.store.AssignPlan(secret, nodePlan, 1, -1)
		if err != nil {
			return status, err
		}

		if planStatus.Failure() {
			logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: restart failed for %s/%s",
				s.op.Namespace, s.op.Name, secret.Namespace, secret.Name)

			status.SetPhase(opv1alpha1.OperationPhaseFailed)

			opv1alpha1.FailedCondition.True(&status)
			opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
			opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("restart failed for %s/%s", secret.Namespace, secret.Name))

			return status, nil
		}

		if wait, msg := planStatus.Wait(); wait {
			logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for restart: %s", s.op.Namespace, s.op.Name, msg)

			opv1alpha1.InProgressCondition.True(&status)
			opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
			opv1alpha1.InProgressCondition.Message(&status, msg)

			return status, nil
		}
	}

	if nextStep != "" {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to %s", s.op.Namespace, s.op.Name, nextStep)
		status.SetStep(nextStep)
		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: marking as success", s.op.Namespace, s.op.Name)

	status.SetPhase(opv1alpha1.OperationPhaseSucceeded)

	opv1alpha1.SucceededCondition.True(&status)
	opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.FinishedReason)
	opv1alpha1.SucceededCondition.Message(&status, "Operation completed successfully")

	return status, nil
}

// buildPostRestoreNodeCleanupPlan assembles the plan that runs the node-cleanup script on the init
// node. A non-empty skipReason signals that the caller should skip the cleanup phase entirely (the
// returned plan is nil in that case).
func buildPostRestoreNodeCleanupPlan(s *scope, initSecret *corev1.Secret, allSecrets []*corev1.Secret) (plan *planapi.Plan, skipReason string) {
	kubectl := s.adapter.KubectlPath(initSecret)
	kubeconfig := s.adapter.KubeconfigPath(initSecret)
	if kubectl == "" || kubeconfig == "" {
		return nil, "adapter did not provide kubectl/kubeconfig paths"
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
		return nil, "no node names available from machine-plan secrets"
	}

	provisioningDir := s.adapter.ProvisioningDataDirectory(initSecret)
	value := s.idempotencyValue()

	cleanupScriptPath := etcdRestoreScriptPath(s, initSecret, nodeCleanupScriptName)
	nodeNamesPath := etcdRestoreScriptPath(s, initSecret, fmt.Sprintf("node-names-%s", string(s.op.UID)))

	return &planapi.Plan{
		Files: []planapi.File{
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
		OneTimeInstructions: []planapi.OneTimeInstruction{
			ops.IdempotentInstruction(provisioningDir, idempotencyKey+"/cleanup-nodes", value, "/bin/sh",
				[]string{cleanupScriptPath, nodeNamesPath},
				[]string{
					fmt.Sprintf("KUBECTL=%s", kubectl),
					fmt.Sprintf("KUBECONFIG=%s", kubeconfig),
				}),
		},
	}, ""
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
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	initSecret, err := s.adapter.FindOrElectLeader(ControllerOwnerKey, ops.IsEtcd)
	if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	if initSecret == nil {
		logrus.Warnf("[etcdsnapshotrestore] %s/%s: no eligible etcd leader for node cleanup, skipping", s.op.Namespace, s.op.Name)
		status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestartCluster)
		return status, nil
	}

	allSecrets, err := planapi.NewCollector(h.secrets, s.clusterObj, s.namespace).
		WithSorter(planapi.DefaultSorter()).
		Collect()
	if planapi.IsTransient(err) {
		return status, err
	} else if err != nil {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: encountered terminal error collecting machine-plan secrets: %v", s.op.Namespace, s.op.Name, err)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err))
		return status, nil
	}

	nodePlan, skipReason := buildPostRestoreNodeCleanupPlan(s, initSecret, allSecrets)
	if skipReason != "" {
		logrus.Warnf("[etcdsnapshotrestore] %s/%s: %s, skipping node cleanup", s.op.Namespace, s.op.Name, skipReason)
		status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestartCluster)
		return status, nil
	}

	planStatus, err := h.store.AssignPlan(initSecret, nodePlan, 1, -1)
	if err != nil {
		return status, err
	}

	if planStatus.Failure() {
		logrus.Errorf("[etcdsnapshotrestore] %s/%s: marking operation as failed: node cleanup failed for %s/%s",
			s.op.Namespace, s.op.Name, initSecret.Namespace, initSecret.Name)

		status.SetPhase(opv1alpha1.OperationPhaseFailed)

		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.PlanFailedReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("post-restore node cleanup failed for %s/%s", initSecret.Namespace, initSecret.Name))

		return status, nil
	}

	if wait, msg := planStatus.Wait(); wait {
		logrus.Infof("[etcdsnapshotrestore] %s/%s: waiting for node cleanup: %s", s.op.Namespace, s.op.Name, msg)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, msg)

		return status, nil
	}

	logrus.Infof("[etcdsnapshotrestore] %s/%s: transitioning to final restart", s.op.Namespace, s.op.Name)

	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepRestartCluster)
	return status, nil
}

// handleCanceled is called when an external controller cancels the operation. It runs the
// Canceled-phase hook first so delegates can react to the cancellation, then releases the beacon
// if this controller still owns it. Mirrors save's handleCanceled — the cancel-vs-fail
// distinction is that an external party cancels whereas the operation fails itself.
func (h *handler) handleCanceled(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling operation canceled", s.op.Namespace, s.op.Name)

	delegated, err := h.handleHook(s, planv1alpha1.CanceledPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.CanceledCondition.True(&status)
		opv1alpha1.CanceledCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.CanceledCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	if planapi.IsOwningBeaconHolder(s.beacon, ControllerOwnerKey) {
		s.beacon, err = planapi.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}
		if err := planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey); err != nil {
			return status, err
		}
	}
	return status, nil
}

func (h *handler) handleFailed(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	// Failed-phase hook gates beacon release on the failure path. A delegate that wants to inspect
	// the failure state (op conditions, plan-secret statuses, leftover scripts on nodes) can hold
	// the beacon here before the next operation acquires it.
	delegated, err := h.handleHook(s, planv1alpha1.FailedPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	if err := planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey); err != nil {
		return status, err
	}
	return status, nil
}

func (h *handler) handleSucceeded(s *scope, status opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error) {
	logrus.Debugf("[etcdsnapshotrestore] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

	// Succeeded-phase hook gates the beacon release that signals "next operation may acquire".
	// Delegates use this to chain follow-up work (e.g. snapshotbackpopulate post-restore) before
	// the cluster goes back to accepting new operations.
	delegated, err := h.handleHook(s, planv1alpha1.SucceededPhaseHookLabelPrefix)
	if err != nil {
		return status, err
	} else if delegated {
		opv1alpha1.SucceededCondition.True(&status)
		opv1alpha1.SucceededCondition.Reason(&status, opv1alpha1.WaitingForDelegateReason)
		opv1alpha1.SucceededCondition.Message(&status, fmt.Sprintf("Waiting for delegates to finish: %v", opv1alpha1.WaitingForDelegateMessage(s.beacon)))
		return status, nil
	}

	if planapi.AuthorizedForBeacon(s.beacon, ControllerOwnerKey) {
		s.beacon, err = planapi.ToggleBeacon(s.beacon, false, h.beacons)
		if err != nil {
			return status, err
		}

		if err := planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey); err != nil {
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
func updateStatus(op *opv1alpha1.ETCDSnapshotRestore, status opv1alpha1.ETCDSnapshotRestoreStatus) opv1alpha1.ETCDSnapshotRestoreStatus {
	logrus.Tracef("[etcdsnapshotrestore] %s/%s: updating conditions", op.Namespace, op.Name)

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
