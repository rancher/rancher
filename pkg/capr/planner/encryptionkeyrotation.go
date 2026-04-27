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
	encryptionKeyRotationStageStart             = "start"
	encryptionKeyRotationStageReencryptFinished = "reencrypt_finished"
	encryptionKeyRotationHashesMatch            = "All hashes match"

	encryptionKeyRotationCommandRotateKeys = "rotate-keys"

	encryptionKeyRotationSecretsEncryptStatusCommand = "secrets-encrypt-status"
	encryptionKeyRotationRotateKeysTimeoutMessage    = "see server log for details"
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

func (restartTargets encryptionKeyRotationRestartTargets) onlyLeaderNeedsRestart() bool {
	return len(restartTargets.etcdOnly) == 0 && len(restartTargets.controlPlane) == 1
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

// rotateEncryptionKeys drives one requested generation through the simplified upstream flow:
// elect one control-plane leader, run rotate-keys there, then restart the remaining server nodes
// and wait for secrets-encrypt status to converge before marking the generation done.
func (p *Planner) rotateEncryptionKeys(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	if controlPlane == nil || clusterPlan == nil {
		return status, fmt.Errorf("cannot pass nil parameters to rotateEncryptionKeys")
	}

	// If the spec was cleared, undo any rotation-owned pause/status before the main
	// planner resumes normal reconciliation.
	if controlPlane.Spec.RotateEncryptionKeys == nil {
		if status.RotateEncryptionKeys != nil || status.RotateEncryptionKeysPhase != "" || status.RotateEncryptionKeysLeader != "" {
			if err := p.pauseCAPICluster(controlPlane, false); err != nil {
				return status, errWaiting("unpausing CAPI cluster")
			}
		}
		return p.resetEncryptionKeyRotateState(status)
	}

	// Gate the flow before touching plans. Unsupported versions are marked failed so
	// the requested generation does not keep requeueing forever.
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

	// rotate-keys needs an initialized server and a valid join URL because the same
	// plan generation may later restart other server nodes.
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

	// Starting a generation is a status transition only. The next reconcile elects
	// the leader and writes the rotate-keys plan using the stored generation.
	if encryptionKeyRotationShouldStart(controlPlane) {
		logrus.Debugf("[planner] rkecluster %s/%s: starting/restarting encryption key rotation", controlPlane.Namespace, controlPlane.Name)
		return p.setEncryptionKeyRotateState(status, controlPlane.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseRotate)
	}

	// Keep the same leader once chosen so requeues keep observing the same
	// rotate-keys command output instead of moving the operation between servers.
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
		// Pause CAPI while rotate-keys is active so unrelated rollout activity does
		// not race with the encryption-key rotation plan.
		if err := p.pauseCAPICluster(controlPlane, true); err != nil {
			return status, errWaiting("pausing CAPI cluster")
		}

		// rotate-keys is the upstream combined flow. It may finish the reencrypt work
		// after the CLI times out, so progress is decided from periodic status output.
		rotationStatus, status, err := p.encryptionKeyRotationRotateKeysReconcile(controlPlane, status, tokensSecret, joinServer, leader)
		if err != nil {
			return p.encryptionKeyRotationHandleFailure(controlPlane, status, err)
		}
		if rotationStatus.Stage != encryptionKeyRotationStageReencryptFinished {
			return status, errWaitingf("waiting for encryption key rotation stage to finish on leader [%s]", leader.Machine.Name)
		}

		restartTargets := encryptionKeyRotationRestartTargetsForCluster(controlPlane, clusterPlan, leader, initNode)
		if restartTargets.onlyLeaderNeedsRestart() {
			// Single-server rotations can finish as soon as the leader reports final
			// stage and matching hashes because there are no follower servers to restart.
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
		// Multi-server rotations finish by restarting all remaining server nodes in
		// topology order and checking convergence on control-plane nodes.
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

// encryptionKeyRotationSupported returns true when the downstream kubernetes version
// is new enough to use the upstream rotate-keys flow.
func encryptionKeyRotationSupported(controlPlane *rkev1.RKEControlPlane) (bool, error) {
	if controlPlane == nil {
		return false, fmt.Errorf("unable to determine encryption key rotation support for nil control plane")
	}

	kubernetesVersion, err := semver.Make(strings.TrimPrefix(controlPlane.Spec.KubernetesVersion, "v"))
	if err != nil {
		return false, fmt.Errorf("unable to parse kubernetes version for encryption key rotation: %s", controlPlane.Spec.KubernetesVersion)
	}
	if kubernetesVersion.LT(encryptionKeyRotationMinimumVersion) {
		return false, nil
	}

	return true, nil
}

// encryptionKeyRotationShouldSkip returns true when there is nothing to reconcile for
// encryption key rotation. Once a generation is already in progress, the planner keeps
// reconciling even if the control plane is temporarily not Ready so it can finish or
// unwind the in-flight rotation instead of abandoning it mid-flow.
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

// encryptionKeyRotationShouldStart returns true when the planner should start or
// restart the requested generation. Unknown phases are treated as stale state so
// upgrades can recover older planner state by beginning the current generation again.
func encryptionKeyRotationShouldStart(controlPlane *rkev1.RKEControlPlane) bool {
	if controlPlane == nil || controlPlane.Spec.RotateEncryptionKeys == nil {
		return false
	}
	if controlPlane.Status.RotateEncryptionKeys == nil || controlPlane.Status.RotateEncryptionKeysPhase == "" {
		return true
	}
	if !encryptionKeyRotationPhaseIsKnown(controlPlane.Status.RotateEncryptionKeysPhase) {
		return true
	}
	if controlPlane.Spec.RotateEncryptionKeys.Generation != controlPlane.Status.RotateEncryptionKeys.Generation {
		return controlPlane.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone ||
			controlPlane.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseFailed
	}

	return false
}

// encryptionKeyRotationPhaseIsKnown reports whether the phase belongs to the
// RKEControlPlane rotate-keys state machine.
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

// encryptionKeyRotationFindLeader reuses the stored leader when it is still valid.
// During the rotate phase, it elects a new suitable control-plane leader when needed.
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

// encryptionKeyRotationIsSuitableControlPlane returns true for control-plane entries
// that are present, not deleting, Ready, and already joined to the cluster.
func encryptionKeyRotationIsSuitableControlPlane(entry *planEntry) bool {
	return isControlPlane(entry) && isNotDeleting(entry) && entry.Machine.Status.NodeRef.IsDefined() && capr.Ready.IsTrue(entry.Machine)
}

// encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit selects follower control-plane
// machines that should be restarted after the leader rotation finishes.
func encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit(controlPlane *rkev1.RKEControlPlane) roleFilter {
	return func(entry *planEntry) bool {
		return isControlPlaneAndNotInitNode(entry) &&
			controlPlane.Status.RotateEncryptionKeysLeader != entry.Machine.Name
	}
}

// encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit selects etcd-only
// followers that should restart before control-plane nodes during post-rotate restart.
func encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit(controlPlane *rkev1.RKEControlPlane) roleFilter {
	return func(entry *planEntry) bool {
		return isEtcd(entry) && !isControlPlane(entry) &&
			controlPlane.Status.RotateEncryptionKeysLeader != entry.Machine.Name &&
			!isInitNode(entry)
	}
}

// encryptionKeyRotationRestartTargetsForCluster groups nodes for the restart phase.
// Etcd-only nodes are restarted first but are not used for local status/hash checks.
// Control-plane nodes are restarted after that and are used for convergence checks.
func encryptionKeyRotationRestartTargetsForCluster(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan, leader, initNode *planEntry) encryptionKeyRotationRestartTargets {
	targets := encryptionKeyRotationRestartTargets{
		controlPlane: []*planEntry{leader},
	}

	if initNode != nil && !isInitNode(leader) {
		if isControlPlane(initNode) {
			targets.controlPlane = append(targets.controlPlane, initNode)
		} else if isEtcd(initNode) {
			targets.etcdOnly = append(targets.etcdOnly, initNode)
		}
	}

	targets.etcdOnly = append(targets.etcdOnly, collect(clusterPlan, encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit(controlPlane))...)
	targets.controlPlane = append(targets.controlPlane, collect(clusterPlan, encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit(controlPlane))...)

	return targets
}

// encryptionKeyRotationRestartNodes restarts etcd-only nodes first and then control-plane nodes.
// Only control-plane nodes are used for local secrets-encrypt status checks during convergence,
// and the last control-plane node must also report matching hashes before the phase can finish.
func (p *Planner) encryptionKeyRotationRestartNodes(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan, leader *planEntry, initNode *planEntry, joinServer string) (rkev1.RKEControlPlaneStatus, error) {
	restartTargets := encryptionKeyRotationRestartTargetsForCluster(controlPlane, clusterPlan, leader, initNode)

	for _, entry := range restartTargets.etcdOnly {
		_, updatedStatus, err := p.encryptionKeyRotationRestartService(controlPlane, status, tokensSecret, joinServer, entry)
		if err != nil {
			return updatedStatus, err
		}
		status = updatedStatus
	}

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

// encryptionKeyRotationRestartService applies the restart plan for one node and, for
// control-plane nodes, returns the local secrets-encrypt status observed after restart.
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

// encryptionKeyRotationRestartPlan builds the post-rotate restart plan for a single node.
// Control-plane nodes also get the local secrets-encrypt status checks used for convergence.
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

// encryptionKeyRotationRotateKeysReconcile runs rotate-keys on the elected leader and then trusts
// periodic secrets-encrypt status as the source of progress. A CLI timeout is treated as ambiguous,
// so the planner keeps watching periodic status before deciding whether to wait or fail.
func (p *Planner) encryptionKeyRotationRotateKeysReconcile(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, joinServer string, leader *planEntry) (encryptionKeyRotationRuntimeStatus, rkev1.RKEControlPlaneStatus, error) {
	nodePlan, joinedServer, err := p.encryptionKeyRotationRotateKeysPlan(controlPlane, tokensSecret, joinServer, leader)
	if err != nil {
		return encryptionKeyRotationRuntimeStatus{}, status, err
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
			rotationStatus, err := encryptionKeyRotationSecretsEncryptStatusFromPeriodic(leader)
			if err == nil {
				return rotationStatus, status, nil
			}
			return encryptionKeyRotationRuntimeStatus{}, status, errWaitingf("waiting for encryption key rotation status after rotate-keys timeout on leader [%s]: %v", leader.Machine.Name, err)
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

// encryptionKeyRotationRotateKeysPlan builds the leader plan that starts the
// rotate-keys operation and periodically observes secrets-encrypt status.
func (p *Planner) encryptionKeyRotationRotateKeysPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, joinServer string, leader *planEntry) (plan.NodePlan, string, error) {
	nodePlan, _, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, leader, joinServer, true)
	if err != nil {
		return plan.NodePlan{}, "", err
	}

	apply, err := encryptionKeyRotationSecretsEncryptInstruction(controlPlane)
	if err != nil {
		return plan.NodePlan{}, "", err
	}

	nodePlan.Instructions = []plan.OneTimeInstruction{apply}
	nodePlan.PeriodicInstructions = []plan.PeriodicInstruction{
		encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction(controlPlane),
	}

	return nodePlan, joinedServer, nil
}

// encryptionKeyRotationSecretsEncryptStatusFromPeriodic reads the latest periodic status output from
// system-agent and converts it into the small runtime state Rancher reconciles on.
func encryptionKeyRotationSecretsEncryptStatusFromPeriodic(entry *planEntry) (encryptionKeyRotationRuntimeStatus, error) {
	output, ok := entry.Plan.PeriodicOutput[encryptionKeyRotationSecretsEncryptStatusCommand]
	if !ok {
		for _, periodicInstruction := range entry.Plan.Plan.PeriodicInstructions {
			if periodicInstruction.Name == encryptionKeyRotationSecretsEncryptStatusCommand {
				return encryptionKeyRotationRuntimeStatus{}, errWaitingf("could not extract current status from plan for [%s]: no output for status", entry.Machine.Name)
			}
		}
		return encryptionKeyRotationRuntimeStatus{}, fmt.Errorf("could not extract current status from plan for [%s]: status command not present in plan", entry.Machine.Name)
	}

	if stdout := strings.TrimSpace(string(output.Stdout)); stdout != "" {
		return encryptionKeyRotationStatusFromOutput(stdout)
	}
	return encryptionKeyRotationRuntimeStatus{}, errWaitingf("unable to parse rotation stage from output for [%s]", entry.Machine.Name)
}

// encryptionKeyRotationStatusFromOutput parses the human-readable secrets-encrypt status
// output into the small state used by planner reconciliation.
func encryptionKeyRotationStatusFromOutput(output string) (encryptionKeyRotationRuntimeStatus, error) {
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

// encryptionKeyRotationActiveGeneration returns the generation Rancher should pin into
// node plans. In-progress phases keep using the status generation until they finish.
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

// encryptionKeyRotationRotateKeysInstructionValue is the idempotency value for
// the rotate-keys instruction and changes when the requested generation changes.
func encryptionKeyRotationRotateKeysInstructionValue(controlPlane *rkev1.RKEControlPlane) string {
	return strconv.FormatInt(encryptionKeyRotationActiveGeneration(controlPlane), 10)
}

// encryptionKeyRotationSecretsEncryptInstruction builds the one-time rotate-keys
// instruction for the active generation.
func encryptionKeyRotationSecretsEncryptInstruction(controlPlane *rkev1.RKEControlPlane) (plan.OneTimeInstruction, error) {
	if controlPlane == nil {
		return plan.OneTimeInstruction{}, fmt.Errorf("control plane cannot be nil")
	}
	if controlPlane.Status.RotateEncryptionKeysPhase != rkev1.RotateEncryptionKeysPhaseRotate {
		return plan.OneTimeInstruction{}, fmt.Errorf("cannot determine desired secrets-encrypt command for phase: [%s]", controlPlane.Status.RotateEncryptionKeysPhase)
	}

	instruction := idempotentInstruction(
		controlPlane,
		strings.ToLower(fmt.Sprintf("encryption-key-rotation/%s", controlPlane.Status.RotateEncryptionKeysPhase)),
		encryptionKeyRotationRotateKeysInstructionValue(controlPlane),
		"/bin/sh",
		[]string{
			"-c",
			fmt.Sprintf("%s secrets-encrypt %s 2>&1", capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion), encryptionKeyRotationCommandRotateKeys),
		},
		[]string{
			encryptionKeyRotationGenerationEnv(controlPlane),
		},
	)
	instruction.SaveOutput = true

	return instruction, nil
}

// encryptionKeyRotationRotateKeysTimedOut reports whether the saved rotate-keys output
// matches a known CLI timeout on the upstream /encrypt/config endpoint.
func encryptionKeyRotationRotateKeysTimedOut(entry *planEntry, instructionName string) bool {
	message, ok := encryptionKeyRotationRotateKeysOutput(entry, instructionName)
	if !ok {
		return false
	}

	return encryptionKeyRotationCommandTimedOut(message, encryptionKeyRotationRotateKeysTimeoutEndpoint)
}

// encryptionKeyRotationRotateKeysOutput returns the saved one-time output for the given
// rotate-keys instruction, using failed output when the node plan is marked failed.
func encryptionKeyRotationRotateKeysOutput(entry *planEntry, instructionName string) (string, bool) {
	if entry == nil || entry.Plan == nil || instructionName == "" {
		return "", false
	}

	outputs := entry.Plan.Output
	if entry.Plan.Failed {
		outputs = entry.Plan.FailedOutput
	}

	output, ok := outputs[instructionName]
	if !ok {
		return "", false
	}

	return string(output), true
}

// encryptionKeyRotationCommandTimedOut matches the timeout signatures Rancher has seen
// from secrets-encrypt CLI calls, for example:
// `time="..." level=fatal msg="Error: see server log for details: Put \"https://127.0.0.1:9345/v1-rke2/encrypt/config\": net/http: timeout awaiting response headers (Client.Timeout exceeded while awaiting headers)"`
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

// encryptionKeyRotationStatusEnv forces restart and status instructions to rerun as the planner
// moves through encryption key rotation phases.
func encryptionKeyRotationStatusEnv(controlPlane *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("ENCRYPTION_KEY_ROTATION_STAGE=%s", controlPlane.Status.RotateEncryptionKeysPhase)
}

func encryptionKeyRotationGenerationEnv(controlPlane *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("%s=%d", encryptionKeyRotationGenerationEnvName, encryptionKeyRotationActiveGeneration(controlPlane))
}

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

func (p *Planner) encryptionKeyRotationFailed(status rkev1.RKEControlPlaneStatus, err error) (rkev1.RKEControlPlaneStatus, error) {
	status.RotateEncryptionKeysPhase = rkev1.RotateEncryptionKeysPhaseFailed
	status.RotateEncryptionKeysLeader = ""
	return status, errors.Wrap(err, "encryption key rotation failed")
}

func encryptionKeyRotationScriptPath(controlPlane *rkev1.RKEControlPlane, file string) string {
	return path.Join(capr.GetDistroDataDir(controlPlane), encryptionKeyRotationBinPrefix, file)
}
