package planner

import (
	"encoding/base64"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

const (
	// start is the pre-rotation steady state reported by current K3s/RKE2 status output. Rancher does not branch on it
	// directly, but keeping it named documents the expected intermediate state we wait through before convergence.
	encryptionKeyRotationStageStart             = "start"
	encryptionKeyRotationStageReencryptFinished = "reencrypt_finished"
	encryptionKeyRotationHashesMatch            = "All hashes match"

	encryptionKeyRotationCommandRotateKeys = "rotate-keys"

	encryptionKeyRotationSecretsEncryptStatusCommand = "secrets-encrypt-status"
	encryptionKeyRotationRotateKeysTimeoutMessage    = "see server log for details"
	encryptionKeyRotationRotateKeysErrorIDMessage    = "secret-encrypt error ID"
	encryptionKeyRotationRotateKeysTimeoutEndpoint   = "/encrypt/config"
	encryptionKeyRotationStatusTimeoutEndpoint       = "/encrypt/status"

	encryptionKeyRotationBinPrefix = "capr/encryption-key-rotation/bin"

	encryptionKeyRotationWaitForSystemctlStatusPath      = "wait_for_systemctl_status.sh"
	encryptionKeyRotationWaitForSecretsEncryptStatusPath = "wait_for_secrets_encrypt_status.sh"

	encryptionKeyRotationWaitForSystemctlStatus = `
#!/bin/sh

runtimeServer=$1
i=0

while [ $i -lt 30 ]; do
	systemctl is-active $runtimeServer
	if [ $? -eq 0 ]; then
		exit 0
	fi
	sleep 10
	i=$((i + 1))
done
exit 1
`
	encryptionKeyRotationWaitForSecretsEncryptStatusScript = `
#!/bin/sh

runtime=$1
i=0

while [ $i -lt 10 ]; do
	$runtime secrets-encrypt status
	if [ $? -eq 0 ]; then
		exit 0
	fi
	sleep 10
	i=$((i + 1))
done
exit 1
`

	encryptionKeyRotationEndpointEnv       = "CONTAINER_RUNTIME_ENDPOINT=unix:///var/run/k3s/containerd/containerd.sock"
	encryptionKeyRotationGenerationEnvName = "ENCRYPTION_KEY_ROTATION_GENERATION"
	encryptionKeyRotationAttemptEnvName    = "ENCRYPTION_KEY_ROTATION_ATTEMPT"
	encryptionKeyRotationRetryCountEnv     = "ENCRYPTION_KEY_ROTATION_RETRY_COUNT"

	encryptionKeyRotationMaxRotateKeysRetries = 3
)

var encryptionKeyRotationMinimumVersion = semver.Version{Major: 1, Minor: 30}

var encryptionKeyRotationTimeoutMarkers = []string{
	"Client.Timeout exceeded while awaiting headers",
	"timeout awaiting response headers",
	"context deadline exceeded",
}

type encryptionKeyRotationRuntimeStatus struct {
	Stage       string
	HashesMatch bool
}

type encryptionKeyRotationRestartTargets struct {
	etcdOnly     []*planEntry
	controlPlane []*planEntry
}

func (restartTargets encryptionKeyRotationRestartTargets) count() int {
	return len(restartTargets.etcdOnly) + len(restartTargets.controlPlane)
}

func (p *Planner) setEncryptionKeyRotateState(status rkev1.RKEControlPlaneStatus, rotate *rkev1.RotateEncryptionKeys, phase rkev1.RotateEncryptionKeysPhase) (rkev1.RKEControlPlaneStatus, error) {
	if equality.Semantic.DeepEqual(status.RotateEncryptionKeys, rotate) && equality.Semantic.DeepEqual(status.RotateEncryptionKeysPhase, phase) {
		return status, nil
	}
	status.RotateEncryptionKeys = rotate
	status.RotateEncryptionKeysPhase = phase
	return status, errWaiting("refreshing encryption key rotation state")
}

func (p *Planner) resetEncryptionKeyRotateState(status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if status.RotateEncryptionKeys == nil && status.RotateEncryptionKeysPhase == "" && status.RotateEncryptionKeysLeader == "" {
		return status, nil
	}
	status.RotateEncryptionKeys = nil
	status.RotateEncryptionKeysPhase = ""
	status.RotateEncryptionKeysLeader = ""
	return status, errWaiting("refreshing encryption key rotation state")
}

// rotateEncryptionKeys first verifies that the control plane is in a state where the next step can be derived. If encryption key rotation is required, the corresponding phase and status fields will be set.
// The function is expected to be called multiple times throughout encryption key rotation, and will set the next corresponding phase based on previous output.
func (p *Planner) rotateEncryptionKeys(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	if controlPlane == nil || clusterPlan == nil {
		return status, fmt.Errorf("cannot pass nil parameters to rotateEncryptionKeys")
	}

	if controlPlane.Spec.RotateEncryptionKeys == nil {
		return p.resetEncryptionKeyRotateState(status)
	}

	if supported, err := encryptionKeyRotationSupported(controlPlane); err != nil {
		status, err = p.encryptionKeyRotationFailed(status, err)
		return p.encryptionKeyRotationHandleFailure(controlPlane, status, err)
	} else if !supported {
		logrus.Debugf("rkecluster %s/%s: marking encryption key rotation phase as failed as it was not supported by version: %s", controlPlane.Namespace, controlPlane.Name, controlPlane.Spec.KubernetesVersion)
		if err := p.pauseCAPICluster(controlPlane, false); err != nil {
			return status, errWaiting("unpausing CAPI cluster")
		}
		status.RotateEncryptionKeysLeader = ""
		return p.setEncryptionKeyRotateState(status, controlPlane.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseFailed)
	}

	if encryptionKeyRotationShouldSkip(controlPlane) {
		return status, nil
	}

	if !ptr.Deref(status.Initialization.ControlPlaneInitialized, false) {
		// cluster is not yet initialized, so return nil for now.
		logrus.Warnf("[planner] rkecluster %s/%s: skipping encryption key rotation as cluster was not initialized", controlPlane.Namespace, controlPlane.Name)
		return status, nil
	}

	found, joinServer, initNode, err := p.findInitNode(controlPlane, clusterPlan)
	if err != nil {
		logrus.Errorf("[planner] rkecluster %s/%s: error encountered while searching for init node during encryption key rotation: %v", controlPlane.Namespace, controlPlane.Name, err)
		return status, err
	}
	if !found || joinServer == "" {
		logrus.Warnf("[planner] rkecluster %s/%s: skipping encryption key rotation as cluster does not have an init node", controlPlane.Namespace, controlPlane.Name)
		return status, nil
	}

	shouldStart, err := encryptionKeyRotationShouldStart(controlPlane)
	if err != nil {
		status, err = p.encryptionKeyRotationFailed(status, err)
		return p.encryptionKeyRotationHandleFailure(controlPlane, status, err)
	}

	if shouldStart {
		logrus.Debugf("[planner] rkecluster %s/%s: starting/restarting encryption key rotation", controlPlane.Namespace, controlPlane.Name)
		return p.setEncryptionKeyRotateState(status, controlPlane.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseRotate)
	}

	// The stored leader is part of Rancher's idempotency contract. rotate-keys must only be launched from one server,
	// and once it has been chosen we keep reconciling through that server until the generation either finishes or fails.
	leader, err := p.encryptionKeyRotationFindLeader(status, clusterPlan, initNode)
	if err != nil {
		status, err = p.encryptionKeyRotationFailed(status, err)
		return p.encryptionKeyRotationHandleFailure(controlPlane, status, err)
	}

	if status.RotateEncryptionKeysLeader != leader.Machine.Name {
		status.RotateEncryptionKeysLeader = leader.Machine.Name
		return status, errWaitingf("elected %s as control plane leader for encryption key rotation", leader.Machine.Name)
	}

	logrus.Debugf("[planner] rkecluster %s/%s: current encryption key rotation phase: [%s]", controlPlane.Namespace, controlPlane.Spec.ClusterName, controlPlane.Status.RotateEncryptionKeysPhase)

	switch controlPlane.Status.RotateEncryptionKeysPhase {
	case rkev1.RotateEncryptionKeysPhaseRotate:
		if err := p.pauseCAPICluster(controlPlane, true); err != nil {
			return status, errWaiting("pausing CAPI cluster")
		}

		rotationStatus, status, err := p.encryptionKeyRotationRotateKeysReconcile(controlPlane, status, tokensSecret, joinServer, leader)
		if err != nil {
			return p.encryptionKeyRotationHandleFailure(controlPlane, status, err)
		}
		if rotationStatus.Stage != encryptionKeyRotationStageReencryptFinished {
			return status, errWaitingf("waiting for encryption key rotation stage to finish on leader [%s]", leader.Machine.Name)
		}

		restartTargets := encryptionKeyRotationRestartTargetsForCluster(controlPlane, clusterPlan, leader, initNode)
		if restartTargets.count() == 1 {
			if !rotationStatus.HashesMatch {
				return status, errWaitingf("waiting for encryption key rotation hashes to converge on leader [%s]", leader.Machine.Name)
			}
			if err := p.pauseCAPICluster(controlPlane, false); err != nil {
				return status, errWaiting("unpausing CAPI cluster")
			}
			status.RotateEncryptionKeysLeader = ""
			return p.setEncryptionKeyRotateState(status, controlPlane.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseDone)
		}

		return p.setEncryptionKeyRotateState(status, controlPlane.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhasePostRotateRestart)
	case rkev1.RotateEncryptionKeysPhasePostRotateRestart:
		status, err = p.encryptionKeyRotationRestartNodes(controlPlane, status, tokensSecret, clusterPlan, leader, initNode, joinServer)
		if err != nil {
			return p.encryptionKeyRotationHandleFailure(controlPlane, status, err)
		}
		if err = p.pauseCAPICluster(controlPlane, false); err != nil {
			return status, errWaiting("unpausing CAPI cluster")
		}
		status.RotateEncryptionKeysLeader = ""
		return p.setEncryptionKeyRotateState(status, controlPlane.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseDone)
	}

	status, err = p.encryptionKeyRotationFailed(status, fmt.Errorf("encountered unknown encryption key rotation phase: %s", controlPlane.Status.RotateEncryptionKeysPhase))
	return p.encryptionKeyRotationHandleFailure(controlPlane, status, err)
}

// encryptionKeyRotationSupported returns a boolean indicating whether encryption key rotation is supported for the control plane version,
// and an error if one was encountered.
func encryptionKeyRotationSupported(controlPlane *rkev1.RKEControlPlane) (bool, error) {
	if controlPlane == nil {
		return false, fmt.Errorf("unable to determine encryption key rotation support for nil control plane")
	}

	kubernetesVersion, err := semver.Make(strings.TrimPrefix(controlPlane.Spec.KubernetesVersion, "v"))
	if err != nil {
		return false, fmt.Errorf("unable to parse kubernetes version for encryption key rotation: %s", controlPlane.Spec.KubernetesVersion)
	}
	// Older Rancher releases relied on the KDM encryption-key-rotation feature gate because the legacy orchestration
	// could not be enabled retroactively for all historical versions without risking unrecoverable clusters. Rancher
	// v2.15 only supports downstream v1.30+ and only uses the newer rotate-keys flow, so the Kubernetes version itself
	// is the compatibility boundary for this planner path.
	if kubernetesVersion.LT(encryptionKeyRotationMinimumVersion) {
		return false, nil
	}

	return true, nil
}

// encryptionKeyRotationShouldSkip returns true when the planner should ignore the current spec/status combination:
// either the control plane is not ready and rotation is not already in progress, or the requested generation already
// finished and should not be re-run.
func encryptionKeyRotationShouldSkip(controlPlane *rkev1.RKEControlPlane) bool {
	if controlPlane == nil || controlPlane.Spec.RotateEncryptionKeys == nil {
		return true
	}

	phase := controlPlane.Status.RotateEncryptionKeysPhase
	inProgress := phase == rkev1.RotateEncryptionKeysPhaseRotate || phase == rkev1.RotateEncryptionKeysPhasePostRotateRestart
	if !capr.Ready.IsTrue(controlPlane) && !inProgress {
		return true
	}

	return controlPlane.Status.RotateEncryptionKeys != nil &&
		controlPlane.Status.RotateEncryptionKeys.Generation == controlPlane.Spec.RotateEncryptionKeys.Generation &&
		(phase == rkev1.RotateEncryptionKeysPhaseDone || phase == rkev1.RotateEncryptionKeysPhaseFailed)
}

// encryptionKeyRotationShouldStart collapses the phase/generation restart logic for the simplified rotate-keys flow.
// It returns true when a new reconciliation should initialize the Rotate phase, and returns an error if the current
// generation is stuck in an unsupported legacy phase.
func encryptionKeyRotationShouldStart(controlPlane *rkev1.RKEControlPlane) (bool, error) {
	if controlPlane == nil || controlPlane.Spec.RotateEncryptionKeys == nil {
		return false, nil
	}
	if controlPlane.Status.RotateEncryptionKeys == nil || controlPlane.Status.RotateEncryptionKeysPhase == "" {
		return true, nil
	}
	if controlPlane.Spec.RotateEncryptionKeys.Generation != controlPlane.Status.RotateEncryptionKeys.Generation {
		return controlPlane.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone ||
			controlPlane.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseFailed ||
			!encryptionKeyRotationPhaseIsKnown(controlPlane.Status.RotateEncryptionKeysPhase), nil
	}
	// Rancher v2.15 only understands the simplified rotate-keys flow. If we ever see a stale legacy phase for the
	// generation currently being reconciled, fail it explicitly instead of trying to map it back into the new flow.
	if encryptionKeyRotationPhaseIsKnown(controlPlane.Status.RotateEncryptionKeysPhase) {
		return false, nil
	}
	return false, fmt.Errorf("unsupported encryption key rotation phase [%s]", controlPlane.Status.RotateEncryptionKeysPhase)
}

func encryptionKeyRotationPhaseIsKnown(phase rkev1.RotateEncryptionKeysPhase) bool {
	switch phase {
	case "",
		rkev1.RotateEncryptionKeysPhaseRotate,
		rkev1.RotateEncryptionKeysPhasePostRotateRestart,
		rkev1.RotateEncryptionKeysPhaseDone,
		rkev1.RotateEncryptionKeysPhaseFailed:
		return true
	default:
		return false
	}
}

// encryptionKeyRotationFindLeader returns the current encryption rotation leader if it is valid, otherwise, if the
// phase is "rotate", it will re-elect a new leader. It will look for the init node, and if the init node is not valid
// (etcd-only), it will elect the first suitable control plane node. If the phase is not in "rotate" and a re-election
// of the leader is necessary, the phase will be set to failed as this is unexpected.
func (p *Planner) encryptionKeyRotationFindLeader(status rkev1.RKEControlPlaneStatus, clusterPlan *plan.Plan, initNode *planEntry) (*planEntry, error) {
	machineName := status.RotateEncryptionKeysLeader
	if machine, ok := clusterPlan.Machines[machineName]; ok {
		entry := &planEntry{
			Machine:  machine,
			Plan:     clusterPlan.Nodes[machineName],
			Metadata: clusterPlan.Metadata[machineName],
		}
		if encryptionKeyRotationIsSuitableControlPlane(entry) {
			return entry, nil
		}
	}

	if status.RotateEncryptionKeysPhase != rkev1.RotateEncryptionKeysPhaseRotate {
		return nil, fmt.Errorf("cannot elect control plane leader in phase %s", status.RotateEncryptionKeysPhase)
	}

	leader := initNode
	if !isControlPlane(initNode) {
		machines := collect(clusterPlan, encryptionKeyRotationIsSuitableControlPlane)
		if len(machines) == 0 {
			return nil, fmt.Errorf("no suitable control plane nodes for encryption key rotation")
		}
		leader = machines[0]
	}

	return leader, nil
}

// encryptionKeyRotationIsSuitableControlPlane ensures that a control plane node has not been deleted and has a valid
// node associated with it.
func encryptionKeyRotationIsSuitableControlPlane(entry *planEntry) bool {
	return isControlPlane(entry) && isNotDeleting(entry) && entry.Machine.Status.NodeRef.IsDefined() && capr.Ready.IsTrue(entry.Machine)
}

// encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit allows us to filter cluster plans to restart healthy follower nodes.
func encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit(controlPlane *rkev1.RKEControlPlane) roleFilter {
	return func(entry *planEntry) bool {
		return isControlPlaneAndNotInitNode(entry) &&
			controlPlane.Status.RotateEncryptionKeysLeader != entry.Machine.Name
	}
}

// encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit allows us to filter cluster plans to restart healthy follower nodes.
func encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit(controlPlane *rkev1.RKEControlPlane) roleFilter {
	return func(entry *planEntry) bool {
		return isEtcd(entry) && !isControlPlane(entry) &&
			controlPlane.Status.RotateEncryptionKeysLeader != entry.Machine.Name &&
			!isInitNode(entry)
	}
}

// encryptionKeyRotationRestartTargetsForCluster centralizes the restart topology for split-role clusters.
// The etcd-only and control-plane restart slices are kept separate because only control-plane nodes can be used as
// secrets-encrypt status sources during convergence, while etcd-only nodes still need to participate in the restart pass.
func encryptionKeyRotationRestartTargetsForCluster(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan, leader, initNode *planEntry) encryptionKeyRotationRestartTargets {
	targets := encryptionKeyRotationRestartTargets{
		controlPlane: []*planEntry{leader},
	}

	if initNode != nil && !isInitNode(leader) {
		if isControlPlane(initNode) {
			targets.controlPlane = append([]*planEntry{initNode}, targets.controlPlane...)
		} else {
			targets.etcdOnly = append(targets.etcdOnly, initNode)
		}
	}

	// Upstream documents restarting the server that ran rotate-keys before the remaining servers. Rancher follows that
	// ordering after the init node when they are different, so split-role clusters still bring the designated init server
	// back first and then restart the elected command leader before the rest of the high-availability servers.
	targets.etcdOnly = append(targets.etcdOnly, collect(clusterPlan, encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit(controlPlane))...)
	targets.controlPlane = append(targets.controlPlane, collect(clusterPlan, encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit(controlPlane))...)

	return targets
}

// encryptionKeyRotationRestartNodes restarts the required server nodes and waits for control plane nodes to converge.
// Etcd-only nodes are restarted first. Rancher does not use them as local secrets-encrypt status sources during
// convergence because the local encrypt status handler depends on Runtime.Core, which is not initialized on
// disable-apiserver servers.
// Control plane nodes are then restarted in order, with each required to reach the reencrypt_finished stage and the
// last one also required to report matching hashes.
func (p *Planner) encryptionKeyRotationRestartNodes(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan, leader *planEntry, initNode *planEntry, joinServer string) (rkev1.RKEControlPlaneStatus, error) {
	restartTargets := encryptionKeyRotationRestartTargetsForCluster(controlPlane, clusterPlan, leader, initNode)

	// Restart etcd-only nodes first. These nodes do not produce usable local status output after restart,
	// so they only participate in the restart portion of the flow.
	for _, entry := range restartTargets.etcdOnly {
		_, updatedStatus, err := p.encryptionKeyRotationRestartService(controlPlane, status, tokensSecret, joinServer, entry)
		if err != nil {
			return updatedStatus, err
		}
		status = updatedStatus
	}

	// Restart control plane nodes in order and verify secrets-encrypt status convergence.
	for i, entry := range restartTargets.controlPlane {
		rotationStatus, updatedStatus, err := p.encryptionKeyRotationRestartService(controlPlane, status, tokensSecret, joinServer, entry)
		if err != nil {
			return updatedStatus, err
		}
		status = updatedStatus

		if rotationStatus.Stage != encryptionKeyRotationStageReencryptFinished {
			return status, errWaitingf("waiting for encryption key rotation stage to finish on machine [%s]", entry.Machine.Name)
		}

		if i == len(restartTargets.controlPlane)-1 && !rotationStatus.HashesMatch {
			return status, errWaitingf("waiting for encryption key rotation hashes to converge on machine [%s]", entry.Machine.Name)
		}
	}

	return status, nil
}

// encryptionKeyRotationRestartService restarts the server unit on the downstream node, waits until secrets-encrypt
// status can be successfully queried, and then returns the current runtime status from the periodic status output.
// For etcd-only nodes (non control plane), the restart is performed but no runtime status is returned. The local
// secrets-encrypt status endpoint requires Runtime.Core, which is not initialized on disable-apiserver servers.
func (p *Planner) encryptionKeyRotationRestartService(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, joinServer string, entry *planEntry) (encryptionKeyRotationRuntimeStatus, rkev1.RKEControlPlaneStatus, error) {
	nodePlan, joinedServer, err := p.encryptionKeyRotationRestartPlan(controlPlane, tokensSecret, joinServer, entry)
	if err != nil {
		return encryptionKeyRotationRuntimeStatus{}, status, err
	}

	err = assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation [%s] for machine [%s]", controlPlane.Status.RotateEncryptionKeysPhase, entry.Machine.Name), entry, nodePlan, joinedServer, 5, 5)
	if err != nil {
		if IsErrWaiting(err) {
			if planAppliedButWaitingForProbes(entry) {
				return encryptionKeyRotationRuntimeStatus{}, status, errWaitingf("%s: %s", err.Error(), probesMessage(entry.Plan))
			}
			return encryptionKeyRotationRuntimeStatus{}, status, err
		}
		status, err = p.encryptionKeyRotationFailed(status, err)
		return encryptionKeyRotationRuntimeStatus{}, status, err
	}

	if !isControlPlane(entry) {
		return encryptionKeyRotationRuntimeStatus{}, status, nil
	}

	rotationStatus, err := encryptionKeyRotationSecretsEncryptStatusFromPeriodic(entry)
	if err != nil {
		return encryptionKeyRotationRuntimeStatus{}, status, err
	}

	return rotationStatus, status, nil
}

// encryptionKeyRotationRestartPlan creates a restart-only plan. During the high-availability convergence step Rancher no longer runs
// additional secrets-encrypt subcommands on follower nodes; it only restarts the server unit and resumes observing the
// periodic secrets-encrypt status that the system-agent reports back in the node plan.
func (p *Planner) encryptionKeyRotationRestartPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, joinServer string, entry *planEntry) (plan.NodePlan, string, error) {
	nodePlan, config, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, entry, joinServer, true)
	if err != nil {
		return plan.NodePlan{}, "", err
	}
	generation := encryptionKeyRotationActiveGeneration(controlPlane)
	generationValue := strconv.FormatInt(generation, 10)

	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSystemctlStatus)),
		Path:    encryptionKeyRotationScriptPath(controlPlane, encryptionKeyRotationWaitForSystemctlStatusPath),
	})

	nodePlan.Instructions = []plan.OneTimeInstruction{}

	if capr.GetRuntime(controlPlane.Spec.KubernetesVersion) == capr.RuntimeRKE2 {
		if generated, instruction := generateManifestRemovalInstruction(controlPlane, entry); generated {
			nodePlan.Instructions = append(nodePlan.Instructions, convertToIdempotentInstruction(
				controlPlane,
				strings.ToLower(fmt.Sprintf("encryption-key-rotation/manifest-cleanup/%s", controlPlane.Status.RotateEncryptionKeysPhase)),
				generationValue,
				instruction))
		}
	}

	nodePlan.Instructions = append(nodePlan.Instructions, idempotentRestartInstructions(
		controlPlane,
		strings.ToLower(fmt.Sprintf("encryption-key-rotation/restart/%s", controlPlane.Status.RotateEncryptionKeysPhase)),
		generationValue,
		capr.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion))...)
	nodePlan.Instructions = append(nodePlan.Instructions,
		encryptionKeyRotationWaitForSystemctlStatusInstruction(controlPlane),
	)

	// Only control plane nodes get local status polling. Rancher relies on them for convergence because etcd-only
	// servers cannot satisfy the local encrypt status endpoint after restart.
	if isControlPlane(entry) {
		nodePlan.Files = append(nodePlan.Files, plan.File{
			Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSecretsEncryptStatusScript)),
			Path:    encryptionKeyRotationScriptPath(controlPlane, encryptionKeyRotationWaitForSecretsEncryptStatusPath),
		})
		nodePlan.Instructions = append(nodePlan.Instructions,
			encryptionKeyRotationWaitForSecretsEncryptStatus(controlPlane),
		)
		nodePlan.PeriodicInstructions = []plan.PeriodicInstruction{
			encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction(controlPlane),
		}
	}

	probes, err := p.generateProbes(controlPlane, entry, config)
	if err != nil {
		return plan.NodePlan{}, "", err
	}
	nodePlan.Probes = probes

	return nodePlan, joinedServer, nil
}

// encryptionKeyRotationRotateKeysReconcile runs the rotate-keys command on the elected leader and returns the most
// recent periodic secrets-encrypt status observed on that node.
func (p *Planner) encryptionKeyRotationRotateKeysReconcile(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, joinServer string, leader *planEntry) (encryptionKeyRotationRuntimeStatus, rkev1.RKEControlPlaneStatus, error) {
	retryCount := encryptionKeyRotationRotateKeysRetryCount(controlPlane, leader)
	nodePlan, joinedServer, err := p.encryptionKeyRotationRotateKeysPlanWithRetryCount(controlPlane, tokensSecret, joinServer, leader, retryCount)
	if err != nil {
		return encryptionKeyRotationRuntimeStatus{}, status, err
	}

	if encryptionKeyRotationRotateKeysFailedWithRetryablePrecondition(leader, nodePlan.Instructions[0].Name) {
		rotationStatus, periodicStatusErr := encryptionKeyRotationSecretsEncryptStatusFromPeriodic(leader)
		if periodicStatusErr != nil {
			return encryptionKeyRotationRuntimeStatus{}, status, errWaitingf("waiting for encryption key rotation status after retryable rotate-keys failure on leader [%s]", leader.Machine.Name)
		}
		if !encryptionKeyRotationRotateKeysCanRetry(rotationStatus) {
			logrus.Warnf("[planner] rkecluster %s/%s: rotate-keys command on leader [%s] failed with a retryable precondition and current periodic status is stage [%s] with hashesMatch=%t; waiting to retry", controlPlane.Namespace, controlPlane.Spec.ClusterName, leader.Machine.Name, rotationStatus.Stage, rotationStatus.HashesMatch)
			return encryptionKeyRotationRuntimeStatus{}, status, errWaitingf("waiting for leader [%s] to reach a stable secrets-encrypt state before retrying rotate-keys", leader.Machine.Name)
		}
		if !encryptionKeyRotationRotateKeysCanRetryAgain(retryCount) {
			logrus.Warnf("[planner] rkecluster %s/%s: rotate-keys command on leader [%s] failed with a retryable precondition after retry count [%d]; surfacing the failure instead of retrying indefinitely", controlPlane.Namespace, controlPlane.Spec.ClusterName, leader.Machine.Name, retryCount)
		} else {
			retryCount = encryptionKeyRotationNextRotateKeysRetryCount(retryCount)
			nodePlan, joinedServer, err = p.encryptionKeyRotationRotateKeysPlanWithRetryCount(controlPlane, tokensSecret, joinServer, leader, retryCount)
			if err != nil {
				return encryptionKeyRotationRuntimeStatus{}, status, err
			}

			logrus.Warnf("[planner] rkecluster %s/%s: rotate-keys command on leader [%s] failed with a retryable precondition; reissuing rotate-keys once periodic status returned to stage [%s] with converged hashes", controlPlane.Namespace, controlPlane.Spec.ClusterName, leader.Machine.Name, rotationStatus.Stage)
		}
	}

	err = assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation [%s] for machine [%s]", controlPlane.Status.RotateEncryptionKeysPhase, leader.Machine.Name), leader, nodePlan, joinedServer, 1, 1)
	if err != nil {
		if IsErrWaiting(err) {
			if strings.HasPrefix(err.Error(), "starting") {
				logrus.Infof("[planner] rkecluster %s/%s: applying encryption key rotation stage command: [%s]", controlPlane.Namespace, controlPlane.Spec.ClusterName, encryptionKeyRotationCommandRotateKeys)
			}
			return encryptionKeyRotationRuntimeStatus{}, status, err
		}
		if encryptionKeyRotationRotateKeysTimedOut(leader, nodePlan.Instructions[0].Name) {
			logrus.Warnf("[planner] rkecluster %s/%s: rotate-keys command on leader [%s] exceeded the CLI timeout; continuing to observe periodic status because rotation may still be running in the background", controlPlane.Namespace, controlPlane.Spec.ClusterName, leader.Machine.Name)
			rotationStatus, periodicStatusErr := encryptionKeyRotationSecretsEncryptStatusFromPeriodic(leader)
			if periodicStatusErr == nil {
				return rotationStatus, status, nil
			}
			return encryptionKeyRotationRuntimeStatus{}, status, errWaitingf("waiting for encryption key rotation status after rotate-keys timeout on leader [%s]", leader.Machine.Name)
		}
		status, err = p.encryptionKeyRotationFailed(status, err)
		return encryptionKeyRotationRuntimeStatus{}, status, err
	}

	rotationStatus, err := encryptionKeyRotationSecretsEncryptStatusFromPeriodic(leader)
	if err != nil {
		return encryptionKeyRotationRuntimeStatus{}, status, err
	}

	logrus.Infof("[planner] rkecluster %s/%s: successfully applied encryption key rotation stage command: [%s]", controlPlane.Namespace, controlPlane.Spec.ClusterName, encryptionKeyRotationCommandRotateKeys)
	return rotationStatus, status, nil
}

// encryptionKeyRotationRotateKeysPlan keeps only the rotate-keys command as a one-time instruction and relies on
// periodic status output for progress. That lets the planner resume cleanly after controller restarts without having to
// preserve one-time status scraping state from an earlier reconcile.
func (p *Planner) encryptionKeyRotationRotateKeysPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, joinServer string, leader *planEntry) (plan.NodePlan, string, error) {
	return p.encryptionKeyRotationRotateKeysPlanWithRetryCount(controlPlane, tokensSecret, joinServer, leader, encryptionKeyRotationRotateKeysRetryCount(controlPlane, leader))
}

func (p *Planner) encryptionKeyRotationRotateKeysPlanWithRetryCount(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, joinServer string, leader *planEntry, retryCount int) (plan.NodePlan, string, error) {
	nodePlan, _, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, leader, joinServer, true)
	if err != nil {
		return plan.NodePlan{}, "", err
	}

	apply, err := encryptionKeyRotationSecretsEncryptInstructionWithRetryCount(controlPlane, retryCount)
	if err != nil {
		return plan.NodePlan{}, "", err
	}

	nodePlan.Instructions = []plan.OneTimeInstruction{apply}
	nodePlan.PeriodicInstructions = []plan.PeriodicInstruction{
		encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction(controlPlane),
	}

	return nodePlan, joinedServer, nil
}

// encryptionKeyRotationSecretsEncryptStatusFromPeriodic will attempt to extract the current secrets-encrypt status from the
// plan by parsing the periodic output.
func encryptionKeyRotationSecretsEncryptStatusFromPeriodic(plan *planEntry) (encryptionKeyRotationRuntimeStatus, error) {
	output, ok := plan.Plan.PeriodicOutput[encryptionKeyRotationSecretsEncryptStatusCommand]
	if !ok {
		for _, periodicInstruction := range plan.Plan.Plan.PeriodicInstructions {
			if periodicInstruction.Name == encryptionKeyRotationSecretsEncryptStatusCommand {
				return encryptionKeyRotationRuntimeStatus{}, errWaitingf("could not extract current status from plan for [%s]: no output for status", plan.Machine.Name)
			}
		}
		return encryptionKeyRotationRuntimeStatus{}, fmt.Errorf("could not extract current status from plan for [%s]: status command not present in plan", plan.Machine.Name)
	}

	stdout := strings.TrimSpace(string(output.Stdout))
	stderr := strings.TrimSpace(string(output.Stderr))

	switch {
	case stdout != "" && stderr != "":
		return encryptionKeyRotationStatusFromOutput(plan, stdout+"\n"+stderr)
	case stdout != "":
		return encryptionKeyRotationStatusFromOutput(plan, stdout)
	default:
		return encryptionKeyRotationStatusFromOutput(plan, stderr)
	}
}

// encryptionKeyRotationStatusFromOutput parses the parts of secrets-encrypt status that Rancher actually reconciles on:
// the current rotation stage and whether the server hash set has converged.
func encryptionKeyRotationStatusFromOutput(plan *planEntry, output string) (encryptionKeyRotationRuntimeStatus, error) {
	var status encryptionKeyRotationRuntimeStatus

	if encryptionKeyRotationCommandTimedOut(output, encryptionKeyRotationStatusTimeoutEndpoint) {
		return encryptionKeyRotationRuntimeStatus{}, errWaitingf("waiting for secrets-encrypt status after transient timeout")
	}

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "Current Rotation Stage":
			status.Stage = value
		case "Server Encryption Hashes":
			status.HashesMatch = value == encryptionKeyRotationHashesMatch
		}
	}

	if status.Stage == "" {
		return encryptionKeyRotationRuntimeStatus{}, errWaitingf("unable to parse rotation stage from output")
	}
	if !strings.Contains(output, "Server Encryption Hashes:") {
		return encryptionKeyRotationRuntimeStatus{}, errWaitingf("unable to parse rotation hashes from output")
	}

	return status, nil
}

// encryptionKeyRotationActiveGeneration returns the generation that should be embedded in node plans.
// Once a rotation has started, the planner must stay pinned to status.RotateEncryptionKeys.Generation until
// that generation either completes or fails, even if spec has already been bumped to request a later one.
func encryptionKeyRotationActiveGeneration(controlPlane *rkev1.RKEControlPlane) int64 {
	if controlPlane == nil || controlPlane.Spec.RotateEncryptionKeys == nil {
		return 0
	}

	if controlPlane.Status.RotateEncryptionKeys != nil {
		switch controlPlane.Status.RotateEncryptionKeysPhase {
		case rkev1.RotateEncryptionKeysPhaseRotate, rkev1.RotateEncryptionKeysPhasePostRotateRestart:
			return controlPlane.Status.RotateEncryptionKeys.Generation
		}
	}

	return controlPlane.Spec.RotateEncryptionKeys.Generation
}

func encryptionKeyRotationRotateKeysInstructionValue(controlPlane *rkev1.RKEControlPlane, retryCount int) string {
	return fmt.Sprintf("%d-retry-%d", encryptionKeyRotationActiveGeneration(controlPlane), retryCount)
}

func encryptionKeyRotationRotateKeysRetryCount(controlPlane *rkev1.RKEControlPlane, leader *planEntry) int {
	activeGeneration := encryptionKeyRotationActiveGeneration(controlPlane)
	plannedGeneration, retryCount, ok := encryptionKeyRotationRotateKeysRetryCountFromPlan(leader)
	if ok && plannedGeneration == activeGeneration {
		return retryCount
	}

	return 0
}

func encryptionKeyRotationRotateKeysRetryCountFromPlan(entry *planEntry) (int64, int, bool) {
	if entry == nil || entry.Plan == nil {
		return 0, 0, false
	}

	for _, instruction := range entry.Plan.Plan.Instructions {
		if !strings.HasPrefix(instruction.Name, "idempotent-encryption-key-rotation/rotate-") {
			continue
		}

		var generation int64
		var retryCount int
		var foundGeneration, foundRetryCount bool
		var legacyAttempt string

		for _, env := range instruction.Env {
			switch {
			case strings.HasPrefix(env, encryptionKeyRotationGenerationEnvName+"="):
				value := strings.TrimPrefix(env, encryptionKeyRotationGenerationEnvName+"=")
				parsedGeneration, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return 0, 0, false
				}
				generation = parsedGeneration
				foundGeneration = true
			case strings.HasPrefix(env, encryptionKeyRotationRetryCountEnv+"="):
				value := strings.TrimPrefix(env, encryptionKeyRotationRetryCountEnv+"=")
				parsedRetryCount, err := strconv.Atoi(value)
				if err != nil || parsedRetryCount < 0 {
					return 0, 0, false
				}
				retryCount = parsedRetryCount
				foundRetryCount = true
			case strings.HasPrefix(env, encryptionKeyRotationAttemptEnvName+"="):
				legacyAttempt = strings.TrimPrefix(env, encryptionKeyRotationAttemptEnvName+"=")
			}
		}

		if foundGeneration && foundRetryCount {
			return generation, retryCount, true
		}
		if legacyAttempt != "" {
			parsedGeneration, parsedRetryCount, ok := encryptionKeyRotationLegacyRetryCount(legacyAttempt)
			if ok {
				return parsedGeneration, parsedRetryCount, true
			}
		}
	}

	return 0, 0, false
}

func encryptionKeyRotationLegacyRetryCount(attempt string) (int64, int, bool) {
	if attempt == "" {
		return 0, 0, false
	}

	parts := strings.Split(attempt, "-")
	generation, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}

	retryCount := 0
	for _, part := range parts[1:] {
		if part != "retry" {
			return 0, 0, false
		}
		retryCount++
	}

	return generation, retryCount, true
}

func encryptionKeyRotationNextRotateKeysRetryCount(retryCount int) int {
	return retryCount + 1
}

func encryptionKeyRotationRotateKeysCanRetryAgain(retryCount int) bool {
	return retryCount < encryptionKeyRotationMaxRotateKeysRetries
}

func encryptionKeyRotationRotateKeysRetryCountEnv(retryCount int) string {
	return fmt.Sprintf("%s=%d", encryptionKeyRotationRetryCountEnv, retryCount)
}

// encryptionKeyRotationSecretsEncryptInstruction generates a secrets-encrypt command to run on the leader node given
// the current secrets-encrypt phase.
func encryptionKeyRotationSecretsEncryptInstruction(controlPlane *rkev1.RKEControlPlane) (plan.OneTimeInstruction, error) {
	return encryptionKeyRotationSecretsEncryptInstructionWithRetryCount(controlPlane, 0)
}

func encryptionKeyRotationSecretsEncryptInstructionWithRetryCount(controlPlane *rkev1.RKEControlPlane, retryCount int) (plan.OneTimeInstruction, error) {
	if controlPlane == nil {
		return plan.OneTimeInstruction{}, fmt.Errorf("control plane cannot be nil")
	}
	if controlPlane.Status.RotateEncryptionKeysPhase != rkev1.RotateEncryptionKeysPhaseRotate {
		return plan.OneTimeInstruction{}, fmt.Errorf("cannot determine desired secrets-encrypt command for phase: [%s]", controlPlane.Status.RotateEncryptionKeysPhase)
	}
	if retryCount < 0 {
		retryCount = 0
	}

	// SaveOutput on one-time instructions only persists stdout. Wrap the runtime command in `sh -c ... 2>&1` so Rancher
	// can inspect timeout text from the K3s/RKE2 CLI when the client exits before the background rotation finishes.
	instruction := idempotentInstruction(
		controlPlane,
		strings.ToLower(fmt.Sprintf("encryption-key-rotation/%s", controlPlane.Status.RotateEncryptionKeysPhase)),
		encryptionKeyRotationRotateKeysInstructionValue(controlPlane, retryCount),
		"/bin/sh",
		[]string{
			"-c",
			fmt.Sprintf("%s secrets-encrypt %s 2>&1", capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion), encryptionKeyRotationCommandRotateKeys),
		},
		[]string{
			encryptionKeyRotationGenerationEnv(controlPlane),
			encryptionKeyRotationRotateKeysRetryCountEnv(retryCount),
		},
	)
	instruction.SaveOutput = true

	return instruction, nil
}

// encryptionKeyRotationRotateKeysTimedOut reports whether the saved one-time instruction output matches the
// expected timeout signature from the K3s or RKE2 secrets-encrypt client.
func encryptionKeyRotationRotateKeysTimedOut(entry *planEntry, instructionName string) bool {
	message, ok := encryptionKeyRotationRotateKeysOutput(entry, instructionName)
	if !ok {
		return false
	}

	return encryptionKeyRotationCommandTimedOut(message, encryptionKeyRotationRotateKeysTimeoutEndpoint)
}

// encryptionKeyRotationRotateKeysFailedWithRetryablePrecondition intentionally matches the coarse CLI error shape that
// K3s/RKE2 exposes to Rancher. The CLI does not include the underlying server-side cause, so the planner only retries
// after separately observing a stable periodic secrets-encrypt status, and the retry count is strictly bounded.
func encryptionKeyRotationRotateKeysFailedWithRetryablePrecondition(entry *planEntry, instructionName string) bool {
	message, ok := encryptionKeyRotationRotateKeysOutput(entry, instructionName)
	if !ok {
		return false
	}

	return strings.Contains(message, encryptionKeyRotationRotateKeysTimeoutMessage) &&
		strings.Contains(message, encryptionKeyRotationRotateKeysErrorIDMessage)
}

func encryptionKeyRotationRotateKeysOutput(entry *planEntry, instructionName string) (string, bool) {
	if entry == nil || entry.Plan == nil || instructionName == "" {
		return "", false
	}

	outputs := entry.Plan.Output
	if entry.Plan.Failed {
		// system-agent writes SaveOutput for failed one-time instructions to the failed-output secret key instead of
		// applied-output, so Rancher has to inspect the corresponding FailedOutput map on failed plans.
		outputs = entry.Plan.FailedOutput
	}

	output, ok := outputs[instructionName]
	if !ok {
		return "", false
	}

	return string(output), true
}

func encryptionKeyRotationCommandTimedOut(message, endpoint string) bool {
	if message == "" || endpoint == "" {
		return false
	}
	if !strings.Contains(message, encryptionKeyRotationRotateKeysTimeoutMessage) || !strings.Contains(message, endpoint) {
		return false
	}
	for _, marker := range encryptionKeyRotationTimeoutMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func encryptionKeyRotationRotateKeysCanRetry(status encryptionKeyRotationRuntimeStatus) bool {
	if !status.HashesMatch {
		return false
	}

	switch status.Stage {
	case encryptionKeyRotationStageStart, encryptionKeyRotationStageReencryptFinished:
		return true
	default:
		return false
	}
}

// encryptionKeyRotationStatusEnv returns an environment variable in order to force followers to rerun their plans
// when the planner advances between the rotate-keys and restart phases. assignAndCheckPlan only reapplies when the
// desired plan changes, so the planner uses phase/generation env vars to deliberately perturb otherwise identical
// restart and status instructions across reconciliations.
func encryptionKeyRotationStatusEnv(controlPlane *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("ENCRYPTION_KEY_ROTATION_STAGE=%s", controlPlane.Status.RotateEncryptionKeysPhase)
}

// encryptionKeyRotationGenerationEnv returns an environment variable in order to force followers to rerun their plans
// on subsequent generations.
func encryptionKeyRotationGenerationEnv(controlPlane *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("%s=%d", encryptionKeyRotationGenerationEnvName, encryptionKeyRotationActiveGeneration(controlPlane))
}

// encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction generates a periodic instruction which will scrape the secrets-encrypt
// status from the node every 5 seconds.
func encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction(controlPlane *rkev1.RKEControlPlane) plan.PeriodicInstruction {
	return plan.PeriodicInstruction{
		Name:    encryptionKeyRotationSecretsEncryptStatusCommand,
		Command: capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			"status",
		},
		Env: []string{
			encryptionKeyRotationStatusEnv(controlPlane),
			encryptionKeyRotationGenerationEnv(controlPlane),
		},
		PeriodSeconds: 5,
	}
}

// encryptionKeyRotationWaitForSystemctlStatusInstruction is intended to run after a node is restart, and wait until the
// node is online and able to provide systemctl status, ensuring that the server service is able to be restarted. If the
// service never comes active, the plan advances anyway in order to restart the service. If restarting the service
// fails, then the plan will fail.
func encryptionKeyRotationWaitForSystemctlStatusInstruction(controlPlane *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "wait-for-systemctl-status",
		Command: "sh",
		Args: []string{
			"-x", encryptionKeyRotationScriptPath(controlPlane, encryptionKeyRotationWaitForSystemctlStatusPath), capr.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion),
		},
		Env: []string{
			encryptionKeyRotationEndpointEnv,
			encryptionKeyRotationStatusEnv(controlPlane),
			encryptionKeyRotationGenerationEnv(controlPlane),
		},
		SaveOutput: false,
	}
}

// encryptionKeyRotationWaitForSecretsEncryptStatus is intended to run after a node is restart, and wait until the node
// is online and able to provide secrets-encrypt status, ensuring that subsequent status commands from the system-agent
// will be successful.
func encryptionKeyRotationWaitForSecretsEncryptStatus(controlPlane *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "wait-for-secrets-encrypt-status",
		Command: "sh",
		Args: []string{
			"-x", encryptionKeyRotationScriptPath(controlPlane, encryptionKeyRotationWaitForSecretsEncryptStatusPath), capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
		},
		Env: []string{
			encryptionKeyRotationEndpointEnv,
			encryptionKeyRotationStatusEnv(controlPlane),
			encryptionKeyRotationGenerationEnv(controlPlane),
		},
		SaveOutput: true,
	}
}

// encryptionKeyRotationHandleFailure makes sure the planner leaves the owning CAPI cluster unpaused after an
// unrecoverable rotation failure. The phase transition to Failed is handled separately so Rancher can persist status
// and surface the original reconciliation error.
func (p *Planner) encryptionKeyRotationHandleFailure(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, err error) (rkev1.RKEControlPlaneStatus, error) {
	if err == nil || IsErrWaiting(err) || status.RotateEncryptionKeysPhase != rkev1.RotateEncryptionKeysPhaseFailed {
		return status, err
	}
	if pauseErr := p.pauseCAPICluster(controlPlane, false); pauseErr != nil {
		return status, errWaiting("unpausing CAPI cluster")
	}
	status.RotateEncryptionKeysLeader = ""
	return status, err
}

// encryptionKeyRotationFailed updates the various status objects on the control plane, allowing the cluster to
// continue the reconciliation loop. Encryption key rotation will not be restarted again until requested.
func (p *Planner) encryptionKeyRotationFailed(status rkev1.RKEControlPlaneStatus, err error) (rkev1.RKEControlPlaneStatus, error) {
	status.RotateEncryptionKeysPhase = rkev1.RotateEncryptionKeysPhaseFailed
	status.RotateEncryptionKeysLeader = ""
	return status, errors.Wrap(err, "encryption key rotation failed")
}

func encryptionKeyRotationScriptPath(controlPlane *rkev1.RKEControlPlane, file string) string {
	return path.Join(capr.GetDistroDataDir(controlPlane), encryptionKeyRotationBinPrefix, file)
}
