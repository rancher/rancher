package planner

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/managesystemagent"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (p *Planner) setEtcdSnapshotCreateState(status rkev1.RKEControlPlaneStatus, create *rkev1.ETCDSnapshotCreate, phase rkev1.ETCDSnapshotPhase) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotCreatePhase != phase || !equality.Semantic.DeepEqual(status.ETCDSnapshotCreate, create) {
		status.ETCDSnapshotCreatePhase = phase
		status.ETCDSnapshotCreate = create
		return status, errWaiting("refreshing etcd create state")
	}
	return status, nil
}

func (p *Planner) resetEtcdSnapshotCreateState(status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotCreate == nil && status.ETCDSnapshotCreatePhase == "" {
		return status, nil
	}
	return p.setEtcdSnapshotCreateState(status, nil, "")
}

func (p *Planner) startOrRestartEtcdSnapshotCreate(status rkev1.RKEControlPlaneStatus, snapshot *rkev1.ETCDSnapshotCreate) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotCreate == nil || !equality.Semantic.DeepEqual(snapshot, status.ETCDSnapshotCreate) {
		return p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseStarted)
	}
	return status, nil
}

func (p *Planner) runEtcdSnapshotCreate(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan, joinServer string) []error {
	servers := collect(clusterPlan, isEtcd)
	if len(servers) == 0 {
		return []error{errors.New("failed to find node to perform etcd snapshot")}
	}

	var errs []error

	for _, server := range servers {
		createPlan, joinedServer, err := p.generateEtcdSnapshotCreatePlan(controlPlane, tokensSecret, server, joinServer)
		if err != nil {
			return []error{err}
		}
		msg := fmt.Sprintf("etcd snapshot on machine %s/%s", server.Machine.Namespace, server.Machine.Name)
		if server.Machine.Status.NodeRef != nil && server.Machine.Status.NodeRef.Name != "" {
			msg = fmt.Sprintf("etcd snapshot on node %s", server.Machine.Status.NodeRef.Name)
		}
		if err = assignAndCheckPlan(p.store, msg, server, createPlan, joinedServer, 3, 3); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// runEtcdSnapshotManagementServiceStart walks through the reconciliation process for the controlplane and etcd nodes.
// Notably, this function will blatantly ignore drain and concurrency options, as during an etcd snapshot operation, there is no necessity to drain nodes.
func (p *Planner) runEtcdSnapshotManagementServiceStart(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan, include roleFilter, operation string) error {
	// Generate and deliver desired plan for the bootstrap/init node first.
	if err := p.reconcile(controlPlane, tokensSecret, clusterPlan, true, bootstrapTier, isEtcd, isNotInitNodeOrIsDeleting,
		"1", "",
		controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions); err != nil {
		return err
	}

	_, joinServer, _, err := p.findInitNode(controlPlane, clusterPlan)
	if err != nil {
		return err
	}

	if joinServer == "" {
		return fmt.Errorf("error encountered restarting cluster during %s, joinServer was empty", operation)
	}

	for _, entry := range collect(clusterPlan, include) {
		if isInitNodeOrDeleting(entry) {
			continue
		}
		plan, joinedServer, err := p.desiredPlan(controlPlane, tokensSecret, entry, joinServer)
		if err != nil {
			return err
		}
		if err = assignAndCheckPlan(p.store, fmt.Sprintf("%s management plane restart", operation), entry, plan, joinedServer, 1, -1); err != nil {
			return err
		}
	}
	return nil
}

// generateEtcdSnapshotCreatePlan generates a plan that contains an instruction to create an etcd snapshot.
func (p *Planner) generateEtcdSnapshotCreatePlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, entry *planEntry, joinServer string) (plan.NodePlan, string, error) {
	v, err := semver.NewVersion(controlPlane.Spec.KubernetesVersion)
	if err != nil {
		return plan.NodePlan{}, "", err
	}

	args := []string{
		"etcd-snapshot",
	}

	// Starting in v1.26, we must specify "save" when creating an etcd snapshot
	if v.GreaterThan(managesystemagent.Kubernetes125) {
		args = append(args, "save")
	}

	createPlan, _, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, entry, joinServer, true)
	createPlan.Instructions = append(createPlan.Instructions, p.generateInstallInstructionWithSkipStart(controlPlane, entry),
		plan.OneTimeInstruction{
			Name:    "create",
			Command: capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
			Args:    args,
		})
	return createPlan, joinedServer, err
}

func (p *Planner) createEtcdSnapshot(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	var err error
	if controlPlane.Spec.ETCDSnapshotCreate == nil {
		status, err := p.resetEtcdSnapshotCreateState(status)
		return status, err
	}

	// Don't create an etcd snapshot if the cluster is not initialized or bootstrapped.
	if !status.Initialized || !capr.Bootstrapped.IsTrue(&status) {
		logrus.Warnf("[planner] rkecluster %s/%s: skipping etcd snapshot creation as cluster has not yet been initialized or bootstrapped", controlPlane.Namespace, controlPlane.Name)
		return status, nil
	}

	snapshot := controlPlane.Spec.ETCDSnapshotCreate

	if status, err = p.startOrRestartEtcdSnapshotCreate(status, snapshot); err != nil {
		return status, err
	}

	switch controlPlane.Status.ETCDSnapshotCreatePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		var stateSet bool
		var finErrs []error
		found, joinServer, _, err := p.findInitNode(controlPlane, clusterPlan)
		if err != nil {
			logrus.Errorf("[planner] rkecluster %s/%s: error encountered while searching for init node during etcd snapshot creation: %v", controlPlane.Namespace, controlPlane.Name, err)
			return status, err
		}
		if !found || joinServer == "" {
			logrus.Warnf("[planner] rkecluster %s/%s: skipping etcd snapshot creation as cluster does not have an init node", controlPlane.Namespace, controlPlane.Name)
			return status, nil
		}
		if errs := p.runEtcdSnapshotCreate(controlPlane, tokensSecret, clusterPlan, joinServer); len(errs) > 0 {
			for _, err := range errs {
				if err == nil {
					continue
				}
				finErrs = append(finErrs, err)
				if !IsErrWaiting(err) {
					// we have a failed snapshot from a node.
					if !stateSet {
						status, err = p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseFailed)
						if err != nil {
							finErrs = append(finErrs, err)
						} else {
							stateSet = true
						}
					}
				}
			}
			return status, errWaiting(merr.NewErrors(finErrs...).Error())
		}
		if status, err = p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseRestartCluster); err != nil {
			return status, err
		}
		return status, nil
	case rkev1.ETCDSnapshotPhaseRestartCluster:
		if err = p.runEtcdSnapshotManagementServiceStart(controlPlane, tokensSecret, clusterPlan, isEtcd, "etcd snapshot creation"); err != nil {
			return status, err
		}
		if status, err = p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseFinished); err != nil {
			return status, err
		}
		return status, nil
	case rkev1.ETCDSnapshotPhaseFailed:
		fallthrough
	case rkev1.ETCDSnapshotPhaseFinished:
		return status, nil
	default:
		if status, err = p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseStarted); err != nil {
			return status, err
		}
		return status, nil
	}
}
