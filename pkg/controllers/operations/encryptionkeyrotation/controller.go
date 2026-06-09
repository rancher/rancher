package encryptionkeyrotation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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
		secretCache:            clients.Core.Secret().Cache(),
		dynamic:                clients.Dynamic,
		store:                  planapi.NewStore(clients.Core.Secret()),
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

	ekrAdapter, err := ops.NewAdapter(h.clients, &ustr)
	if err != nil {
		return status, err
	}

	s := &scope{
		op:         op,
		beacon:     beacon,
		namespace:  namespace,
		clusterObj: &ustr,
		adapter:    ekrAdapter,
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

	// Failed and Succeeded run their cleanup and then fall through to the shared TTL deletion block.
	if status.Phase == opv1alpha1.OperationPhaseFailed {
		status, err = h.handleFailed(s, status)
	} else if status.Phase == opv1alpha1.OperationPhaseSucceeded {
		status, err = h.handleSucceeded(s, status)
	} else {
		opv1alpha1.FailedCondition.True(&status)
		opv1alpha1.FailedCondition.Reason(&status, opv1alpha1.UnknownPhaseReason)
		opv1alpha1.FailedCondition.Message(&status, fmt.Sprintf("unknown phase [%s]", op.Status.Phase))
		return status, nil
	}

	if err != nil {
		return status, err
	}

	// Delete expired terminal CRs after cleanup has run.
	if ops.IsTerminal(status.Phase) && ops.IsExpired(&op.Spec.OperationSpec, &status.OperationStatus) {
		err = h.encryptionkeyrotations.Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
		if err != nil {
			return status, err
		}
		return status, generic.ErrSkip
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

	// FindOrElectLeader persists the elected node via annotation so the same
	// node is reused across Rancher restarts and cache churn.
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

	logrus.Debugf("[encryptionkeyrotation] %s/%s: using leader %s for rotate-keys", s.op.Namespace, s.op.Name, leader.Name)
	return h.assignRotateKeysPlan(s, status, leader)
}

// assignRotateKeysPlan assigns the rotate-keys plan to the leader and waits for runtime
// completion. The plan uses a shell wrapper so the controller classifies timeout vs failure
// itself from saved output, advancing to Restart requires reencrypt_finished in periodic status.
func (h *handler) assignRotateKeysPlan(s *scope, status opv1alpha1.EncryptionKeyRotationStatus, leader *corev1.Secret) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	// Pause the CAPI cluster while rotate-keys is active so unrelated activity
	// does not race with the encryption-key rotation plan.
	if err := s.adapter.PauseClusterActivity(true); err != nil {
		return status, err
	}

	probes, err := s.adapter.RenderProbes(leader, true)
	if err != nil {
		return status, err
	}

	uid := encryptionKeyRotationOperationUIDEnv(s.op)
	step := encryptionKeyRotationStepEnv(status.Step)
	runtime := s.adapter.RuntimeCommand()

	nodePlan := &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{
			// 1. Run rotate-keys via wrapper that always exits 0; captures real exit code in output.
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    ekrRotateKeysInstructionName,
					Command: "/bin/sh",
					Args:    []string{"-c", rotateKeysScript(runtime)},
					Env:     []string{uid, step},
				},
				SaveOutput: true,
			},
			// 2. Poll until secrets-encrypt status responds; gates planStatus.Applied until
			// the encryption server is reachable after key reload.
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "wait-for-secrets-encrypt-status",
					Command: "/bin/sh",
					Args:    []string{"-c", waitForStatusScript(runtime)},
					Env:     []string{uid, step},
				},
			},
			// 3. One-time status snapshot captured when the plan is applied; provides an
			// observability anchor and confirms the endpoint is stable.
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    ekrStatusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     []string{uid, step},
				},
				SaveOutput: true,
			},
		},
		PeriodicInstructions: []planapi.PeriodicInstruction{
			// Runs every 5s independently; used for stage/hash convergence checking.
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    ekrStatusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     []string{uid, step},
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

	if wait, msg := planStatus.Wait(); wait {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for rotate-keys plan on leader %s: %s", s.op.Namespace, s.op.Name, leader.Name, msg)

		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, msg)

		return status, nil
	}

	// Plan applied and probes passed. Read the one-time output to check the rotate-keys exit code.
	appliedOutput, err := planapi.ReadAppliedOutput(leader)
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
	waitMsg, err := ekrConvergenceWaitMessage(leader, false)
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

func (h *handler) reconcileRestart(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling service restart", s.op.Namespace, s.op.Name)

	pool, err := planapi.NewLabeler().
		And(
			planapi.Label(capr.ClusterNameLabel, s.clusterObj.GetName()),
			planapi.Or(
				planapi.Label(capr.EtcdRoleLabel, "true"),
				planapi.Label(capr.ControlPlaneRoleLabel, "true"),
			)).
		WithSorter(planapi.DefaultSorter()).
		Collect(h.secretCache, s.namespace)
	if err != nil {
		return status, err
	}

	if len(pool) == 0 || !ops.IsControlPlane(pool[len(pool)-1]) {
		logrus.Errorf("[encryptionkeyrotation] %s/%s: no control-plane nodes found at restart step", s.op.Namespace, s.op.Name)
		markFailed(&status, opv1alpha1.UnknownStepReason, "no control-plane nodes found; cannot verify post-restart encryption status")
		return status, nil
	}

	uid := encryptionKeyRotationOperationUIDEnv(s.op)
	serverUnit := s.adapter.ServerUnit()
	runtime := s.adapter.RuntimeCommand()

	for i, secret := range pool {
		if !ops.IsControlPlane(secret) {
			status, done, err := h.reconcileEtcdOnlyRestartNode(s, status, secret, uid, serverUnit)
			if err != nil {
				return status, err
			}
			if !done {
				return status, nil
			}
			continue
		}

		isLastCP := i == len(pool)-1
		status, done, err := h.reconcileCPRestartNode(s, status, secret, uid, serverUnit, runtime, isLastCP)
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

// reconcileEtcdOnlyRestartNode assigns the restart plan for an etcd-only node.
// Returns done=true when the node is finished; done=false when reconciliation should stop and retry.
func (h *handler) reconcileEtcdOnlyRestartNode(s *scope, status opv1alpha1.EncryptionKeyRotationStatus, secret *corev1.Secret, uid, serverUnit string) (opv1alpha1.EncryptionKeyRotationStatus, bool, error) {
	probes, err := s.adapter.RenderProbes(secret, true)
	if err != nil {
		return status, false, err
	}

	step := encryptionKeyRotationStepEnv(status.Step)
	nodePlan := &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "restart",
					Command: "systemctl",
					Args:    []string{"restart", serverUnit},
					Env:     []string{uid, step},
				},
			},
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "wait-for-systemctl-status",
					Command: "/bin/sh",
					Args:    []string{"-c", waitForSystemctlStatusScript(serverUnit)},
					Env:     []string{uid, step},
				},
			},
		},
		Probes: probes,
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

	if wait, msg := planStatus.Wait(); wait {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for restart on %s: %s", s.op.Namespace, s.op.Name, secret.Name, msg)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("waiting for restart on %s: %s", secret.Name, msg))
		return status, false, nil
	}

	return status, true, nil
}

// reconcileCPRestartNode assigns the restart plan for a control-plane node, including post-restart
// secrets-encrypt convergence. requireHashMatch should be true for the last CP node.
// Returns done=true when the node is finished; done=false when reconciliation should stop and retry.
func (h *handler) reconcileCPRestartNode(s *scope, status opv1alpha1.EncryptionKeyRotationStatus, secret *corev1.Secret, uid, serverUnit, runtime string, requireHashMatch bool) (opv1alpha1.EncryptionKeyRotationStatus, bool, error) {
	probes, err := s.adapter.RenderProbes(secret, true)
	if err != nil {
		return status, false, err
	}

	step := encryptionKeyRotationStepEnv(status.Step)
	nodePlan := &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "restart",
					Command: "systemctl",
					Args:    []string{"restart", serverUnit},
					Env:     []string{uid, step},
				},
			},
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "wait-for-systemctl-status",
					Command: "/bin/sh",
					Args:    []string{"-c", waitForSystemctlStatusScript(serverUnit)},
					Env:     []string{uid, step},
				},
			},
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    "wait-for-secrets-encrypt-status",
					Command: "/bin/sh",
					Args:    []string{"-c", waitForStatusScript(runtime)},
					Env:     []string{uid, step},
				},
			},
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    ekrStatusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     []string{uid, step},
				},
				SaveOutput: true,
			},
		},
		PeriodicInstructions: []planapi.PeriodicInstruction{
			{
				CommonInstruction: planapi.CommonInstruction{
					Name:    ekrStatusPeriodicName,
					Command: runtime,
					Args:    []string{"secrets-encrypt", "status"},
					Env:     []string{uid, step},
				},
				PeriodSeconds: 5,
			},
		},
		Probes: probes,
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

	if wait, msg := planStatus.Wait(); wait {
		logrus.Debugf("[encryptionkeyrotation] %s/%s: waiting for restart on %s: %s", s.op.Namespace, s.op.Name, secret.Name, msg)
		opv1alpha1.InProgressCondition.True(&status)
		opv1alpha1.InProgressCondition.Reason(&status, opv1alpha1.WaitingForPlanAppliedReason)
		opv1alpha1.InProgressCondition.Message(&status, fmt.Sprintf("waiting for restart on %s: %s", secret.Name, msg))
		return status, false, nil
	}

	waitMsg, err := ekrConvergenceWaitMessage(secret, requireHashMatch)
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
	return status, true, nil
}

func (h *handler) handleFailed(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation failed", s.op.Namespace, s.op.Name)

	if err := s.adapter.PauseClusterActivity(false); err != nil {
		return status, err
	}

	err := planapi.ReleaseBeacon(s.beacon, h.beacons, ControllerOwnerKey)
	if err != nil {
		return status, err
	}
	return status, nil
}

func (h *handler) handleSucceeded(s *scope, status opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error) {
	logrus.Debugf("[encryptionkeyrotation] %s/%s: handling operation succeeded", s.op.Namespace, s.op.Name)

	if err := s.adapter.PauseClusterActivity(false); err != nil {
		return status, err
	}

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

// encryptionKeyRotationOperationUIDEnv returns an env var that embeds the operation UID into machine plan
// instructions. This ensures that two distinct EncryptionKeyRotation CRs targeting the same cluster produce
// different serialized plan bytes, so system-agent reruns the operation rather than treating it as already applied.
func encryptionKeyRotationOperationUIDEnv(op *opv1alpha1.EncryptionKeyRotation) string {
	return fmt.Sprintf("ENCRYPTION_KEY_ROTATION_OPERATION_UID=%s", op.UID)
}

// encryptionKeyRotationStepEnv returns an env var that embeds the current operation step into
// plan instructions. This ensures that rotate-phase and restart-phase plans have different
// serialized bytes, so system-agent reruns them and clears stale applied output rather than
// treating the new plan as already applied.
func encryptionKeyRotationStepEnv(step opv1alpha1.EncryptionKeyRotationStep) string {
	return fmt.Sprintf("ENCRYPTION_KEY_ROTATION_STEP=%s", step)
}

// errRotateKeysOutputNotYet is returned by readRotateKeysResult when the rotate-keys output
// or exit-code line is not yet written to the plan secret. Callers should wait and retry.
// Corrupt/unparse-able exit codes return a different error so callers can fail the operation.
var errRotateKeysOutputNotYet = errors.New("rotate-keys output not yet available")

// errEKRStatusTimeout is returned by ekrStatusFromOutput when a transient CLI timeout is
// detected in the secrets-encrypt status output. Callers should wait for the next periodic
// run rather than failing the operation.
var errEKRStatusTimeout = errors.New("secrets-encrypt status timed out")

const (
	ekrRotateKeysInstructionName = "rotate-keys"
	ekrStatusPeriodicName        = "secrets-encrypt-status"
	ekrStageReencryptFinished    = "reencrypt_finished"
	ekrHashesMatch               = "All hashes match"
	ekrExitCodePrefix            = "rancher-rotate-keys-exit-code="

	// ekrRotateKeysTimeoutMessage and ekrRotateKeysTimeoutEndpoint are combined with
	// ekrTimeoutMarkers to identify CLI timeouts from the rotate-keys wrapper output.
	ekrRotateKeysTimeoutMessage  = "see server log for details"
	ekrRotateKeysTimeoutEndpoint = "/encrypt/config"

	// ekrStatusTimeoutEndpoint identifies a transient timeout from secrets-encrypt status.
	ekrStatusTimeoutEndpoint = "/encrypt/status"
)

// ekrTimeoutMarkers are the known CLI timeout signatures from secrets-encrypt calls.
var ekrTimeoutMarkers = []string{
	"Client.Timeout exceeded while awaiting headers",
	"timeout awaiting response headers",
	"context deadline exceeded",
}

type ekrCommandResult struct {
	output   string
	exitCode int
}

type ekrRuntimeStatus struct {
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
		fmt.Sprintf(`printf '%s%%s\n' "$exitCode"`, ekrExitCodePrefix),
		"exit 0",
	}, "\n")
}

// readRotateKeysResult extracts the embedded exit code from the applied one-time instruction
// output. Returns errRotateKeysOutputNotYet when the key or exit-code line is not yet written
// (callers should wait). Returns a non-sentinel error when the exit code is present but
// corrupt (callers should fail the operation).
func readRotateKeysResult(appliedOutput map[string][]byte) (ekrCommandResult, error) {
	raw, ok := appliedOutput[ekrRotateKeysInstructionName]
	if !ok {
		return ekrCommandResult{}, errRotateKeysOutputNotYet
	}
	message := string(raw)
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ekrExitCodePrefix) {
			continue
		}
		exitCode, err := strconv.Atoi(strings.TrimPrefix(line, ekrExitCodePrefix))
		if err != nil {
			return ekrCommandResult{}, fmt.Errorf("corrupt rotate-keys exit code in output: %w", err)
		}
		return ekrCommandResult{output: message, exitCode: exitCode}, nil
	}
	return ekrCommandResult{}, errRotateKeysOutputNotYet
}

// rotateKeysCommandTimedOut returns true when the rotate-keys output matches the known CLI
// timeout signature for the encrypt/config endpoint.
func rotateKeysCommandTimedOut(output string) bool {
	return ekrCommandTimedOut(output, ekrRotateKeysTimeoutEndpoint)
}

// ekrCommandTimedOut returns true when the output contains a known CLI timeout signature
// for the given endpoint.
func ekrCommandTimedOut(output, endpoint string) bool {
	if output == "" || endpoint == "" {
		return false
	}
	if !strings.Contains(output, ekrRotateKeysTimeoutMessage) || !strings.Contains(output, endpoint) {
		return false
	}
	for _, marker := range ekrTimeoutMarkers {
		if strings.Contains(output, marker) {
			return true
		}
	}
	return false
}

// ekrConvergenceWaitMessage checks whether the periodic secrets-encrypt status on secret has
// reached reencrypt_finished (and, when requireHashMatch is true, hash convergence).
//
// Return values:
//   - ("", nil)    — all criteria met, caller may advance
//   - (msg, nil)   — still waiting, msg describes the reason
//   - ("", err)    — hard error: corrupt payload, plan instruction missing, parse failure,
//     or hash field absent when requireHashMatch is true at reencrypt_finished
func ekrConvergenceWaitMessage(secret *corev1.Secret, requireHashMatch bool) (string, error) {
	periodicOutput, err := planapi.ReadAppliedPeriodicOutput(secret)
	if err != nil {
		// gzip decode or JSON unmarshal failure — corrupt payload, not a normal wait.
		return "", fmt.Errorf("corrupt applied-periodic-output on %s: %w", secret.Name, err)
	}

	var entry planapi.PeriodicInstructionOutput
	var entryOK bool
	if periodicOutput != nil {
		entry, entryOK = periodicOutput[ekrStatusPeriodicName]
	}

	if !entryOK {
		// No output yet, only a wait if the instruction is actually in the assigned plan.
		// Missing instruction means a malformed/regressed plan so fail immediately.
		if err := validateEKRPlanHasPeriodicStatus(secret); err != nil {
			return "", fmt.Errorf("failed reading assigned plan from %s: %w", secret.Name, err)
		}
		return fmt.Sprintf("waiting for secrets-encrypt status on %s", secret.Name), nil
	}

	stdout := strings.TrimSpace(string(entry.Stdout))
	if stdout == "" {
		return fmt.Sprintf("waiting for secrets-encrypt status output on %s", secret.Name), nil
	}

	rotStatus, err := ekrStatusFromOutput(stdout)
	if err != nil {
		if errors.Is(err, errEKRStatusTimeout) {
			// Transient CLI timeout: wait for the next periodic run.
			return fmt.Sprintf("secrets-encrypt status timed out on %s; waiting for next run", secret.Name), nil
		}
		// Durable parse failure (e.g. missing rotation stage): surface as a hard error
		// so the caller can fail the operation cleanly rather than looping indefinitely.
		return "", fmt.Errorf("unable to parse secrets-encrypt status on %s: %w", secret.Name, err)
	}

	if rotStatus.stage != ekrStageReencryptFinished {
		return fmt.Sprintf("waiting for reencrypt_finished on %s, current stage: %s", secret.Name, rotStatus.stage), nil
	}
	if requireHashMatch {
		if !rotStatus.hashesPresent {
			// Hash field absent at reencrypt_finished: the runtime output does not satisfy the
			// convergence contract. Fail rather than waiting indefinitely.
			return "", fmt.Errorf("secrets-encrypt status on %s reached %s but hash field is absent; runtime output may be incompatible", secret.Name, ekrStageReencryptFinished)
		}
		if !rotStatus.hashesMatch {
			return fmt.Sprintf("waiting for encryption key rotation hashes to converge on %s", secret.Name), nil
		}
	}
	return "", nil
}

// validateEKRPlanHasPeriodicStatus reports whether the plan currently assigned to secret includes a
// periodic instruction named ekrStatusPeriodicName. Returns an error when the plan data is
// absent or not decodable.
func validateEKRPlanHasPeriodicStatus(secret *corev1.Secret) error {
	raw := secret.Data["plan"]
	if len(raw) == 0 {
		return fmt.Errorf("plan data absent from %s", secret.Name)
	}
	var p planapi.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("decoding plan from %s: %w", secret.Name, err)
	}
	for _, inst := range p.PeriodicInstructions {
		if inst.Name == ekrStatusPeriodicName {
			return nil
		}
	}
	return fmt.Errorf("periodic instruction %s not present in plan for %s", ekrStatusPeriodicName, secret.Name)
}

// ekrStatusFromOutput parses the human-readable secrets-encrypt status output.
// Returns errEKRStatusTimeout for transient CLI timeouts.
// Returns a non-sentinel error when the rotation stage is missing.
func ekrStatusFromOutput(output string) (ekrRuntimeStatus, error) {
	// A timed-out status call is transient; wait for the next periodic run.
	if ekrCommandTimedOut(output, ekrStatusTimeoutEndpoint) {
		return ekrRuntimeStatus{}, fmt.Errorf("%w; waiting for next run", errEKRStatusTimeout)
	}

	var result ekrRuntimeStatus
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
			result.hashesMatch = strings.TrimSpace(value) == ekrHashesMatch
		}
	}

	if result.stage == "" {
		return ekrRuntimeStatus{}, fmt.Errorf("unable to parse rotation stage from secrets-encrypt status output")
	}
	return result, nil
}
