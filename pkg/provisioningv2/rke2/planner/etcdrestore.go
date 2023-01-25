package planner

import (
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
)

const ETCDRestoreMessage = "etcd restore"

func (p *Planner) setEtcdSnapshotRestoreState(status rkev1.RKEControlPlaneStatus, restore *rkev1.ETCDSnapshotRestore, phase rkev1.ETCDSnapshotPhase) (rkev1.RKEControlPlaneStatus, error) {
	if !equality.Semantic.DeepEqual(status.ETCDSnapshotRestore, restore) || status.ETCDSnapshotRestorePhase != phase {
		status.ETCDSnapshotRestore = restore
		status.ETCDSnapshotRestorePhase = phase
		return status, ErrWaiting("refreshing etcd restore state")
	}
	return status, nil
}

func (p *Planner) resetEtcdSnapshotRestoreState(status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotRestore == nil && status.ETCDSnapshotRestorePhase == "" {
		return status, nil
	}
	return p.setEtcdSnapshotRestoreState(status, nil, "")
}

func (p *Planner) startOrRestartEtcdSnapshotRestore(status rkev1.RKEControlPlaneStatus, restore *rkev1.ETCDSnapshotRestore) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotRestore == nil || !equality.Semantic.DeepEqual(restore, status.ETCDSnapshotRestore) {
		return p.setEtcdSnapshotRestoreState(status, restore, rkev1.ETCDSnapshotPhaseStarted)
	}
	return status, nil
}

func (p *Planner) runEtcdSnapshotRestorePlan(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshot, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	var joinServer string
	var err error
	isS3 := snapshot.SnapshotFile.S3 != nil
	if isS3 {
		joinServer, err = p.electInitNode(controlPlane, clusterPlan)
		if err != nil {
			return err
		}
	} else {
		logrus.Infof("rkecluster %s/%s: re-electing specific init node for etcd snapshot restore", controlPlane.Namespace, controlPlane.Spec.ClusterName)
		joinServer, err = p.designateInitNodeByMachineID(controlPlane, clusterPlan, snapshot.Labels[rke2.MachineIDLabel])
		if err != nil {
			return err
		}
	}
	servers := collect(clusterPlan, isInitNode)

	for _, server := range servers {
		if isS3 ||
			(server.Machine.Labels[rke2.MachineIDLabel] != "" && snapshot.Labels[rke2.MachineIDLabel] != "" &&
				server.Machine.Labels[rke2.MachineIDLabel] == snapshot.Labels[rke2.MachineIDLabel]) {
			restorePlan, err := p.generateEtcdSnapshotRestorePlan(controlPlane, snapshot, tokensSecret, server, joinServer)
			if err != nil {
				return err
			}
			return assignAndCheckPlan(p.store, ETCDRestoreMessage, server, restorePlan, 0, 0)
		}
	}

	return ErrWaiting("failed to find etcd node to restore on")
}

// generateEtcdSnapshotRestorePlan returns a node plan that contains instructions to stop etcd, remove the tombstone file (if one exists), then restore etcd in that order.
func (p *Planner) generateEtcdSnapshotRestorePlan(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshot, tokensSecret plan.Secret, server *planEntry, joinServer string) (plan.NodePlan, error) {
	if controlPlane.Spec.ETCDSnapshotRestore == nil {
		return plan.NodePlan{}, fmt.Errorf("ETCD Snapshot restore was not defined")
	}
	args := []string{
		"server",
		"--cluster-reset",
	}

	if snapshot.SnapshotFile.S3 == nil {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshot.SnapshotFile.Name), "--etcd-s3=false")
	} else {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=%s", snapshot.SnapshotFile.Name))
	}

	s3Args, s3Env, s3Files, err := p.etcdS3Args.ToArgs(snapshot.SnapshotFile.S3, controlPlane, "etcd-", true)
	if err != nil {
		return plan.NodePlan{}, err
	}

	// This is likely redundant but can make sense in the event that there is an external watchdog.
	stopPlan, err := p.generateStopServiceAndKillAllPlan(controlPlane, tokensSecret, server, joinServer)
	if err != nil {
		return plan.NodePlan{}, err
	}

	// make sure to install the desired version before performing restore
	stopPlan.Instructions = append(stopPlan.Instructions, p.generateInstallInstructionWithSkipStart(controlPlane, server))

	planInstructions := append(stopPlan.Instructions,
		plan.OneTimeInstruction{
			Name:    "remove-etcd-db-dir",
			Command: "rm",
			Args: []string{
				"-rf",
				fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd", rke2.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
			}})

	nodePlan := plan.NodePlan{
		Files: s3Files,
		Instructions: append(planInstructions, plan.OneTimeInstruction{
			Name:    "restore",
			Env:     s3Env,
			Args:    append(args, s3Args...),
			Command: rke2.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
		}),
	}

	return nodePlan, nil
}

func (p *Planner) generateStopServiceAndKillAllPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, server *planEntry, joinServer string) (plan.NodePlan, error) {
	nodePlan, _, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, server, joinServer)
	if err != nil {
		return nodePlan, err
	}
	runtime := rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)
	nodePlan.Instructions = append(nodePlan.Instructions,
		generateKillAllInstruction(runtime))
	return nodePlan, nil
}

func generateKillAllInstruction(runtime string) plan.OneTimeInstruction {
	killAllScript := runtime + "-killall.sh"
	return plan.OneTimeInstruction{
		Name:    "shutdown",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			fmt.Sprintf("if [ -z $(command -v %s) ] && [ -z $(command -v %s) ]; then echo %s does not appear to be installed; exit 0; else %s; fi", runtime, killAllScript, runtime, killAllScript),
		},
	}
}

func generateCreateEtcdTombstoneInstruction(controlPlane *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "create-etcd-tombstone",
		Command: "touch",
		Args: []string{
			fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd/tombstone", rke2.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
		},
	}
}

// runEtcdRestoreServiceStop generates service stop plans for every non-windows node in the cluster and
// assigns/checks the plans to ensure they were successful
func (p *Planner) runEtcdRestoreServiceStop(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshot, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	var joinServer string
	var err error
	isS3 := snapshot.SnapshotFile.S3 != nil
	if !isS3 { // In the event that we are restoring a local snapshot, we need to reset our initNode
		joinServer, err = p.designateInitNodeByMachineID(controlPlane, clusterPlan, snapshot.Labels[rke2.MachineIDLabel])
		if err != nil {
			return fmt.Errorf("error while designating init node during control plane/etcd stop: %w", err)
		}
	} else {
		joinServer, err = p.electInitNode(controlPlane, clusterPlan)
		if err != nil {
			return err
		}
	}
	servers := collect(clusterPlan, anyRoleWithoutWindows)
	updated := false
	for _, server := range servers {
		stopPlan, err := p.generateStopServiceAndKillAllPlan(controlPlane, tokensSecret, server, joinServer)
		if err != nil {
			return err
		}
		if isEtcd(server) {
			stopPlan.Instructions = append(stopPlan.Instructions, generateCreateEtcdTombstoneInstruction(controlPlane))
		}
		if server.Plan == nil || !equality.Semantic.DeepEqual(server.Plan.Plan, stopPlan) {
			if err := p.store.UpdatePlan(server, stopPlan, 0, 0); err != nil {
				return err
			}
			updated = true
		}
	}

	// If any of the controlplane/etcd node plans were updated, return an errwaiting message for shutting down control plane and etcd
	if updated {
		return ErrWaiting("stopping " + rke2.GetRuntime(controlPlane.Spec.KubernetesVersion) + " services on control plane and etcd machines/nodes")
	}

	for _, server := range servers {
		if !server.Plan.InSync {
			if server.Machine.Status.NodeRef == nil {
				return ErrWaiting(fmt.Sprintf("waiting to stop %s services on machine [%s]", rke2.GetRuntime(controlPlane.Spec.KubernetesVersion), server.Machine.Name))
			}
			return ErrWaiting(fmt.Sprintf("waiting to stop %s services on node [%s]", rke2.GetRuntime(controlPlane.Spec.KubernetesVersion), server.Machine.Status.NodeRef.Name))
		}
	}

	return nil
}

// runEtcdRestoreServiceStart walks through the reconciliation process for the entire cluster.
// Notably, this function will blatantly ignore drain and concurrency options, as during an etcd snapshot restore, there is no necessity to drain nodes.
func (p *Planner) runEtcdRestoreServiceStart(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	if err := p.runEtcdSnapshotManagementServiceStart(controlPlane, tokensSecret, clusterPlan, isControlPlaneEtcd, "etcd restore"); err != nil {
		return err
	}
	return p.runEtcdSnapshotWorkerServiceStart(controlPlane, tokensSecret, clusterPlan, "etcd restore")
}

// runEtcdSnapshotManagementServiceStart walks through the reconciliation process for the controlplane and etcd nodes.
// Notably, this function will blatantly ignore drain and concurrency options, as during an etcd snapshot operation, there is no necessity to drain nodes.
func (p *Planner) runEtcdSnapshotManagementServiceStart(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan, include roleFilter, operation string) error {
	_, joinServer, initNode, err := p.findInitNode(controlPlane, clusterPlan)
	if err != nil {
		return err
	}
	if joinServer == "" {
		return fmt.Errorf("error encountered restarting cluster during %s, joinServer was empty", operation)
	}

	plan, err := p.desiredPlan(controlPlane, tokensSecret, initNode, "")
	if err != nil {
		return err
	}

	if err = assignAndCheckPlan(p.store, fmt.Sprintf("%s bootstrap restart", operation), initNode, plan, 1, -1); err != nil {
		return err
	}

	for _, entry := range collect(clusterPlan, include) {
		if isInitNodeOrDeleting(entry) {
			continue
		}
		plan, err = p.desiredPlan(controlPlane, tokensSecret, entry, joinServer)
		if err != nil {
			return err
		}
		if err = assignAndCheckPlan(p.store, fmt.Sprintf("%s management plane restart", operation), entry, plan, 1, -1); err != nil {
			return err
		}
	}
	return nil
}

// runEtcdSnapshotControlPlaneEtcdServiceStart walks through the reconciliation process for the worker nodes.
// Notably, this function will blatantly ignore drain and concurrency options, as during an etcd snapshot operation, there is no necessity to drain nodes.
func (p *Planner) runEtcdSnapshotWorkerServiceStart(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan, operation string) error {
	joinServer := getControlPlaneJoinURL(clusterPlan)
	if joinServer == "" {
		return ErrWaiting("waiting for control plane to be available")
	}

	for _, entry := range collect(clusterPlan, isOnlyWorker) {
		if isInitNodeOrDeleting(entry) {
			continue
		}
		plan, err := p.desiredPlan(controlPlane, tokensSecret, entry, joinServer)
		if err != nil {
			return err
		}
		if err = assignAndCheckPlan(p.store, fmt.Sprintf("%s worker restart", operation), entry, plan, 1, -1); err != nil {
			return err
		}
	}
	return nil
}

func (p *Planner) retrieveEtcdSnapshot(controlPlane *rkev1.RKEControlPlane) (*rkev1.ETCDSnapshot, error) {
	if controlPlane == nil {
		return nil, fmt.Errorf("controlplane was nil")
	}
	if controlPlane.Spec.ETCDSnapshotRestore == nil {
		return nil, fmt.Errorf("etcdsnapshotrestore spec was nil")
	}
	if controlPlane.Spec.ClusterName == "" {
		return nil, fmt.Errorf("cluster name on rkecontrolplane %s/%s was blank", controlPlane.Namespace, controlPlane.Name)
	}
	return p.etcdSnapshotCache.Get(controlPlane.Namespace, controlPlane.Spec.ETCDSnapshotRestore.Name)
}

// restoreEtcdSnapshot is called multiple times during an etcd snapshot restoration.
// restoreEtcdSnapshot utilizes the status of the corresponding control plane object of the cluster to track state
// The phases are in order:
// Started -> When the phase is started, it gets set to shutdown
// Shutdown -> When the phase is shutdown, it attempts to shut down etcd on all nodes (stop etcd)
// Restore ->  When the phase is restore, it attempts to restore etcd
// Finished -> When the phase is finished, Restore returns nil.
func (p *Planner) restoreEtcdSnapshot(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	if cp.Spec.ETCDSnapshotRestore == nil || cp.Spec.ETCDSnapshotRestore.Name == "" {
		return p.resetEtcdSnapshotRestoreState(status)
	}

	if status, err := p.startOrRestartEtcdSnapshotRestore(status, cp.Spec.ETCDSnapshotRestore); err != nil {
		return status, err
	}

	switch cp.Status.ETCDSnapshotRestorePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseShutdown)
	case rkev1.ETCDSnapshotPhaseShutdown:
		snapshot, err := p.retrieveEtcdSnapshot(cp)
		if err != nil {
			return status, err
		}
		if err = p.runEtcdRestoreServiceStop(cp, snapshot, tokensSecret, clusterPlan); err != nil {
			return status, err
		}
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseRestore)
	case rkev1.ETCDSnapshotPhaseRestore:
		snapshot, err := p.retrieveEtcdSnapshot(cp)
		if err != nil {
			return status, err
		}
		if err = p.runEtcdSnapshotRestorePlan(cp, snapshot, tokensSecret, clusterPlan); err != nil {
			return status, err
		}
		status.ConfigGeneration++
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseRestartCluster)
	case rkev1.ETCDSnapshotPhaseRestartCluster:
		if err := p.runEtcdRestoreServiceStart(cp, tokensSecret, clusterPlan); err != nil {
			return status, err
		}
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseFinished)
	case rkev1.ETCDSnapshotPhaseFinished:
		return status, nil
	default:
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseStarted)
	}
}
