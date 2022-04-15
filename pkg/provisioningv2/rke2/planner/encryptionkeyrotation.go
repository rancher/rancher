package planner

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/sirupsen/logrus"
)

const (
	encryptionKeyRotationStageStart             = "start"
	encryptionKeyRotationStagePrepare           = "prepare"
	encryptionKeyRotationStageRotate            = "rotate"
	encryptionKeyRotationStageReencryptRequest  = "reencrypt_request"
	encryptionKeyRotationStageReencryptActive   = "reencrypt_active"
	encryptionKeyRotationStageReencryptFinished = "reencrypt_finished"
	encryptionKeyRotationCommandPrepare         = "prepare"
	encryptionKeyRotationCommandRotate          = "rotate"
	encryptionKeyRotationCommandReencrypt       = "reencrypt"

	encryptionKeyRotationSecretsEncryptApplyCommand  = "secrets-encrypt-apply"
	encryptionKeyRotationSecretsEncryptStatusCommand = "secrets-encrypt-status"
	encryptionKeyRotationRestartApiserverCommand     = "restart-apiserver"
	encryptionKeyRotationApiserverStatusCommand      = "apiserver-status"

	encryptionKeyRotationLeaderAnnotation      = "rke.cattle.io/encrypt-key-rotation-leader"
	encryptionKeyRotationApiserverIDAnnotation = "rke.cattle.io/encrypt-key-apiserver-id"

	encryptionKeyRotationRestartAPIServerScriptPath            = "/var/lib/rancher/rke2/bin/restart_apiserver.sh"
	encryptionKeyRotationGetAPIServerStatusScriptPath          = "/var/lib/rancher/rke2/bin/apiserver_status.sh"
	encryptionKeyRotationWaitForSystemctlStatusPath            = "/var/lib/rancher/rke2/bin/wait_for_systemctl_status.sh"
	encryptionKeyRotationWaitForSecretsEncryptStatusScriptPath = "/var/lib/rancher/rke2/bin/wait_for_secrets_encrypt_status.sh"

	// This script finds the apiserver container and restarts it.
	// Update to use --output json from https://github.com/k3s-io/k3s/issues/5138 when implemented in rke2
	encryptionKeyRotationRestartAPIServerScript = `
#!/bin/sh

apiServer=$(/var/lib/rancher/rke2/bin/crictl ps --name kube-apiserver | sed "1 d" | awk '{print $1}')

/var/lib/rancher/rke2/bin/crictl stop $apiServer
`

	// This script finds the apiserver pod and writes the container id to stdout.
	encryptionKeyRotationGetAPIServerStatusScript = `
#!/bin/sh

/var/lib/rancher/rke2/bin/crictl ps --name kube-apiserver | sed "1 d" | awk '{print $1}'
`

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

	encryptionKeyRotationEndpointEnv = "CONTAINER_RUNTIME_ENDPOINT=unix:///var/run/k3s/containerd/containerd.sock"
)

// rotateEncryptionKeys first checks if encryption keys are in need of rotation, then verifies that the control plane is
// in a state where the next step can be derived. If this is not the case, the entire encryption key rotation sequence
// is restarted. Otherwise, the next phase is moved to, first getting the secrets encrypt status from the init node,
// restarting said node, getting the status again, restarting the control plane nodes and verifying that the phases
// match the init node, and either finishing if the status is reencrypt_finished, or applying the next secrets-encrypt
// command.
func (p *Planner) rotateEncryptionKeys(cp *rkev1.RKEControlPlane, clusterPlan *plan.Plan) error {
	if !shouldRotateEncryptionKeys(cp) {
		return nil
	}

	if shouldRestartEncryptionKeyRotation(cp) {
		return p.restartEncryptionKeyRotation(cp)
	}

	leader, init, err := p.encryptionKeyRotationElectLeader(cp, clusterPlan)
	if err != nil {
		return err
	}

	logrus.Debugf("rkecluster %s/%s: current encryption key rotation phase: [%s]", cp.Namespace, cp.Spec.ClusterName, cp.Status.RotateEncryptionKeysPhase)

	switch cp.Status.RotateEncryptionKeysPhase {
	case rkev1.RotateEncryptionKeysPhaseRestartNodes:
		return p.encryptionKeyRotationRestartNodes(cp, clusterPlan, leader, init)
	case rkev1.RotateEncryptionKeysPhaseApplyLeader:
		return p.encryptionKeyRotationApplyLeader(cp, clusterPlan, leader)
	}

	return p.enqueueAndSkip(cp)
}

// shouldRotateEncryptionKeys `true` if encryption key rotation has been requested or has not finished.
func shouldRotateEncryptionKeys(cp *rkev1.RKEControlPlane) bool {
	// Do not stop rotating for any reason, except when encryption key rotation cannot be completed.
	if rke2.Ready.IsUnknown(cp) && rke2.Ready.GetReason(cp) == rke2.RotatingEncryptionKeysReason {
		return true
	}

	// If we haven't started rotating yet, the cluster must be ready before we start.
	if (!rke2.Ready.IsTrue(cp) &&
		rke2.Ready.GetReason(cp) != rke2.RotatingEncryptionKeysReason) ||
		cp.Spec.RotateEncryptionKeys == nil ||
		(cp.Status.RotateEncryptionKeys != nil && cp.Status.RotateEncryptionKeys.Generation == cp.Spec.RotateEncryptionKeys.Generation) {
		return false
	}

	return true
}

// shouldRestartEncryptionKeyRotation `true` if the current phase is unknown, or finished
func shouldRestartEncryptionKeyRotation(cp *rkev1.RKEControlPlane) bool {
	if cp.Spec.RotateEncryptionKeys.Generation > 0 && cp.Status.RotateEncryptionKeys == nil {
		return true
	}
	if cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone &&
		cp.Spec.RotateEncryptionKeys.Generation != cp.Status.RotateEncryptionKeys.Generation {
		return true
	}
	if !hasValidRotateEncryptionKeyStatus(cp) {
		return true
	}
	return false
}

// hasValidRotateEncryptionKeyStatus verifies that the control plane encryption key status is an expected value
func hasValidRotateEncryptionKeyStatus(cp *rkev1.RKEControlPlane) bool {
	return cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseRestartNodes ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseApplyLeader ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone
}

// restartEncryptionKeyRotation restarts encryption key rotation if it is required, by resetting all related control
// plane statuses.
func (p *Planner) restartEncryptionKeyRotation(cp *rkev1.RKEControlPlane) error {
	logrus.Infof("rkecluster %s/%s: restarting encryption key rotation, current phase: [%s]", cp.Namespace, cp.Spec.ClusterName, cp.Status.RotateEncryptionKeysPhase)

	cp = cp.DeepCopy()
	rke2.Ready.Unknown(cp)
	rke2.Ready.Message(cp, "encryption key rotation: restarting nodes")
	rke2.Ready.Reason(cp, rke2.RotatingEncryptionKeysReason)
	if cp.Status.RotateEncryptionKeys == nil {
		cp.Status.RotateEncryptionKeys = &rkev1.RotateEncryptionKeysStatus{}
	}
	cp.Status.RotateEncryptionKeys.LastRestart = time.Now().Format(time.RFC3339)
	cp.Status.RotateEncryptionKeysPhase = rkev1.RotateEncryptionKeysPhaseRestartNodes
	_, err := p.rkeControlPlanes.UpdateStatus(cp)
	if err != nil {
		return err
	}
	return generic.ErrSkip
}

// encryptionKeyRotationElectLeader returns the current leader if it is valid, otherwise it selects the first suitable
// control plane it can find.
func (p *Planner) encryptionKeyRotationElectLeader(cp *rkev1.RKEControlPlane, clusterPlan *plan.Plan) (*planEntry, *planEntry, error) {
	_, _, init, _ := p.findInitNode(cp, clusterPlan)
	if init == nil {
		return nil, nil, p.encryptionKeyRotationFailed(cp)
	}

	if machineName, ok := cp.Annotations[encryptionKeyRotationLeaderAnnotation]; ok {
		if machine, ok := clusterPlan.Machines[machineName]; ok {
			entry := &planEntry{
				Machine:  machine,
				Plan:     clusterPlan.Nodes[machineName],
				Metadata: clusterPlan.Metadata[machineName],
			}
			if encryptionKeyRotationIsSuitableControlPlane(entry) {
				return entry, init, nil
			}
		}
	}

	leader := init
	if !isControlPlane(init) {
		machines := collect(clusterPlan, encryptionKeyRotationIsSuitableControlPlane)
		if len(machines) == 0 {
			return nil, nil, p.encryptionKeyRotationFailed(cp)
		}
		if cp.Status.RotateEncryptionKeysPhase != rkev1.RotateEncryptionKeysPhaseRestartNodes {
			return nil, nil, p.restartEncryptionKeyRotation(cp)
		}
		leader = machines[0]
	}

	cp.Annotations[encryptionKeyRotationLeaderAnnotation] = leader.Machine.Name
	_, err := p.rkeControlPlanes.Update(cp)
	if err != nil {
		return nil, nil, err
	}
	// a new leader was chosen, must restart
	return nil, nil, generic.ErrSkip
}

// encryptionKeyRotationIsSuitableControlPlane ensures that a control plane node has not been deleted and has a valid
// node associated with it.
func encryptionKeyRotationIsSuitableControlPlane(entry *planEntry) bool {
	return isControlPlane(entry) && entry.Machine.DeletionTimestamp == nil && entry.Machine.Status.NodeRef != nil && rke2.Ready.IsTrue(entry.Machine)
}

// encryptionKeyRotationRestartNodes will restart the leader to ensure that the current encryption key rotation status
// is persisted to the datastore. With a single node control plane, the apiserver container needs to be restarted,
// however with an HA control plane, the leader must first be restarted then all the followers, as the followers will
// update their status when reading from the datastore on startup of the rke2/k3s services.
func (p *Planner) encryptionKeyRotationRestartNodes(cp *rkev1.RKEControlPlane, clusterPlan *plan.Plan, leader *planEntry, init *planEntry) error {
	// If leader is not the init, it must be restarted first or the datastore update fails.
	if init != nil && leader.Machine.Name != init.Machine.Name {
		_, err := p.encryptionKeyRotationRestartService(cp, init, "", false)
		if err != nil {
			return err
		}
	}
	var (
		leaderStage string
		err         error
	)
	if rke2.GetRuntime(cp.Spec.KubernetesVersion) == rke2.RuntimeRKE2 && len(collect(clusterPlan, encryptionKeyRotationIsSuitableControlPlane)) == 1 {
		leaderStage, err = p.encryptionKeyRotationRestartSingleNodeAPIServer(cp, leader)
	} else {
		leaderStage, err = p.encryptionKeyRotationRestartServices(cp, clusterPlan, leader)
	}
	if err != nil {
		return err
	}

	// a phase has been completed at least once for this generation
	if leaderStage == encryptionKeyRotationStageReencryptFinished &&
		cp.Status.RotateEncryptionKeys.Stage != "" {
		return p.encryptionKeyRotationFinished(cp)
	}

	cp.Status.RotateEncryptionKeys.Stage = leaderStage
	err = p.encryptionKeyRotationUpdateControlPlanePhase(cp, rkev1.RotateEncryptionKeysPhaseApplyLeader)
	if err != nil {
		return err
	}
	return generic.ErrSkip
}

// encryptionKeyRotationIsControlPlaneAndNotLeader allows us to filter cluster plans to restart healthy follower nodes.
func encryptionKeyRotationIsControlPlaneAndNotLeader(cp *rkev1.RKEControlPlane) roleFilter {
	return func(entry *planEntry) bool {
		return encryptionKeyRotationIsSuitableControlPlane(entry) &&
			cp.Annotations[encryptionKeyRotationLeaderAnnotation] != entry.Machine.Name
	}
}

// encryptionKeyRotationRestartServices restarts the leader's server service, extracting the current stage afterwards.
// The followers (if any exist) are subsequently restarted.
func (p *Planner) encryptionKeyRotationRestartServices(cp *rkev1.RKEControlPlane, clusterPlan *plan.Plan, leader *planEntry) (string, error) {
	leaderStage, err := p.encryptionKeyRotationRestartService(cp, leader, "", true)
	if err != nil {
		return "", err
	}

	for _, entry := range collect(clusterPlan, encryptionKeyRotationIsControlPlaneAndNotLeader(cp)) {
		stage, err := p.encryptionKeyRotationRestartService(cp, entry, leaderStage, true)
		if err != nil {
			return "", err
		}

		if stage != leaderStage {
			// secrets-encrypt command was run on another node, restart
			logrus.Infof("rkecluster %s/%s: leader [%s] with %s stage and follower [%s] with %s stage", cp.Namespace, cp.Spec.ClusterName, leader.Machine.Status.NodeRef.Name, leaderStage, entry.Machine.Status.NodeRef.Name, stage)
			return "", p.restartEncryptionKeyRotation(cp)
		}
	}

	return leaderStage, nil
}

// encryptionKeyRotationRestartSingleNodeAPIServer restarts the apiserver container on the server, waits until it is
// healthy again, and then waits until secrets-encrypt status can be successfully queried.
func (p *Planner) encryptionKeyRotationRestartSingleNodeAPIServer(cp *rkev1.RKEControlPlane, leader *planEntry) (string, error) {
	nodePlan := plan.NodePlan{
		Files: []plan.File{
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationRestartAPIServerScript)),
				Path:    encryptionKeyRotationRestartAPIServerScriptPath,
			},
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationGetAPIServerStatusScript)),
				Path:    encryptionKeyRotationGetAPIServerStatusScriptPath,
			},
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSecretsEncryptStatusScript)),
				Path:    encryptionKeyRotationWaitForSecretsEncryptStatusScriptPath,
			},
		},
		Instructions: []plan.OneTimeInstruction{
			p.encryptionKeyRotationRestartApiserver(cp),
			encryptionKeyRotationWaitForSecretsEncryptStatus(cp, ""),
			encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction(cp, ""),
		},
		PeriodicInstructions: []plan.PeriodicInstruction{
			p.encryptionKeyRotationApiserverStatus(cp),
		},
	}
	err := assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), leader, nodePlan, 0, 0)
	if isErrWaiting(err) {
		if strings.HasPrefix(err.Error(), "starting") {
			logrus.Infof("rkecluster %s/%s: restarting apiserver on %s", cp.Namespace, cp.Spec.ClusterName, leader.Machine.Name)
			rke2.Ready.Message(cp, fmt.Sprintf("encryption key rotation: restarting apiserver for %s", leader.Machine.Name))
			cp, err = p.rkeControlPlanes.UpdateStatus(cp)
			if err != nil {
				return "", p.enqueueAndSkip(cp)
			}
		}
		return "", p.enqueueAndSkip(cp)
	} else if err != nil {
		return "", err
	}

	if _, ok := cp.Annotations[encryptionKeyRotationApiserverIDAnnotation]; !ok {
		apiserverID := strings.TrimSuffix(string(leader.Plan.Output[encryptionKeyRotationRestartApiserverCommand]), "\n")
		if len(apiserverID) != 13 { // not a valid container id
			return "", p.enqueueAndSkip(cp)
		}
		cp.Annotations[encryptionKeyRotationApiserverIDAnnotation] = apiserverID
		cp, err = p.rkeControlPlanes.Update(cp)
		if err != nil {
			return "", err
		}
		return "", p.enqueueAndSkip(cp)
	}

	apiserverStatus := strings.TrimSuffix(string(leader.Plan.PeriodicOutput[encryptionKeyRotationApiserverStatusCommand].Stdout), "\n")
	if apiserverStatus == "" || cp.Annotations[encryptionKeyRotationApiserverIDAnnotation] == apiserverStatus || len(apiserverStatus) != 13 { // not a valid container id
		return "", p.enqueueAndSkip(cp)
	}

	return encryptionKeyRotationSecretsEncryptStageFromOneTimeStatus(leader)
}

// encryptionKeyRotationRestartService restarts the server unit on the downstream node, waits until secrets-encrypt
// status can be successfully queried, and then gets the status. leaderStage is allowed to be empty if entry is the
// leader.
func (p *Planner) encryptionKeyRotationRestartService(cp *rkev1.RKEControlPlane, entry *planEntry, leaderStage string, getStatus bool) (string, error) {
	nodePlan := plan.NodePlan{
		Instructions: []plan.OneTimeInstruction{
			encryptionKeyRotationRestartInstruction(cp, leaderStage),
		},
	}
	if getStatus {
		nodePlan.Files = []plan.File{
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSystemctlStatus)),
				Path:    encryptionKeyRotationWaitForSystemctlStatusPath,
			},
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSecretsEncryptStatusScript)),
				Path:    encryptionKeyRotationWaitForSecretsEncryptStatusScriptPath,
			},
		}
		nodePlan.Instructions = []plan.OneTimeInstruction{
			encryptionKeyRotationWaitForSystemctlStatusInstruction(cp, leaderStage),
			encryptionKeyRotationRestartInstruction(cp, leaderStage),
			encryptionKeyRotationWaitForSecretsEncryptStatus(cp, leaderStage),
			encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction(cp, leaderStage),
		}
	}
	err := assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), entry, nodePlan, 0, 0)
	if isErrWaiting(err) {
		if strings.HasPrefix(err.Error(), "starting") {
			logrus.Infof("rkecluster %s/%s: restarting %s on %s", cp.Namespace, cp.Spec.ClusterName, rke2.GetRuntimeServerUnit(cp.Spec.KubernetesVersion), entry.Machine.Name)
			rke2.Ready.Message(cp, fmt.Sprintf("encryption key rotation: restarting %s", entry.Machine.Name))
			cp, err = p.rkeControlPlanes.UpdateStatus(cp)
			if err != nil {
				return "", p.enqueueAndSkip(cp)
			}
		}
		return "", p.enqueueAndSkip(cp)
	} else if err != nil {
		return "", err
	}

	if !getStatus {
		return "", nil
	}

	stage, err := encryptionKeyRotationSecretsEncryptStageFromOneTimeStatus(entry)
	if err != nil {
		return "", p.enqueueIfErrWaiting(cp, err)
	}
	return stage, nil
}

// encryptionKeyRotationApplyLeader will run the next secrets-encrypt command, and scrape output to ensure that it was
// successful. If the secrets-encrypt command does not exist on the plan, that means this is the first entrypoint, and
// it must be added, otherwise reenqueue until the plan is in sync.
func (p *Planner) encryptionKeyRotationApplyLeader(cp *rkev1.RKEControlPlane, clusterPlan *plan.Plan, leader *planEntry) error {
	if _, ok := cp.Annotations[encryptionKeyRotationApiserverIDAnnotation]; ok {
		delete(cp.Annotations, encryptionKeyRotationApiserverIDAnnotation)
		return p.enqueueIfErrWaiting(p.rkeControlPlanes.Update(cp))
	}

	apply, err := encryptionKeyRotationSecretsEncryptInstruction(cp, cp.Status.RotateEncryptionKeys.Stage)
	if err != nil {
		return p.restartEncryptionKeyRotation(cp)
	}

	rke2.Ready.Message(cp, fmt.Sprintf("encryption key rotation: running secrets-encrypt %s", apply.Args[1]))
	cp, err = p.rkeControlPlanes.UpdateStatus(cp)
	if err != nil {
		return p.enqueueAndSkip(cp)
	}

	nodePlan := plan.NodePlan{
		Files: []plan.File{
			{
				Content: base64.StdEncoding.EncodeToString([]byte(encryptionKeyRotationWaitForSecretsEncryptStatusScript)),
				Path:    encryptionKeyRotationWaitForSecretsEncryptStatusScriptPath,
			},
		},
		Instructions: []plan.OneTimeInstruction{
			apply,
			encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction(cp, ""),
		},
		PeriodicInstructions: []plan.PeriodicInstruction{
			encryptionKeyRotationSecretsEncryptStatusPeriodicInstruction(cp),
		},
	}
	err = assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), leader, nodePlan, 1, 1)
	if err != nil {
		if isErrWaiting(err) {
			if strings.HasPrefix(err.Error(), "starting") {
				logrus.Infof("rkecluster %s/%s: applying encryption key rotation stage command: [%s]", cp.Namespace, cp.Spec.ClusterName, apply.Args[1])
			}
			return p.enqueueAndSkip(cp)
		}
		return p.restartEncryptionKeyRotation(cp)
	}
	stage, err := encryptionKeyRotationSecretsEncryptStageFromOneTimeStatus(leader)
	if err != nil {
		if isErrWaiting(err) {
			return p.enqueueAndSkip(cp)
		}
		return p.restartEncryptionKeyRotation(cp)
	}
	periodic, err := encryptionKeyRotationSecretsEncryptStageFromPeriodic(leader)
	if err != nil {
		if isErrWaiting(err) {
			return p.enqueueAndSkip(cp)
		}
		return p.restartEncryptionKeyRotation(cp)
	}
	allowed := encryptionKeyRotationIsCurrentStageAllowed(stage, leader)
	if allowed {
		if stage == encryptionKeyRotationStageReencryptRequest || stage == encryptionKeyRotationStageReencryptActive {
			if periodic != encryptionKeyRotationStageReencryptFinished {
				return p.enqueueAndSkip(cp)
			}
		}

		// successful restart, complete same phases for rotate & reencrypt
		logrus.Infof("rkecluster %s/%s: successfully applied encryption key rotation stage command: [%s]", cp.Namespace, cp.Spec.ClusterName, leader.Plan.Plan.Instructions[0].Args[1])

		nodes := collect(clusterPlan, encryptionKeyRotationIsSuitableControlPlane)
		// it is not necessary to restart a leader node after single node rotation
		if rke2.GetRuntime(cp.Spec.KubernetesVersion) == rke2.RuntimeRKE2 && len(nodes) == 1 {
			// a phase has been completed at least once for this generation
			if periodic == encryptionKeyRotationStageReencryptFinished &&
				cp.Status.RotateEncryptionKeys.Stage != "" {
				return p.encryptionKeyRotationFinished(cp)
			}
		}
	}

	return p.restartEncryptionKeyRotation(cp)
}

// encryptionKeyRotationUpdateControlPlanePhase updates the control plane status if it has not been done yet.
func (p *Planner) encryptionKeyRotationUpdateControlPlanePhase(cp *rkev1.RKEControlPlane, phase rkev1.RotateEncryptionKeysPhase) error {
	if cp.Status.RotateEncryptionKeysPhase != phase {
		cp = cp.DeepCopy()
		logrus.Infof("rkecluster %s/%s: applying encryption key rotation phase: [%s]", cp.Namespace, cp.Spec.ClusterName, phase)
		cp.Status.RotateEncryptionKeysPhase = phase
		rke2.Ready.Message(cp, fmt.Sprintf("applying encryption key rotation phase: [%s]", phase))
		rke2.Ready.Reason(cp, rke2.RotatingEncryptionKeysReason)
		_, err := p.rkeControlPlanes.UpdateStatus(cp)
		if err != nil {
			return err
		}
	}
	return nil
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
		return "", ErrWaitingf("Could not extract current status from plan for [%s]: no output for status", plan.Machine.Name)
	}
	status, err := encryptionKeyRotationStageFromOutput(plan, string(output))
	return status, err
}

// encryptionKeyRotationStageFromOutput parses the output of a secrets-encrypt status command.
func encryptionKeyRotationStageFromOutput(plan *planEntry, output string) (string, error) {
	a := strings.Split(output, "\n")
	if len(a) < 2 {
		return "", ErrWaitingf("Could not extract current stage from plan for [%s]: status output is incomplete", plan.Machine.Name)
	}
	a = strings.Split(a[1], ": ")
	if len(a) < 2 {
		return "", ErrWaitingf("Could not extract current stage from plan for [%s]: status output is partially complete", plan.Machine.Name)
	}
	status := a[1]
	return status, nil
}

// encryptionKeyRotationSecretsEncryptInstruction generates a secrets-encrypt command to run on the leader node given
// the current secrets-encrypt status.
func encryptionKeyRotationSecretsEncryptInstruction(cp *rkev1.RKEControlPlane, stage string) (plan.OneTimeInstruction, error) {
	var command string
	switch stage {
	case encryptionKeyRotationStageStart, encryptionKeyRotationStageReencryptFinished:
		command = encryptionKeyRotationCommandPrepare
	case encryptionKeyRotationStagePrepare:
		command = encryptionKeyRotationCommandRotate
	case encryptionKeyRotationStageRotate:
		command = encryptionKeyRotationCommandReencrypt
	default:
		return plan.OneTimeInstruction{}, fmt.Errorf("cannot determine next desired secrets-encrypt status for node with status: [%s]", stage)
	}

	return plan.OneTimeInstruction{
		Name:    encryptionKeyRotationSecretsEncryptApplyCommand,
		Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			command,
		},
	}, nil
}

// encryptionKeyRotationLeaderStageEnv returns an environment variable in order to force followers to rerun their plans
// following a leader command. This is also required if the nodes are out of sync, i.e. someone manually ran
// secrets-encrypt commands on a node that was not the leader before requesting rancher to do the rotation.
func encryptionKeyRotationLeaderStageEnv(stage string) string {
	return fmt.Sprintf("LAST_KNOWN_LEADER_STAGE=%s", stage)
}

// lastUpdateEnv returns an environment variable in order to force followers to rerun their plans in the vent that a
// follower must restart however the leader status didn't change. This is possible when a follower has a state "further"
// than the leader, however it hasn't been restarted.
func lastUpdateEnv(cp *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("LAST_UPDATED=%s", cp.Status.RotateEncryptionKeys.LastRestart)
}

// encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction generates a one time instruction which will scrape the secrets-encrypt
// status.
func encryptionKeyRotationSecretsEncryptStatusOneTimeInstruction(cp *rkev1.RKEControlPlane, leaderStage string) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    encryptionKeyRotationSecretsEncryptStatusCommand,
		Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			"status",
		},
		Env: []string{
			encryptionKeyRotationLeaderStageEnv(leaderStage),
			lastUpdateEnv(cp),
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
		Env: []string{
			lastUpdateEnv(cp),
		},
	}
}

// encryptionKeyRotationWaitForSystemctlStatusInstruction is intended to run after a node is restart, and wait until the node
// is online and able to provide systemctl status, ensuring that the server service is able to be restarted.
func encryptionKeyRotationWaitForSystemctlStatusInstruction(cp *rkev1.RKEControlPlane, leaderStage string) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "wait-for-systemctl-status",
		Command: "sh",
		Args: []string{
			"-x", encryptionKeyRotationWaitForSystemctlStatusPath, rke2.GetRuntimeServerUnit(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			lastUpdateEnv(cp),
			encryptionKeyRotationEndpointEnv,
			encryptionKeyRotationLeaderStageEnv(leaderStage),
		},
		SaveOutput: false,
	}
}

// encryptionKeyRotationRestartInstruction generates a restart command for the rke2/k3s server, using the last known
// leader stage in order to ensure that non-init nodes have a refreshed plan if the leader stage changes. If
// secrets-encrypt commands were run on a node that is not the init node, this ensures that after this situation is
// identified and the leader is restarted, other control plane nodes will be restarted given that the leader stage will
// have changed without a corresponding apply command.
func encryptionKeyRotationRestartInstruction(cp *rkev1.RKEControlPlane, leaderStage string) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "restart-service",
		Command: "systemctl",
		Args: []string{
			"restart", rke2.GetRuntimeServerUnit(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			encryptionKeyRotationLeaderStageEnv(leaderStage),
			lastUpdateEnv(cp),
		},
	}
}

// encryptionKeyRotationWaitForSecretsEncryptStatus is intended to run after a node is restart, and wait until the node
// is online and able to provide secrets-encrypt status, ensuring that subsequent status commands from the system-agent
// will be successful.
func encryptionKeyRotationWaitForSecretsEncryptStatus(cp *rkev1.RKEControlPlane, leaderStage string) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "wait-for-secrets-encrypt-status",
		Command: "sh",
		Args: []string{
			"-x", encryptionKeyRotationWaitForSecretsEncryptStatusScriptPath, rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			lastUpdateEnv(cp),
			encryptionKeyRotationEndpointEnv,
			encryptionKeyRotationLeaderStageEnv(leaderStage),
		},
		SaveOutput: true,
	}
}

// encryptionKeyRotationRestartApiserver returns an instruction for restarting the apiserver during single node
// encryption key rotation, with the path to the crictl binary as its argument.
func (p *Planner) encryptionKeyRotationRestartApiserver(cp *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    encryptionKeyRotationRestartApiserverCommand,
		Command: "sh",
		Args: []string{
			"-xe", encryptionKeyRotationRestartAPIServerScriptPath,
		},
		Env: []string{
			lastUpdateEnv(cp),
			encryptionKeyRotationEndpointEnv,
		},
		SaveOutput: true,
	}
}

// encryptionKeyRotationApiserverStatus returns an instruction for querying the apiserver during single node
// encryption key rotation, with the path to the crictl binary as its argument.
func (p *Planner) encryptionKeyRotationApiserverStatus(cp *rkev1.RKEControlPlane) plan.PeriodicInstruction {
	return plan.PeriodicInstruction{
		Name:    encryptionKeyRotationApiserverStatusCommand,
		Command: "sh",
		Args: []string{
			"-xe", encryptionKeyRotationGetAPIServerStatusScriptPath,
		},
		Env: []string{
			lastUpdateEnv(cp),
			encryptionKeyRotationEndpointEnv,
		},
		PeriodSeconds: 5,
	}
}

// encryptionKeyRotationExtractApplyFromPlan will traverse the nodePlan in search of the last `secrets-encrypt` command
// that applies a new encryption phase, which can be used to verify that the current phase matches the expected phase
// from running said command.
func encryptionKeyRotationExtractApplyFromPlan(entry *planEntry) (plan.OneTimeInstruction, error) {
	if entry.Plan == nil {
		return plan.OneTimeInstruction{}, fmt.Errorf("could not find apply command for [%s]: plan is empty", entry.Machine.Name)
	}
	for _, i := range entry.Plan.Plan.Instructions {
		if i.Name == encryptionKeyRotationSecretsEncryptApplyCommand {
			return i, nil
		}
	}
	return plan.OneTimeInstruction{}, fmt.Errorf("could not find apply command for [%s]", entry.Machine.Name)
}

// encryptionKeyRotationIsCurrentStageAllowed returns the expected secrets-encrypt stage extracted from the
// leader's apply command present in the plan. Since reencrypt can be any of three statuses (reencrypt_request,
// reencrypt_active, and reencrypt_finished), any of these stages are valid, however the first two (request & active) do
// not explicitly indicate that the current command was successful, just that it hasn't failed yet. In certain cases, a
// cluster may never move beyond request and active.
func encryptionKeyRotationIsCurrentStageAllowed(leaderStage string, leader *planEntry) bool {
	applyCommand, err := encryptionKeyRotationExtractApplyFromPlan(leader)
	if err != nil {
		return false
	}
	if len(applyCommand.Args) < 2 {
		return false
	}
	command := applyCommand.Args[1]

	switch command {
	case encryptionKeyRotationCommandPrepare, encryptionKeyRotationCommandRotate:
		return leaderStage == command
	case encryptionKeyRotationCommandReencrypt:
		if leaderStage == encryptionKeyRotationStageReencryptRequest ||
			leaderStage == encryptionKeyRotationStageReencryptActive ||
			leaderStage == encryptionKeyRotationStageReencryptFinished {
			return true
		}
		return false
	}

	return false
}

// encryptionKeyRotationFinished updates the various status objects on the control plane indicating a successful encryption key rotation.
func (p *Planner) encryptionKeyRotationFinished(cp *rkev1.RKEControlPlane) error {
	rke2.Ready.True(cp)
	cp.Status.RotateEncryptionKeys.Generation = cp.Spec.RotateEncryptionKeys.Generation
	cp.Status.RotateEncryptionKeys.Stage = ""
	cp.Status.RotateEncryptionKeysPhase = rkev1.RotateEncryptionKeysPhaseDone

	_, err := p.rkeControlPlanes.UpdateStatus(cp)
	if err != nil {
		return err
	}

	logrus.Infof("rkecluster %s/%s: finished encryption key rotation", cp.Namespace, cp.Spec.ClusterName)
	return generic.ErrSkip
}

// encryptionKeyRotationFailed updates the various status objects on the control plane, allowing the cluster to
// continue the reconciliation loop. Encryption key rotation will not be restarted again until requested.
func (p *Planner) encryptionKeyRotationFailed(cp *rkev1.RKEControlPlane) error {
	rke2.Ready.False(cp)
	cp.Status.RotateEncryptionKeysPhase = rkev1.RotateEncryptionKeysPhaseFailed

	_, err := p.rkeControlPlanes.UpdateStatus(cp)
	if err != nil {
		return err
	}

	logrus.Errorf("rkecluster %s/%s: failed encryption key rotation", cp.Namespace, cp.Spec.ClusterName)
	return fmt.Errorf("encryption key rotation failed")
}
