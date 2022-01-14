package planner

import (
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	rancherruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
)

const ETCDRestoreMessage = "etcd restore"

func (p *Planner) setEtcdSnapshotRestoreState(controlPlane *rkev1.RKEControlPlane, status *rkev1.ETCDSnapshotRestore, phase rkev1.ETCDSnapshotPhase) error {
	controlPlane = controlPlane.DeepCopy()
	controlPlane.Status.ETCDSnapshotRestorePhase = phase
	controlPlane.Status.ETCDSnapshotRestore = status
	_, err := p.rkeControlPlanes.UpdateStatus(controlPlane)
	if err != nil {
		return err
	}
	return ErrWaiting("refreshing etcd restore state")
}

func (p *Planner) resetEtcdSnapshotRestoreState(controlPlane *rkev1.RKEControlPlane) error {
	if controlPlane.Status.ETCDSnapshotRestore == nil && controlPlane.Status.ETCDSnapshotRestorePhase == "" {
		return nil
	}
	return p.setEtcdSnapshotRestoreState(controlPlane, nil, "")
}

func (p *Planner) startOrRestartEtcdSnapshotRestore(controlPlane *rkev1.RKEControlPlane) error {
	if controlPlane.Status.ETCDSnapshotRestore == nil || !equality.Semantic.DeepEqual(*controlPlane.Spec.ETCDSnapshotRestore, *controlPlane.Status.ETCDSnapshotRestore) {
		return p.setEtcdSnapshotRestoreState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseStarted)
	}
	return nil
}

func (p *Planner) runEtcdSnapshotRestorePlan(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, clusterPlan *plan.Plan) error {
	var joinServer string
	var err error
	if controlPlane.Spec.ETCDSnapshotRestore.S3 == nil { // In the event that we are restoring a local snapshot, we need to reset our initNode
		logrus.Infof("rkecluster %s/%s re-electing specific init node for etcd snapshot restore", controlPlane.Namespace, controlPlane.Spec.ClusterName)
		if controlPlane.Spec.ETCDSnapshotRestore.NodeName != "" {
			joinServer, err = p.designateInitNode(controlPlane, clusterPlan, controlPlane.Spec.ETCDSnapshotRestore.NodeName)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("error attempting to run etcd snapshot restore plan -- s3 info not designed and no nodename designated")
		}
	} else {
		joinServer, err = p.electInitNode(controlPlane, clusterPlan)
		if err != nil {
			return err
		}
	}
	servers := collect(clusterPlan, isEtcd)

	for _, server := range servers {
		if controlPlane.Spec.ETCDSnapshotRestore.S3 != nil ||
			(server.Machine.Status.NodeRef != nil &&
				server.Machine.Status.NodeRef.Name == controlPlane.Spec.ETCDSnapshotRestore.NodeName) {
			restorePlan, err := p.generateEtcdSnapshotRestorePlan(controlPlane, secret, server, joinServer)
			if err != nil {
				return err
			}
			return assignAndCheckPlan(p.store, ETCDRestoreMessage, server, restorePlan, 0)
		}
	}

	return ErrWaiting("failed to find etcd node to restore on")
}

// generateRestoreEtcdSnapshotPlan returns a node plan that contains instructions to stop etcd, remove the tombstone file (if one exists), then restore etcd in that order.
func (p *Planner) generateEtcdSnapshotRestorePlan(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, server planEntry, joinServer string) (plan.NodePlan, error) {
	if controlPlane.Spec.ETCDSnapshotRestore == nil {
		return plan.NodePlan{}, fmt.Errorf("ETCD Snapshot restore was not defined")
	}
	snapshot := controlPlane.Spec.ETCDSnapshotRestore
	args := []string{
		"server",
		"--cluster-reset",
	}

	if snapshot.S3 == nil {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshot.Name))
	} else {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=%s", snapshot.Name))
	}

	s3Args, s3Env, s3Files, err := p.etcdS3Args.ToArgs(snapshot.S3, controlPlane)
	if err != nil {
		return plan.NodePlan{}, err
	}

	// This is likely redundant but can make sense in the event that there is an external watchdog.
	stopPlan, err := p.generateStopServiceAndKillAllPlan(controlPlane, secret, server, joinServer)
	if err != nil {
		return plan.NodePlan{}, err
	}

	planInstructions := append(stopPlan.Instructions, plan.Instruction{
		Name:    "remove-tombstone",
		Command: "rm",
		Args: []string{
			"-f",
			fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd/tombstone", runtime.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
		},
	})

	nodePlan := plan.NodePlan{
		Files: s3Files,
		Instructions: append(planInstructions, plan.Instruction{
			Name:    "restore",
			Env:     s3Env,
			Args:    append(args, s3Args...),
			Command: runtime.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
		}),
	}

	return nodePlan, nil
}

func (p *Planner) generateStopServiceAndKillAllPlan(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, server planEntry, joinServer string) (plan.NodePlan, error) {
	nodePlan, _, err := p.generatePlanWithConfigFiles(controlPlane, secret, server, joinServer)
	if err != nil {
		return nodePlan, err
	}
	nodePlan.Instructions = append(nodePlan.Instructions,
		p.generateInstallInstructionWithSkipStart(controlPlane, server.Machine),
		plan.Instruction{
			Name:    "stop-service",
			Command: "systemctl",
			Args: []string{
				"stop", runtime.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion),
			},
		},
		plan.Instruction{
			Name:    "shutdown",
			Command: runtime.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion) + "-killall.sh",
		})
	return nodePlan, nil
}

func generateCreateEtcdTombstoneInstruction(controlPlane *rkev1.RKEControlPlane) plan.Instruction {
	return plan.Instruction{
		Name:    "create-etcd-tombstone",
		Command: "touch",
		Args: []string{
			fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd/tombstone", runtime.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
		},
	}
}

// runControlPlaneEtcdServiceStop generates service stop plans for every etcd and controlplane node in the cluster and
// assigns/checks the plans to ensure they were successful
func (p *Planner) runControlPlaneEtcdServiceStop(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, clusterPlan *plan.Plan) error {
	var joinServer string
	var err error
	if controlPlane.Spec.ETCDSnapshotRestore.S3 == nil { // In the event that we are restoring a local snapshot, we need to reset our initNode
		logrus.Infof("rkecluster %s/%s re-electing specific init node for etcd snapshot restore", controlPlane.Namespace, controlPlane.Spec.ClusterName)
		if controlPlane.Spec.ETCDSnapshotRestore.NodeName != "" {
			joinServer, err = p.designateInitNode(controlPlane, clusterPlan, controlPlane.Spec.ETCDSnapshotRestore.NodeName)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("error attempting to run etcd snapshot restore plan: either s3 info or nodename must be designated")
		}
	} else {
		joinServer, err = p.electInitNode(controlPlane, clusterPlan)
		if err != nil {
			return err
		}
	}
	servers := collect(clusterPlan, isControlPlaneEtcd)
	updated := false
	for _, server := range servers {
		var stopPlan plan.NodePlan
		var err error
		stopPlan, err = p.generateStopServiceAndKillAllPlan(controlPlane, secret, server, joinServer)
		if err != nil {
			return err
		}
		if isEtcd(server.Machine) {
			stopPlan.Instructions = append(stopPlan.Instructions, generateCreateEtcdTombstoneInstruction(controlPlane))
		}
		if server.Plan == nil || !equality.Semantic.DeepEqual(server.Plan.Plan, stopPlan) {
			if err := p.store.UpdatePlan(server.Machine, stopPlan, 0); err != nil {
				return err
			}
			updated = true
		}
	}

	// If any of the controlplane/etcd node plans were updated, return an errwaiting message for shutting down control plane and etcd
	if updated {
		return ErrWaiting("stopping " + rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion) + " services on control plane and etcd machines/nodes")
	}

	for _, server := range servers {
		if !server.Plan.InSync {
			if server.Machine.Status.NodeRef == nil {
				return ErrWaiting(fmt.Sprintf("waiting to stop %s services on machine [%s]", rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion), server.Machine.Name))
			}
			return ErrWaiting(fmt.Sprintf("waiting to stop %s services on node [%s]", rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion), server.Machine.Status.NodeRef.Name))
		}
	}

	return nil
}

// restoreEtcdSnapshot is called multiple times during an etcd snapshot restoration.
// restoreEtcdSnapshot utilizes the status of the corresponding control plane object of the cluster to track state
// The phases are in order:
// Started -> When the phase is started, it gets set to shutdown
// Shutdown -> When the phase is shutdown, it attempts to shut down etcd on all nodes (stop etcd)
// Restore ->  When the phase is restore, it attempts to restore etcd
// Finished -> When the phase is finished, Restore returns nil.
func (p *Planner) restoreEtcdSnapshot(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, clusterPlan *plan.Plan) error {
	if controlPlane.Spec.ETCDSnapshotRestore == nil {
		return p.resetEtcdSnapshotRestoreState(controlPlane)
	}

	if err := p.startOrRestartEtcdSnapshotRestore(controlPlane); err != nil {
		return err
	}

	switch controlPlane.Status.ETCDSnapshotRestorePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		return p.setEtcdSnapshotRestoreState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseShutdown)
	case rkev1.ETCDSnapshotPhaseShutdown:
		if err := p.runControlPlaneEtcdServiceStop(controlPlane, secret, clusterPlan); err != nil {
			return err
		}
		return p.setEtcdSnapshotRestoreState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseRestore)
	case rkev1.ETCDSnapshotPhaseRestore:
		if err := p.runEtcdSnapshotRestorePlan(controlPlane, secret, clusterPlan); err != nil {
			return err
		}
		controlPlane := controlPlane.DeepCopy()
		controlPlane.Status.ConfigGeneration++
		return p.setEtcdSnapshotRestoreState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseFinished)
	case rkev1.ETCDSnapshotPhaseFinished:
		return nil
	default:
		return p.setEtcdSnapshotRestoreState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseStarted)
	}
}
