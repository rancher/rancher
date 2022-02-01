package planner

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/sirupsen/logrus"
)

const (
	EncryptionKeyRotationStart             = "start"
	EncryptionKeyRotationPrepare           = "prepare"
	EncryptionKeyRotationRotate            = "rotate"
	EncryptionKeyRotationReencrypt         = "reencrypt"
	EncryptionKeyRotationReencryptRequest  = "reencrypt_request"
	EncryptionKeyRotationReencryptActive   = "reencrypt_active"
	EncryptionKeyRotationReencryptFinished = "reencrypt_finished"

	SecretsEncryptApplyCommand  = "secrets-encrypt-apply-command"
	SecretsEncryptStatusCommand = "secrets-encrypt-status-command"
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
	_, _, leader, err := p.findInitNode(cp, clusterPlan)
	if err != nil {
		return err
	}
	if leader == nil {
		logrus.Error("Could not rotate encryption keys: unable to find init node")
		return nil
	}

	switch cp.Status.RotateEncryptionKeysPhase {
	case rkev1.RotateEncryptionKeysPhaseStart:
		return p.encryptionKeyRotationStart(cp, leader)
	case rkev1.RotateEncryptionKeysPhaseRestartLeader:
		return p.encryptionKeyRotationRestartLeader(cp, leader)
	case rkev1.RotateEncryptionKeysPhaseVerifyLeaderStatus:
		return p.encryptionKeyRotationVerifyLeaderStatus(cp, leader)
	case rkev1.RotateEncryptionKeysPhaseRestartFollowers:
		return p.encryptionKeyRotationRestartFollowers(cp, clusterPlan, leader)
	case rkev1.RotateEncryptionKeysPhaseApplyLeader:
		return p.encryptionKeyRotationApplyLeader(cp, leader)
	}

	return p.enqueueAndSkip(cp)
}

// shouldRotateEncryptionKeys `true` if encryption key rotation has been requested or has not finished
func shouldRotateEncryptionKeys(cp *rkev1.RKEControlPlane) bool {
	// The controlplane must be initialized before we rotate anything
	if !cp.Status.Initialized {
		return false
	}
	// if a spec is not defined there is nothing to do
	if cp.Spec.RotateEncryptionKeys == nil {
		return false
	}

	// if this generation has already been applied there is no work
	if cp.Status.RotateEncryptionKeysGeneration == cp.Spec.RotateEncryptionKeys.Generation &&
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone {
		return false
	}

	return true
}

// shouldRestartEncryptionKeyRotation `true` if the current phase is unknown, or finished
func shouldRestartEncryptionKeyRotation(cp *rkev1.RKEControlPlane) bool {
	if cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone {
		return true
	}
	if !hasValidRotateEncryptionKeyStatus(cp) {
		return true
	}
	return false
}

// hasValidRotateEncryptionKeyStatus verifies that the control plane encryption key status is an expected value
func hasValidRotateEncryptionKeyStatus(cp *rkev1.RKEControlPlane) bool {
	return cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseStart ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseRestartLeader ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseVerifyLeaderStatus ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseRestartFollowers ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseApplyLeader ||
		cp.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseDone
}

func (p *Planner) restartEncryptionKeyRotation(cp *rkev1.RKEControlPlane) error {
	err := p.encryptionKeyRotationUpdateControlPlanePhase(cp, rkev1.RotateEncryptionKeysPhaseStart)
	if err != nil {
		return err
	}
	return generic.ErrSkip
}

func (p *Planner) encryptionKeyRotationStart(cp *rkev1.RKEControlPlane, leader *planEntry) error {
	return p.encryptionKeyRotationGetLeaderStatus(cp, leader, rkev1.RotateEncryptionKeysPhaseRestartLeader)
}

func (p *Planner) encryptionKeyRotationRestartLeader(cp *rkev1.RKEControlPlane, leader *planEntry) error {
	phase, err := encryptionKeyRotationPhaseFromPeriodic(leader)
	if err != nil {
		return p.enqueueIfErrWaiting(cp, err)
	}
	nodePlan := plan.NodePlan{
		Instructions: []plan.OneTimeInstruction{encryptionKeyRotationRestartInstruction(cp, leader, phase)},
	}
	err = assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), leader, nodePlan, 0)
	if err != nil {
		return p.enqueueIfErrWaiting(cp, err)
	}

	err = p.encryptionKeyRotationUpdateControlPlanePhase(cp, rkev1.RotateEncryptionKeysPhaseVerifyLeaderStatus)
	if err != nil {
		return err
	}
	return generic.ErrSkip
}

func (p *Planner) encryptionKeyRotationVerifyLeaderStatus(cp *rkev1.RKEControlPlane, leader *planEntry) error {
	return p.encryptionKeyRotationGetLeaderStatus(cp, leader, rkev1.RotateEncryptionKeysPhaseRestartFollowers)
}

// encryptionKeyRotationRestartFollowers restarts each follower, and ensures that the status matches the leader node.
// If this is the first time this function has been run this generation, the generation status will be updated, however
// the phase will not be set to done unless the controlplane secrets-encrypt status is uniformly reencrypt_finished, as
// subsequent runs will begin in the reencrypt_finished phase, and require the applyLeader phase to be reached at least
// once before completion.
func (p *Planner) encryptionKeyRotationRestartFollowers(cp *rkev1.RKEControlPlane, clusterPlan *plan.Plan, leader *planEntry) error {
	leaderPhase, err := encryptionKeyRotationPhaseFromPeriodic(leader)
	if err != nil {
		return p.restartEncryptionKeyRotation(cp)
	}

	for _, entry := range collect(clusterPlan, isControlPlaneAndNotInitNode) {
		nodePlan := plan.NodePlan{
			Instructions:         []plan.OneTimeInstruction{encryptionKeyRotationRestartInstruction(cp, entry, leaderPhase)},
			PeriodicInstructions: []plan.PeriodicInstruction{encryptionKeyRotationPeriodicStatusInstruction(cp)},
		}

		err := assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), entry, nodePlan, 0)
		if err != nil {
			return p.enqueueIfErrWaiting(cp, err)
		}

		phase, err := encryptionKeyRotationPhaseFromPeriodic(entry)
		if err != nil {
			return p.enqueueIfErrWaiting(cp, err)
		}
		if phase != leaderPhase {
			logrus.Debugf("Phase desync between leader node [%s] and follower node [%s]", leader.Machine.Status.NodeRef.Name, entry.Machine.Status.NodeRef.Name)
			return p.enqueueAndSkip(cp)
		}
	}

	if cp.Status.RotateEncryptionKeysGeneration == cp.Spec.RotateEncryptionKeys.Generation &&
		leaderPhase == EncryptionKeyRotationReencryptFinished {
		cp.Status.RotateEncryptionKeysGeneration = cp.Spec.RotateEncryptionKeys.Generation
		cp.Status.RotateEncryptionKeysPhase = rkev1.RotateEncryptionKeysPhaseDone
		_, err = p.rkeControlPlanes.UpdateStatus(cp)
		if err != nil {
			return err
		}
		return generic.ErrSkip
	}

	if cp.Status.RotateEncryptionKeysGeneration != cp.Spec.RotateEncryptionKeys.Generation {
		cp.Status.RotateEncryptionKeysGeneration = cp.Spec.RotateEncryptionKeys.Generation
	}
	err = p.encryptionKeyRotationUpdateControlPlanePhase(cp, rkev1.RotateEncryptionKeysPhaseApplyLeader)
	if err != nil {
		return err
	}
	return generic.ErrSkip
}

// encryptionKeyRotationApplyLeader will run the next secrets-encrypt command, and scrape output to ensure that it was
// successful. If the secrets-encrypt command does not exist on the plan, that means this is the first entrypoint and it
// must be added, otherwise reenqueue until the plan is in sync.
func (p *Planner) encryptionKeyRotationApplyLeader(cp *rkev1.RKEControlPlane, leader *planEntry) error {
	var phase string

	_, err := encryptionKeyRotationExtractApplyFromPlan(leader)
	if err != nil {
		phase, err = encryptionKeyRotationPhaseFromPeriodic(leader)
		if err != nil {
			return p.enqueueIfErrWaiting(cp, err)
		}
		apply, err := encryptionKeyRotationSecretsEncryptInstruction(cp, phase)
		if err != nil {
			return p.enqueueIfErrWaiting(cp, err)
		}

		logrus.Debugf("Rotate encryption keys cluster [%s]: running apply command: [%s]", cp.Spec.ClusterName, apply.Args[1])

		nodePlan := plan.NodePlan{
			Instructions:         []plan.OneTimeInstruction{apply, encryptionKeyRotationStatusOneTimeInstruction(cp)},
			PeriodicInstructions: []plan.PeriodicInstruction{encryptionKeyRotationPeriodicStatusInstruction(cp)},
		}
		err = assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), leader, nodePlan, 0)
		if err != nil {
			return p.enqueueIfErrWaiting(cp, err)
		}
	} else if !leader.Plan.InSync {
		return p.enqueueAndSkip(cp)
	}

	phase, err = encryptionKeyRotationPhaseFromOneTime(leader)
	if err != nil {
		// occasionally apply and status may fail but still be in sync: remove apply and retry.
		nodePlan := plan.NodePlan{
			Instructions:         []plan.OneTimeInstruction{encryptionKeyRotationStatusOneTimeInstruction(cp)},
			PeriodicInstructions: []plan.PeriodicInstruction{encryptionKeyRotationPeriodicStatusInstruction(cp)},
		}
		err = assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), leader, nodePlan, 0)
		return p.enqueueIfErrWaiting(cp, err)
	}
	expected, err := encryptionKeyRotationPhaseIsExpectedFromCommand(phase, leader)
	if err != nil {
		return p.enqueueIfErrWaiting(cp, err)
	}
	if !expected {
		return p.restartEncryptionKeyRotation(cp)
	}
	periodicPhase, err := encryptionKeyRotationPhaseFromPeriodic(leader)
	if err != nil {
		return p.enqueueIfErrWaiting(cp, err)
	}
	if phase == EncryptionKeyRotationReencryptRequest || phase == EncryptionKeyRotationReencryptActive {
		if periodicPhase != EncryptionKeyRotationReencryptFinished {
			return p.enqueueAndSkip(cp)
		}
	}

	err = p.encryptionKeyRotationUpdateControlPlanePhase(cp, rkev1.RotateEncryptionKeysPhaseStart)
	if err != nil {
		return err
	}
	return generic.ErrSkip
}

func (p *Planner) encryptionKeyRotationGetLeaderStatus(cp *rkev1.RKEControlPlane, leader *planEntry, nextPhase rkev1.RotateEncryptionKeysPhase) error {
	nodePlan := plan.NodePlan{
		PeriodicInstructions: []plan.PeriodicInstruction{encryptionKeyRotationPeriodicStatusInstruction(cp)},
	}
	err := assignAndCheckPlan(p.store, fmt.Sprintf("encryption key rotation: [%s]", cp.Status.RotateEncryptionKeysPhase), leader, nodePlan, 0)
	if err != nil {
		return p.enqueueIfErrWaiting(cp, err)
	}

	_, err = encryptionKeyRotationPhaseFromPeriodic(leader)
	if err != nil {
		return p.enqueueIfErrWaiting(cp, err)
	}

	err = p.encryptionKeyRotationUpdateControlPlanePhase(cp, nextPhase)
	if err != nil {
		return err
	}
	return generic.ErrSkip
}

func (p *Planner) encryptionKeyRotationUpdateControlPlanePhase(cp *rkev1.RKEControlPlane, phase rkev1.RotateEncryptionKeysPhase) error {
	if cp.Status.RotateEncryptionKeysPhase != phase {
		cp.Status.RotateEncryptionKeysPhase = phase
		_, err := p.rkeControlPlanes.UpdateStatus(cp)
		if err != nil {
			return err
		}
	}
	return nil
}

// encryptionKeyRotationPhaseFromPeriodic will attempt to extract the current phase (secrets-encrypt status) from the
// plan by parsing the periodic output, or the one time output. In most cases, the periodic output will be the only one
// populated, however the leader requires deterministic status when entering the apply sequence, as clusters with few
// nodes can complete the 'prepare' sequence before the leader has time to rerun the periodic status command, which
// could potentially cause a duplicate apply.
func encryptionKeyRotationPhaseFromPeriodic(plan *planEntry) (string, error) {
	output, ok := plan.Plan.PeriodicOutput[SecretsEncryptStatusCommand]
	if !ok {
		return "", ErrWaitingf("Could not extract current status from plan for [%s]: no output for status", plan.Machine.Name)
	}
	phase, err := encryptionKeyRotationPhaseFromOutput(plan, string(output.Stdout))
	return phase, err
}

func encryptionKeyRotationPhaseFromOneTime(plan *planEntry) (string, error) {
	output, ok := plan.Plan.Output[SecretsEncryptStatusCommand]
	if !ok {
		return "", ErrWaitingf("Could not extract current status from plan for [%s]: no output for status", plan.Machine.Name)
	}
	phase, err := encryptionKeyRotationPhaseFromOutput(plan, string(output))
	return phase, err
}

func encryptionKeyRotationPhaseFromOutput(plan *planEntry, output string) (string, error) {
	a := strings.Split(output, "\n")
	if len(a) < 2 {
		return "", ErrWaitingf("Could not extract current status from plan for [%s]: status output is incomplete", plan.Machine.Name)
	}
	a = strings.Split(a[1], ": ")
	if len(a) < 2 {
		return "", ErrWaitingf("Could not extract current status from plan for [%s]: status output is partially complete", plan.Machine.Name)
	}
	phase := a[1]
	logrus.Debugf("Rotate encryption Status [%s]: %s", plan.Machine.Name, phase)
	return phase, nil
}

func encryptionKeyRotationSecretsEncryptInstruction(cp *rkev1.RKEControlPlane, phase string) (plan.OneTimeInstruction, error) {
	var command string

	switch phase {
	case EncryptionKeyRotationStart, EncryptionKeyRotationReencryptFinished:
		command = EncryptionKeyRotationPrepare
	case EncryptionKeyRotationPrepare:
		command = EncryptionKeyRotationRotate
	case EncryptionKeyRotationRotate:
		command = EncryptionKeyRotationReencrypt
	default:
		return plan.OneTimeInstruction{}, fmt.Errorf("cannot create secrets-encrypt instruction for node with phase: [%s]", phase)
	}

	return plan.OneTimeInstruction{
		Name:    SecretsEncryptApplyCommand,
		Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			command,
		},
	}, nil
}

func encryptionKeyRotationStatusOneTimeInstruction(cp *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    SecretsEncryptStatusCommand,
		Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			"status",
		},
		SaveOutput: true,
	}
}

func encryptionKeyRotationPeriodicStatusInstruction(cp *rkev1.RKEControlPlane) plan.PeriodicInstruction {
	return plan.PeriodicInstruction{
		Name:    SecretsEncryptStatusCommand,
		Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
		Args: []string{
			"secrets-encrypt",
			"status",
		},
		PeriodSeconds: 5,
	}
}

// encryptionKeyRotationRestartInstruction generates a restart command for the rke2/k3s server, using the last known leader phase in order to
// ensure that non-init nodes have a refreshed plan if the leader phase changes. If secrets-encrypt commands were run on
// a node that is not the init node, this ensures that after this situation is identified and the leader is restarted,
// other control plane nodes will be restarted given that the leader phase will have changed without a corresponding
// apply command.
func encryptionKeyRotationRestartInstruction(cp *rkev1.RKEControlPlane, entry *planEntry, leaderPhase string) plan.OneTimeInstruction {
	var lastKnownLeaderPhaseEnv string
	switch entry.Metadata.Labels[rke2.CattleOSLabel] {
	case windows:
		lastKnownLeaderPhaseEnv = fmt.Sprintf("$env:LAST_KNOWN_LEADER_PHASE=%s", leaderPhase)
	default:
		lastKnownLeaderPhaseEnv = fmt.Sprintf("LAST_KNOWN_LEADER_PHASE=%s", leaderPhase)
	}
	return plan.OneTimeInstruction{
		Name:    "restart-service",
		Command: "systemctl",
		Args: []string{
			"restart", rke2.GetRuntimeServerUnit(cp.Spec.KubernetesVersion),
		},
		Env: []string{
			lastKnownLeaderPhaseEnv,
		},
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
		if i.Name == SecretsEncryptApplyCommand {
			return i, nil
		}
	}
	return plan.OneTimeInstruction{}, fmt.Errorf("could not find apply command for [%s]", entry.Machine.Name)
}

// encryptionKeyRotationPhaseIsExpectedFromCommand returns the expected secrets-encrypt phase extracted from the leader's apply command present in the plan.
func encryptionKeyRotationPhaseIsExpectedFromCommand(leaderPhase string, leader *planEntry) (bool, error) {
	applyCommand, err := encryptionKeyRotationExtractApplyFromPlan(leader)
	if err != nil {
		return false, err
	}
	if len(applyCommand.Args) < 2 {
		return false, fmt.Errorf("could not extract secrets-encrypt command from plan")
	}
	command := applyCommand.Args[1]

	switch command {
	case EncryptionKeyRotationPrepare, EncryptionKeyRotationRotate:
		return leaderPhase == command, nil
	case EncryptionKeyRotationReencrypt:
		if leaderPhase == EncryptionKeyRotationReencryptRequest ||
			leaderPhase == EncryptionKeyRotationReencryptActive ||
			leaderPhase == EncryptionKeyRotationReencryptFinished {
			return true, nil
		}
	}

	return false, fmt.Errorf("could not extract secrets-encrypt command from plan: unknown command [%s]", command)
}
