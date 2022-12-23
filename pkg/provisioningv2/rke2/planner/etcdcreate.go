package planner

import (
	"errors"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (p *Planner) setEtcdSnapshotCreateState(status rkev1.RKEControlPlaneStatus, spec *rkev1.ETCDSnapshotCreate, phase rkev1.ETCDSnapshotPhase) (rkev1.RKEControlPlaneStatus, error) {
	status.ETCDSnapshotCreatePhase = phase
	status.ETCDSnapshotCreate = spec
	return status, ErrWaiting("refreshing etcd create state")
}

func (p *Planner) resetEtcdSnapshotCreateState(status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotCreate == nil && status.ETCDSnapshotCreatePhase == "" {
		return status, nil
	}
	return p.setEtcdSnapshotCreateState(status, nil, "")
}

func (p *Planner) startOrRestartEtcdSnapshotCreate(status rkev1.RKEControlPlaneStatus, snapshot *rkev1.ETCDSnapshotCreate) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotCreate == nil || !equality.Semantic.DeepEqual(*snapshot, *status.ETCDSnapshotCreate) {
		return p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseStarted)
	}
	return status, nil
}

func (p *Planner) runEtcdSnapshotCreate(cp *rkev1.RKEControlPlane, clusterPlan *plan.Plan) []error {
	servers := collect(clusterPlan, isEtcd)
	if len(servers) == 0 {
		return []error{errors.New("failed to find node to perform etcd snapshot")}
	}

	var errs []error

	for _, server := range servers {
		createPlan, err := p.generateEtcdSnapshotCreatePlan(cp, server)
		if err != nil {
			return []error{err}
		}
		msg := fmt.Sprintf("etcd snapshot on machine %s/%s", server.Machine.Namespace, server.Machine.Name)
		if server.Machine.Status.NodeRef != nil && server.Machine.Status.NodeRef.Name != "" {
			msg = fmt.Sprintf("etcd snapshot on node %s", server.Machine.Status.NodeRef.Name)
		}
		if err := assignAndCheckPlan(p.store, msg, server, createPlan, 3, 3); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (p *Planner) generateEtcdSnapshotCreatePlan(cp *rkev1.RKEControlPlane, entry *planEntry) (plan.NodePlan, error) {
	args := []string{
		"etcd-snapshot",
	}

	return p.commonNodePlan(cp, plan.NodePlan{
		Instructions: []plan.OneTimeInstruction{
			p.generateInstallInstructionWithSkipStart(cp, entry),
			{
				Name:    "create",
				Command: rke2.GetRuntimeCommand(cp.Spec.KubernetesVersion),
				Args:    args,
			}},
	})
}

func (p *Planner) createEtcdSnapshot(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, []error) {
	if cp.Spec.ETCDSnapshotCreate == nil {
		if status, err := p.resetEtcdSnapshotCreateState(status); err != nil {
			return status, []error{err}
		}
		return status, nil
	}

	if err := p.pauseCAPICluster(cp, true); err != nil {
		return status, []error{err}
	}

	snapshot := cp.Spec.ETCDSnapshotCreate

	if status, err := p.startOrRestartEtcdSnapshotCreate(status, snapshot); err != nil {
		return status, []error{err}
	}

	switch cp.Status.ETCDSnapshotCreatePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		var stateSet bool
		var finErrs []error
		if errs := p.runEtcdSnapshotCreate(cp, clusterPlan); len(errs) > 0 {
			for _, err := range errs {
				if err == nil {
					continue
				}
				finErrs = append(finErrs, err)
				var errWaiting ErrWaiting
				if !errors.As(err, &errWaiting) {
					// we have a failed snapshot from a node.
					if !stateSet {
						var err error
						if status, err = p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseFailed); err != nil {
							finErrs = append(finErrs, err)
						} else {
							stateSet = true
						}
					}
				}
			}
			return status, finErrs
		}
		if status, err := p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseRestartCluster); err != nil {
			return status, []error{err}
		}
		return status, nil
	case rkev1.ETCDSnapshotPhaseRestartCluster:
		if err := p.runEtcdSnapshotManagementServiceStart(cp, tokensSecret, clusterPlan, isEtcd, "etcd snapshot creation"); err != nil {
			return status, []error{err}
		}
		if status, err := p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseFinished); err != nil {
			return status, []error{err}
		}
		return status, nil
	case rkev1.ETCDSnapshotPhaseFailed:
		fallthrough
	case rkev1.ETCDSnapshotPhaseFinished:
		if err := p.pauseCAPICluster(cp, false); err != nil {
			return status, []error{err}
		}
		return status, nil
	default:
		if status, err := p.setEtcdSnapshotCreateState(status, snapshot, rkev1.ETCDSnapshotPhaseStarted); err != nil {
			return status, []error{err}
		}
		return status, nil
	}
}
