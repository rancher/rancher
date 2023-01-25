package planner

import (
	"errors"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/wrangler/pkg/merr"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (p *Planner) setEtcdSnapshotCreateState(status rkev1.RKEControlPlaneStatus, create *rkev1.ETCDSnapshotCreate, phase rkev1.ETCDSnapshotPhase) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotCreatePhase != phase || !equality.Semantic.DeepEqual(status.ETCDSnapshotCreate, create) {
		status.ETCDSnapshotCreatePhase = phase
		status.ETCDSnapshotCreate = create
		return status, ErrWaiting("refreshing etcd create state")
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

func (p *Planner) runEtcdSnapshotCreate(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) []error {
	servers := collect(clusterPlan, isEtcd)
	if len(servers) == 0 {
		return []error{errors.New("failed to find node to perform etcd snapshot")}
	}

	var errs []error

	for _, server := range servers {
		createPlan, err := p.generateEtcdSnapshotCreatePlan(controlPlane, server)
		if err != nil {
			return []error{err}
		}
		msg := fmt.Sprintf("etcd snapshot on machine %s/%s", server.Machine.Namespace, server.Machine.Name)
		if server.Machine.Status.NodeRef != nil && server.Machine.Status.NodeRef.Name != "" {
			msg = fmt.Sprintf("etcd snapshot on node %s", server.Machine.Status.NodeRef.Name)
		}
		if err = assignAndCheckPlan(p.store, msg, server, createPlan, 3, 3); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (p *Planner) generateEtcdSnapshotCreatePlan(controlPlane *rkev1.RKEControlPlane, entry *planEntry) (plan.NodePlan, error) {
	args := []string{
		"etcd-snapshot",
	}

	return p.commonNodePlan(controlPlane, plan.NodePlan{
		Instructions: []plan.OneTimeInstruction{
			p.generateInstallInstructionWithSkipStart(controlPlane, entry),
			{
				Name:    "create",
				Command: rke2.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
				Args:    args,
			}},
	})
}

func (p *Planner) createEtcdSnapshot(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	var err error
	if controlPlane.Spec.ETCDSnapshotCreate == nil {
		status, err := p.resetEtcdSnapshotCreateState(status)
		return status, err
	}

	snapshot := controlPlane.Spec.ETCDSnapshotCreate

	if status, err = p.startOrRestartEtcdSnapshotCreate(status, snapshot); err != nil {
		return status, err
	}

	switch controlPlane.Status.ETCDSnapshotCreatePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		var stateSet bool
		var finErrs []error
		if errs := p.runEtcdSnapshotCreate(controlPlane, clusterPlan); len(errs) > 0 {
			for _, err := range errs {
				if err == nil {
					continue
				}
				finErrs = append(finErrs, err)
				if IsErrWaiting(err) {
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
			return status, ErrWaiting(merr.NewErrors(finErrs...).Error())
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
