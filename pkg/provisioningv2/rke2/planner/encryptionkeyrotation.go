package planner

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/rancher/channelserver/pkg/model"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	encryptionKeyRotationStageReencryptRequest  = "reencrypt_request"
	encryptionKeyRotationStageReencryptActive   = "reencrypt_active"
	encryptionKeyRotationStageReencryptFinished = "reencrypt_finished"

	encryptionKeyRotationCommandPrepare   = "prepare"
	encryptionKeyRotationCommandRotate    = "rotate"
	encryptionKeyRotationCommandReencrypt = "reencrypt"

	encryptionKeyRotationSecretsEncryptApplyCommand  = "secrets-encrypt-apply"
	encryptionKeyRotationSecretsEncryptStatusCommand = "secrets-encrypt-status"

	encryptionKeyRotationInstallRoot = "/var/lib/rancher"
	encryptionKeyRotationBinPrefix   = "rancher_v2prov_encryption_key_rotation/bin"

	encryptionKeyRotationWaitForSystemctlStatusPath      = "wait_for_systemctl_status.sh"
	encryptionKeyRotationWaitForSecretsEncryptStatusPath = "wait_for_secrets_encrypt_status.sh"
	encryptionKeyRotationSecretsEncryptStatusPath        = "secrets_encrypt_status.sh"
	encryptionKeyRotationActionPath                      = "action.sh"

	encryptionKeyRotationWaitForSystemctlStatus = `
#!/bin/sh

runtimeServer=$1
i=0

while [ $i -lt 10 ]; do
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

	encryptionKeyRotationSecretsEncryptStatusScript = `
#!/bin/sh

runtime=$1
i=0

while [ $i -lt 10 ]; do
	output="$($runtime secrets-encrypt status)"
	if [ $? -eq 0 ]; then
		if [ -n "$2" ]; then
			echo $output | grep -q "$2"
				if [ $? -eq 0 ]; then
					exit 0
				fi
		else
			exit 0
		fi
	fi
	sleep 10
	i=$((i + 1))
done
exit 1
`

	idempotentActionScript = `
#!/bin/sh

currentGeneration=""
key=$1
targetGeneration=$2
runtime=$3
shift
shift
shift

dataRoot="/var/lib/rancher/$runtime/$key"
generationFile="$dataRoot/generation"

currentGeneration=$(cat "$generationFile" || echo "")

if [ "$currentGeneration" != "$targetGeneration" ]; then
	$runtime $@
else
	echo "action has already been reconciled to the current generation."
fi

mkdir -p "$dataRoot"
echo "$targetGeneration" > "$generationFile"
`

	encryptionKeyRotationEndpointEnv = "CONTAINER_RUNTIME_ENDPOINT=unix:///var/run/k3s/containerd/containerd.sock"
)

func (p *Planner) setEncryptionKeyRotateState(status rkev1.RKEControlPlaneStatus, rotate *rkev1.RotateEncryptionKeys, phase rkev1.RotateEncryptionKeysPhase) (rkev1.RKEControlPlaneStatus, error) {
	if equality.Semantic.DeepEqual(status.RotateEncryptionKeys, status) && equality.Semantic.DeepEqual(status.RotateEncryptionKeysPhase, phase) {
		return status, nil
	}
	status.RotateEncryptionKeys = rotate
	status.RotateEncryptionKeysPhase = phase
	return status, ErrWaiting("refreshing encryption key rotation state")
}

func (p *Planner) resetEncryptionKeyRotateState(status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if status.RotateEncryptionKeys == nil && status.RotateEncryptionKeysPhase == "" {
		return status, nil
	}
	return p.setEncryptionKeyRotateState(status, nil, "")
}

// rotateEncryptionKeys first verifies that the control plane is in a state where the next step can be derived. If encryption key rotation is required, the corresponding phase and status fields will be set.
// The function is expected to be called multiple times throughout encryption key rotation, and will set the next corresponding phase based on previous output.
func (p *Planner) rotateEncryptionKeys(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, releaseData *model.Release, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	if cp == nil || releaseData == nil || clusterPlan == nil {
		return status, fmt.Errorf("cannot pass nil parameters to rotateEncryptionKeys")
	}

	if cp.Spec.RotateEncryptionKeys == nil {
		return p.resetEncryptionKeyRotateState(status)
	}

	if supported, err := encryptionKeyRotationSupported(releaseData); err != nil {
		return status, err
	} else if !supported {
		logrus.Debugf("rkecluster %s/%s: marking encryption key rotation phase as failed as it was not supported by version: %s", cp.Namespace, cp.Name, cp.Spec.KubernetesVersion)
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseFailed)
	}

	if !canRotateEncryptionKeys(cp) {
		return status, nil
	}

	if shouldRestartEncryptionKeyRotation(cp) {
		logrus.Debugf("[planner] rkecluster %s/%s: starting/restarting encryption key rotation", cp.Namespace, cp.Name)
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhasePrepare)
	}

	if err := p.pauseCAPICluster(cp, true); err != nil {
		return status, err
	}

	leader, err := p.encryptionKeyRotationFindLeader(cp, status, clusterPlan)
	if err != nil {
		return status, err
	}

	if status.RotateEncryptionKeysLeader != leader.Machine.Name {
		status.RotateEncryptionKeysLeader = leader.Machine.Name
		return status, ErrWaitingf("elected %s as control plane leader for encryption key rotation", leader.Machine.Name)
	}

	logrus.Debugf("[planner] rkecluster %s/%s: current encryption key rotation phase: [%s]", cp.Namespace, cp.Spec.ClusterName, cp.Status.RotateEncryptionKeysPhase)

	switch cp.Status.RotateEncryptionKeysPhase {
	case rkev1.RotateEncryptionKeysPhasePrepare:
		status, err = p.encryptionKeyRotationLeaderPhaseReconcile(cp, status, leader)
		if err != nil {
			return status, err
		}
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhasePostPrepareRestart)
	case rkev1.RotateEncryptionKeysPhasePostPrepareRestart:
		status, err = p.encryptionKeyRotationRestartNodes(cp, status, clusterPlan, leader)
		if err != nil {
			return status, err
		}
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseRotate)
	case rkev1.RotateEncryptionKeysPhaseRotate:
		status, err = p.encryptionKeyRotationLeaderPhaseReconcile(cp, status, leader)
		if err != nil {
			return status, err
		}
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhasePostRotateRestart)
	case rkev1.RotateEncryptionKeysPhasePostRotateRestart:
		status, err = p.encryptionKeyRotationRestartNodes(cp, status, clusterPlan, leader)
		if err != nil {
			return status, err
		}
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseReencrypt)
	case rkev1.RotateEncryptionKeysPhaseReencrypt:
		status, err = p.encryptionKeyRotationLeaderPhaseReconcile(cp, status, leader)
		if err != nil {
			return status, err
		}
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhasePostReencryptRestart)
	case rkev1.RotateEncryptionKeysPhasePostReencryptRestart:
		status, err = p.encryptionKeyRotationRestartNodes(cp, status, clusterPlan, leader)
		if err != nil {
			return status, err
		}
		if err = p.pauseCAPICluster(cp, false); err != nil {
			return status, err
		}
		status.RotateEncryptionKeysLeader = ""
		return p.setEncryptionKeyRotateState(status, cp.Spec.RotateEncryptionKeys, rkev1.RotateEncryptionKeysPhaseDone)
	}

	return status, fmt.Errorf("encountered unknown encryption key rotation phase: %s", cp.Status.RotateEncryptionKeysPhase)
}

// encryptionKeyRotationSupported returns a boolean indicating whether encryption key rotation is supported by the release,
// and an error if one was encountered.
func encryptionKeyRotationSupported(releaseData *model.Release) (bool, error) {
	if releaseData == nil {
		return false, fmt.Errorf("unable to find KDM release data for encryption key rotation support in release")
	}
	if featureVersion, ok := releaseData.FeatureVersions["encryption-key-rotation"]; ok {
		version, err := semver.Make(featureVersion)
		if err != nil {
			return false, fmt.Errorf("unable to parse semver version for encryption key rotation: %s", featureVersion)
		}
		// v2.6.4 - v2.6.6 are looking for 1.x.x, but encryption key rotation does not work in those versions. Rather than
		// enable it retroactively, those versions will not be able to rotate encryption keys since some cluster
		// configurations can break in such a way that they become unrecoverable. Additionally, we want to be careful
		// updating the encryption-key-rotation feature gate in KDM in the future, so as not to break backwards
		// compatibility for existing clusters.
		if version.Major == 2 {
			return true, nil
		}
	}
	return false, nil
}

// canRotateEncryptionKeys returns false if the controlplane does not have a Ready: True condition and encryption key rotation is not already in progress, if the spec for
// encryption key rotation is nil, or if the spec has been reconciled but the phase is done or failed.
func canRotateEncryptionKeys(cp *rkev1.RKEControlPlane) bool {
	if (!rke2.Ready.IsTrue(cp) && !rotateEncryptionKeyInProgress(cp)) ||
		cp.Spec.RotateEncryptionKeys == nil ||
		(cp.Status.RotateEncryptionKeys != nil && cp.Status.RotateEncryptionKeys.Generation == cp.Spec.RotateEncryptionKeys.Generation &&
			(cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone || cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseFailed)) {
		return false
	}

	return true
}

// shouldRestartEncryptionKeyRotation returns `true` if encryption key rotation is necessary and the fields on the status object are not valid for an encryption key rotation procedure.
func shouldRestartEncryptionKeyRotation(cp *rkev1.RKEControlPlane) bool {
	if !rke2.Ready.IsTrue(cp) {
		return false
	}
	if cp.Spec.RotateEncryptionKeys.Generation > 0 && cp.Status.RotateEncryptionKeys == nil {
		return true
	}
	if (cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseFailed ||
		cp.Status.RotateEncryptionKeysPhase == "") &&
		cp.Spec.RotateEncryptionKeys.Generation != cp.Status.RotateEncryptionKeys.Generation {
		return true
	}
	if !hasValidNonFailedRotateEncryptionKeyStatus(cp) {
		return true
	}
	return false
}

// hasValidNonFailedRotateEncryptionKeyStatus verifies that the control plane encryption key status is an expected value and is not failed.
func hasValidNonFailedRotateEncryptionKeyStatus(cp *rkev1.RKEControlPlane) bool {
	return rotateEncryptionKeyInProgress(cp) ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone
}

// rotateEncryptionKeyInProgress returns true if the phase of the encryption key rotation indicates that rotation is in progress.
func rotateEncryptionKeyInProgress(cp *rkev1.RKEControlPlane) bool {
	return cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhasePrepare ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhasePostPrepareRestart ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseRotate ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhasePostRotateRestart ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseReencrypt ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhasePostReencryptRestart
}

// encryptionKeyRotationFindLeader returns the current encryption rotation leader if it is valid, otherwise, if the
// phase is "prepare", it will re-elect a new leader. It will look for the init node, and if the init node is not valid
// (etcd-only), it will elect the first suitable control plane node. If the phase is not in "prepare" and a re-election
// of the leader is necessary, the phase will be set to failed as this is unexpected.
func (p *Planner) encryptionKeyRotationFindLeader(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, clusterPlan *plan.Plan) (*planEntry, error) {
	_, _, init, _ := p.findInitNode(cp, clusterPlan)
	if init == nil {
		return nil, fmt.Errorf("cannot find suitable init node")
	}

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

	if status.RotateEncryptionKeysPhase != rkev1.RotateEncryptionKeysPhasePrepare {
		// if we are electing a leader and are not in the "prepare" phase, something is wrong.
		return nil, fmt.Errorf("cannot elect control plane leader in phase %s", status.RotateEncryptionKeysPhase)
	}

	leader := init
	if !isControlPlane(init) {
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
	return isControlPlane(entry) && entry.Machine.DeletionTimestamp == nil && entry.Machine.Status.NodeRef != nil && rke2.Ready.IsTrue(entry.Machine)
}

// encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit allows us to filter cluster plans to restart healthy follower nodes.
func encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit(cp *rkev1.RKEControlPlane) roleFilter {
	return func(entry *planEntry) bool {
		return isControlPlaneAndNotInitNode(entry) &&
			cp.Status.RotateEncryptionKeysLeader != entry.Machine.Name
	}
}

// encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit allows us to filter cluster plans to restart healthy follower nodes.
func encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit(cp *rkev1.RKEControlPlane) roleFilter {
	return func(entry *planEntry) bool {
		return isEtcd(entry) && !isControlPlane(entry) &&
			cp.Status.RotateEncryptionKeysLeader != entry.Machine.Name &&
			!isInitNode(entry)
	}
}

// encryptionKeyRotationRestartNodes restarts the leader's server service, extracting the current stage afterwards.
// The followers (if any exist) are subsequently restarted. Notably, if the encryption key rotation leader is not the init node,
// it will restart the init node, then restart the encryption key rotation leader,
// then finalize walking through etcd nodes (that are not controlplane), then finally controlplane nodes.
func (p *Planner) encryptionKeyRotationRestartNodes(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, clusterPlan *plan.Plan, leader *planEntry) (rkev1.RKEControlPlaneStatus, error) {
	// in certain cases with multi-node setups, we must restart the init node before we can proceed to restarting the leader.
	if !isInitNode(leader) {
		logrus.Debugf("[planner] rkecluster %s/%s: leader %s was not the init node, finding and restarting etcd nodes", cp.Namespace, cp.Name, leader.Machine.Name)
		// find init node and restart service on the init node first.
		initNodeFound, _, initNode, err := p.findInitNode(cp, clusterPlan)
		if err != nil {
			return status, err
		}
		if !initNodeFound || initNode == nil {
			return status, fmt.Errorf("unable to find init node")
		}

		_, status, err = p.encryptionKeyRotationRestartService(cp, status, initNode, false, "")
		if err != nil {
			return status, err
		}
		logrus.Debugf("[planner] rkecluster %s/%s: collecting etcd and not control plane", cp.Namespace, cp.Name)
		for _, entry := range collect(clusterPlan, encryptionKeyRotationIsEtcdAndNotControlPlaneAndNotLeaderAndInit(cp)) {
			_, status, err = p.encryptionKeyRotationRestartService(cp, status, entry, false, "")
			if err != nil {
				return status, err
			}
		}
	}

	leaderStage, status, err := p.encryptionKeyRotationRestartService(cp, status, leader, true, "")
	if err != nil {
		return status, err
	}

	logrus.Debugf("[planner] rkecluster %s/%s: collecting control plane and not leader and init nodes", cp.Namespace, cp.Name)
	for _, entry := range collect(clusterPlan, encryptionKeyRotationIsControlPlaneAndNotLeaderAndInit(cp)) {
		var stage string
		stage, status, err = p.encryptionKeyRotationRestartService(cp, status, entry, true, leaderStage)
		if err != nil {
			return status, err
		}

		if stage != leaderStage {
			// secrets-encrypt command was run on another node. this is considered a failure, but might be a bit too sensitive. to be tested.
			return p.encryptionKeyRotationFailed(status, fmt.Errorf("leader [%s] with %s stage and follower [%s] with %s stage", leader.Machine.Status.NodeRef.Name, leaderStage, entry.Machine.Status.NodeRef.Name, stage))
		}
	}

	return status, nil
}

// encryptionKeyRotationRestartService restarts the server unit on the downstream node, waits until secrets-encrypt
// status can be successfully queried, and then gets the status. leaderStage is allowed to be empty if entry is the
// leader.
func (p *Planner) encryptionKeyRotationRestartService(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, entry *planEntry, scrapeStage bool, leaderStage string) (string, rkev1.RKEControlPlaneStatus, error) {
	//runtime := rke2.GetRuntime(cp.Spec.KubernetesVersion)
	nodePlan := plan.NodePlan{
		Files: []plan.File{
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSystemctlStatus)),
				Path:    encryptionKeyRotationScriptPath(cp, encryptionKeyRotationWaitForSystemctlStatusPath),
			},
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSecretsEncryptStatusScript)),
				Path:    encryptionKeyRotationScriptPath(cp, encryptionKeyRotationWaitForSecretsEncryptStatusPath),
			},
		},
		Instructions: []plan.OneTimeInstruction{
			encryptionKeyRotationWaitForSystemctlStatusInstruction(cp),
			//generateKillAllInstruction(runtime),
			encryptionKeyRotationRestartInstruction(cp),
		},
	}

	if isControlPlane(entry) {
		nodePlan.Files = append(nodePlan.Files,
			plan.File{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationSecretsEncryptStatusScript)),
				Path:    encryptionKeyRotationScriptPath(cp, encryptionKeyRotationSecretsEncryptStatusPath),
			},
		)
		nodePlan.Instructions = append(nodePlan.Instructions,
			encryptionKeyRotationWaitForSecretsEncryptStatus(cp),
			encryptionKeyRotationSecretsEncryptStatusScriptOneTimeInstruction(cp, leaderStage),
			encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction(cp),
		)
	}
	// retry is important here because without it, we always seem to run into some sort of issue such as:
	// - the follower node reporting the wrong status after a restart
	// - the plan failing with the k3s/rke2-server services crashing the first, and resuming subsequent times
	// It's not necessarily ideal if encryption key rotation can never complete, especially since we don't have access to
	// the downstream k3s/rke2-server service logs, but it has to be done in order for encryption key rotation to succeed
	err := assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation [%s] for machine [%s]", cp.Status.RotateEncryptionKeysPhase, entry.Machine.Name), entry, nodePlan, 5, 5)
	if err != nil {
		if IsErrWaiting(err) {
			return "", status, err
		}
		status, err := p.encryptionKeyRotationFailed(status, err)
		return "", status, err
	}

	if !scrapeStage || !isControlPlane(entry) {
		return "", status, nil
	}

	stage, err := encryptionKeyRotationSecretsEncryptStageFromOneTimeStatus(entry)
	if err != nil {
		return "", status, err
	}
	return stage, status, nil
}

// encryptionKeyRotationLeaderPhaseReconcile will run the secrets-encrypt command that corresponds to the phase, and scrape output to ensure that it was
// successful. If the secrets-encrypt command does not exist on the plan, that means this is the first reconciliation, and
// it must be added, otherwise reenqueue until the plan is in sync.
func (p *Planner) encryptionKeyRotationLeaderPhaseReconcile(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, leader *planEntry) (rkev1.RKEControlPlaneStatus, error) {
	apply, err := encryptionKeyRotationSecretsEncryptInstruction(cp)
	if err != nil {
		return p.encryptionKeyRotationFailed(status, err)
	}

	nodePlan := plan.NodePlan{
		Files: []plan.File{
			{
				Content: base64.StdEncoding.EncodeToString([]byte(idempotentActionScript)),
				Path:    encryptionKeyRotationScriptPath(cp, encryptionKeyRotationActionPath),
			},
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSecretsEncryptStatusScript)),
				Path:    encryptionKeyRotationScriptPath(cp, encryptionKeyRotationWaitForSecretsEncryptStatusPath),
			},
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationSecretsEncryptStatusScript)),
				Path:    encryptionKeyRotationScriptPath(cp, encryptionKeyRotationSecretsEncryptStatusPath),
			},
		},
		Instructions: []plan.OneTimeInstruction{
			apply,
			encryptionKeyRotationSecretsEncryptStatusScriptOneTimeInstruction(cp, ""),
			encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction(cp),
		},
		PeriodicInstructions: []plan.PeriodicInstruction{
			encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction(cp),
		},
	}
	err = assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation [%s] for machine [%s]", cp.Status.RotateEncryptionKeysPhase, leader.Machine.Name), leader, nodePlan, 1, 1)
	if err != nil {
		if IsErrWaiting(err) {
			if strings.HasPrefix(err.Error(), "starting") {
				logrus.Infof("[planner] rkecluster %s/%s: applying encryption key rotation stage command: [%s]", cp.Namespace, cp.Spec.ClusterName, apply.Args[1])
			}
			return status, err
		}
		return p.encryptionKeyRotationFailed(status, err)
	}
	scrapedStageFromOneTimeInstructions, err := encryptionKeyRotationSecretsEncryptStageFromOneTimeStatus(leader)
	if err != nil {
		return status, err
	}
	periodic, err := encryptionKeyRotationSecretsEncryptStageFromPeriodic(leader)
	if err != nil {
		return status, err
	}
	err = encryptionKeyRotationIsCurrentStageAllowed(scrapedStageFromOneTimeInstructions, cp.Status.RotateEncryptionKeysPhase)
	if err != nil {
		return p.encryptionKeyRotationFailed(status, err)
	}
	if scrapedStageFromOneTimeInstructions == encryptionKeyRotationStageReencryptRequest || scrapedStageFromOneTimeInstructions == encryptionKeyRotationStageReencryptActive {
		if periodic != encryptionKeyRotationStageReencryptFinished {
			return status, ErrWaitingf("waiting for encryption key rotation stage to be finished")
		}
	}
	// successful restart, complete same phases for rotate & reencrypt
	logrus.Infof("[planner] rkecluster %s/%s: successfully applied encryption key rotation stage command: [%s]", cp.Namespace, cp.Spec.ClusterName, leader.Plan.Plan.Instructions[0].Args[1])
	return status, nil
}

// encryptionKeyRotationSecretsEncryptStageFromPeriodic will attempt to extract the current stage (secrets-encrypt status) from the
// plan by parsing the periodic output.
func encryptionKeyRotationSecretsEncryptStageFromPeriodic(plan *planEntry) (string, error) {
	output, ok := plan.Plan.PeriodicOutput[encryptionKeyRotationSecretsEncryptStatusCommand]
	if !ok {
		for _, pi := range plan.Plan.Plan.PeriodicInstructions {
			if pi.Name == encryptionKeyRotationSecretsEncryptStatusCommand {
				return "", ErrWaitingf("could not extract current status from plan for [%s]: no output for status", plan.Machine.Name)
			}
		}
		return "", fmt.Errorf("could not extract current status from plan for [%s]: status command not present in plan", plan.Machine.Name)
	}
	periodic, err := encryptionKeyRotationStageFromOutput(plan, string(output.Stdout))
	return periodic, err
}

// encryptionKeyRotationSecretsEncryptStageFromOneTimeStatus will attempt to extract the current stage (secrets-encrypt status) from the
// plan by parsing the one time output.
func encryptionKeyRotationSecretsEncryptStageFromOneTimeStatus(plan *planEntry) (string, error) {
	output, ok := plan.Plan.Output[encryptionKeyRotationSecretsEncryptStatusCommand]
	if !ok {
		return "", ErrWaitingf("could not extract current status from plan for [%s]: no output for status", plan.Machine.Name)
	}
	status, err := encryptionKeyRotationStageFromOutput(plan, string(output))
	return status, err
}

// encryptionKeyRotationStageFromOutput parses the output of a secrets-encrypt status command.
func encryptionKeyRotationStageFromOutput(plan *planEntry, output string) (string, error) {
	a := strings.Split(output, "\n")
	if len(a) < 2 {
		return "", ErrWaitingf("could not extract current stage from plan for [%s]: status output is incomplete", plan.Machine.Name)
	}
	for _, v := range a {
		a = strings.Split(v, ": ")
		if a[0] != "Current Rotation Stage" {
			continue
		}
		status := a[1]
		return status, nil
	}
	return "", ErrWaitingf("unable to parse rotation stage from output")
}

// encryptionKeyRotationSecretsEncryptInstruction generates a secrets-encrypt command to run on the leader node given
// the current secrets-encrypt phase.
func encryptionKeyRotationSecretsEncryptInstruction(cp *rkev1.RKEControlPlane) (plan.OneTimeInstruction, error) {
	if cp == nil {
		return plan.OneTimeInstruction{}, fmt.Errorf("controlplane cannot be nil")
	}

	var command string
	switch cp.Status.RotateEncryptionKeysPhase {
	case rkev1.RotateEncryptionKeysPhasePrepare:
		command = encryptionKeyRotationCommandPrepare
	case rkev1.RotateEncryptionKeysPhaseRotate:
		command = encryptionKeyRotationCommandRotate
	case rkev1.RotateEncryptionKeysPhaseReencrypt:
		command = encryptionKeyRotationCommandReencrypt
	default:
		return plan.OneTimeInstruction{}, fmt.Errorf("cannot determine desired secrets-encrypt command for phase: [%s]", cp.Status.RotateEncryptionKeysPhase)
	}

	return plan.OneTimeInstruction{
		Name:    encryptionKeyRotationSecretsEncryptApplyCommand,
		Command: "sh",
		Args: []string{
			"-xe",
			encryptionKeyRotationScriptPath(cp, encryptionKeyRotationActionPath),
			strings.ToLower(fmt.Sprintf("rancher_v2prov_encryption_key_rotation/%s", cp.Status.RotateEncryptionKeysPhase)),
			strconv.FormatInt(cp.Spec.RotateEncryptionKeys.Generation, 10),
			rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
			"secrets-encrypt",
			command,
		},
	}, nil
}

// encryptionKeyRotationStatusEnv returns an environment variable in order to force followers to rerun their plans
// following progression of encryption key rotation. In an HA setup with split role etcd & controlplane nodes, the etcd
// nodes would have identical plans, so this variable is used to spoof an update and force the system-agent to run the
// plan.
func encryptionKeyRotationStatusEnv(cp *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("ENCRYPTION_KEY_ROTATION_STAGE=%s", cp.Status.RotateEncryptionKeysPhase)
}

// encryptionKeyRotationGenerationEnv returns an environment variable in order to force followers to rerun their plans
// on subsequent generations, in the event that encryption key rotation is restarting and failed during prepare.
func encryptionKeyRotationGenerationEnv(cp *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("ENCRYPTION_KEY_ROTATION_GENERATION=%d", cp.Spec.RotateEncryptionKeys.Generation)
}

// encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction generates a one time instruction which will scrape the secrets-encrypt
// status.
func encryptionKeyRotationSecretsEncryptStatusScriptOneTimeInstruction(cp *rkev1.RKEControlPlane, expected string) plan.OneTimeInstruction {
	i := plan.OneTimeInstruction{
		Name:    "secrets-encrypt-status-script",
		Command: "sh",
		Args: []string{
			"-x",
			encryptionKeyRotationScriptPath(cp, encryptionKeyRotationSecretsEncryptStatusPath),
			rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			encryptionKeyRotationStatusEnv(cp),
			encryptionKeyRotationGenerationEnv(cp),
		},
	}
	if expected != "" {
		i.Args = append(i.Args, expected)
	}
	return i
}

// encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction generates a one time instruction which will scrape the secrets-encrypt
// status.
func encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction(cp *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    encryptionKeyRotationSecretsEncryptStatusCommand,
		Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			"status",
		},
		Env: []string{
			encryptionKeyRotationStatusEnv(cp),
			encryptionKeyRotationGenerationEnv(cp),
		},
		SaveOutput: true,
	}
}

// encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction generates a periodic instruction which will scrape the secrets-encrypt
// status from the node every 5 seconds.
func encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction(cp *rkev1.RKEControlPlane) plan.PeriodicInstruction {
	return plan.PeriodicInstruction{
		Name:    encryptionKeyRotationSecretsEncryptStatusCommand,
		Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			"status",
		},
		PeriodSeconds: 5,
	}
}

// encryptionKeyRotationWaitForSystemctlStatusInstruction is intended to run after a node is restart, and wait until the
// node is online and able to provide systemctl status, ensuring that the server service is able to be restarted. If the
// service never comes active, the plan advances anyway in order to restart the service. If restarting the service
// fails, then the plan will fail.
func encryptionKeyRotationWaitForSystemctlStatusInstruction(cp *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "wait-for-systemctl-status",
		Command: "sh",
		Args: []string{
			"-x", encryptionKeyRotationScriptPath(cp, encryptionKeyRotationWaitForSystemctlStatusPath), rke2.GetRuntimeServerUnit(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			encryptionKeyRotationEndpointEnv,
			encryptionKeyRotationStatusEnv(cp),
			encryptionKeyRotationGenerationEnv(cp),
		},
		SaveOutput: false,
	}
}

// encryptionKeyRotationRestartInstruction generates a restart command for the rke2/k3s server, using the last known
// leader stage in order to ensure that non-init nodes have a refreshed plan if the leader stage changes. If
// secrets-encrypt commands were run on a node that is not the init node, this ensures that after this situation is
// identified and the leader is restarted, other control plane nodes will be restarted given that the leader stage will
// have changed without a corresponding apply command.
func encryptionKeyRotationRestartInstruction(cp *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "restart-service",
		Command: "systemctl",
		Args: []string{
			"restart", rke2.GetRuntimeServerUnit(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			encryptionKeyRotationStatusEnv(cp),
			encryptionKeyRotationGenerationEnv(cp),
		},
	}
}

// encryptionKeyRotationWaitForSecretsEncryptStatus is intended to run after a node is restart, and wait until the node
// is online and able to provide secrets-encrypt status, ensuring that subsequent status commands from the system-agent
// will be successful.
func encryptionKeyRotationWaitForSecretsEncryptStatus(cp *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "wait-for-secrets-encrypt-status",
		Command: "sh",
		Args: []string{
			"-x", encryptionKeyRotationScriptPath(cp, encryptionKeyRotationWaitForSecretsEncryptStatusPath), rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			encryptionKeyRotationEndpointEnv,
			encryptionKeyRotationStatusEnv(cp),
			encryptionKeyRotationGenerationEnv(cp),
		},
		SaveOutput: true,
	}
}

// encryptionKeyRotationIsCurrentStageAllowed returns a boolean that indicates whether the scraped stage from the leader is allowed in comparison with the current phase.
// Since reencrypt can be any of three statuses (reencrypt_request, reencrypt_active, and reencrypt_finished), any of these stages are valid, however the first two (request & active) do
// not explicitly indicate that the current command was successful, just that it hasn't failed yet. In certain cases, a
// cluster may never move beyond request and active.
func encryptionKeyRotationIsCurrentStageAllowed(leaderStage string, currentPhase rkev1.RotateEncryptionKeysPhase) error {
	switch currentPhase {
	case rkev1.RotateEncryptionKeysPhasePrepare:
		if leaderStage == encryptionKeyRotationCommandPrepare {
			return nil
		}
	case rkev1.RotateEncryptionKeysPhaseRotate:
		if leaderStage == encryptionKeyRotationCommandRotate {
			return nil
		}
	case rkev1.RotateEncryptionKeysPhaseReencrypt:
		if leaderStage == encryptionKeyRotationStageReencryptRequest ||
			leaderStage == encryptionKeyRotationStageReencryptActive ||
			leaderStage == encryptionKeyRotationStageReencryptFinished {
			return nil
		}
	}
	return fmt.Errorf("unexpected encryption key rotation stage [%s] for phase [%s]", leaderStage, currentPhase)
}

// encryptionKeyRotationFailed updates the various status objects on the control plane, allowing the cluster to
// continue the reconciliation loop. Encryption key rotation will not be restarted again until requested.
func (p *Planner) encryptionKeyRotationFailed(status rkev1.RKEControlPlaneStatus, err error) (rkev1.RKEControlPlaneStatus, error) {
	status.RotateEncryptionKeysPhase = rkev1.RotateEncryptionKeysPhaseFailed
	return status, errors.Wrap(err, "encryption key rotation failed, please perform an etcd restore")
}

func encryptionKeyRotationScriptPath(cp *rkev1.RKEControlPlane, file string) string {
	return fmt.Sprintf("%s/%s/%s/%s", encryptionKeyRotationInstallRoot, rke2.GetRuntime(cp.Spec.KubernetesVersion), encryptionKeyRotationBinPrefix, file)
}
