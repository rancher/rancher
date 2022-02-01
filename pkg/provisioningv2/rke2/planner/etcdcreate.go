package planner

import (
	"errors"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (p *Planner) setEtcdSnapshotCreateState(controlPlane *rkev1.RKEControlPlane, spec *rkev1.ETCDSnapshotCreate, phase rkev1.ETCDSnapshotPhase) error {
	controlPlane = controlPlane.DeepCopy()
	controlPlane.Status.ETCDSnapshotCreatePhase = phase
	controlPlane.Status.ETCDSnapshotCreate = spec
	_, err := p.rkeControlPlanes.UpdateStatus(controlPlane)
	if err != nil {
		return err
	}
	return ErrWaiting("refreshing etcd create state")
}

func (p *Planner) resetEtcdSnapshotCreateState(controlPlane *rkev1.RKEControlPlane) error {
	if controlPlane.Status.ETCDSnapshotCreate == nil && controlPlane.Status.ETCDSnapshotCreatePhase == "" {
		return nil
	}
	return p.setEtcdSnapshotCreateState(controlPlane, nil, "")
}

func (p *Planner) startOrRestartEtcdSnapshotCreate(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshotCreate) error {
	if controlPlane.Status.ETCDSnapshotCreate == nil || !equality.Semantic.DeepEqual(*snapshot, *controlPlane.Status.ETCDSnapshotCreate) {
		return p.setEtcdSnapshotCreateState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseStarted)
	}
	return nil
}

func (p *Planner) runEtcdSnapshotCreate(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan, snapshot *rkev1.ETCDSnapshotCreate) []error {
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
		if err := assignAndCheckPlan(p.store, "etcd snapshot", server, createPlan, 3); err != nil {
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

func (p *Planner) createEtcdSnapshot(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) []error {
	if controlPlane.Spec.ETCDSnapshotCreate == nil {
		if err := p.resetEtcdSnapshotCreateState(controlPlane); err != nil {
			return []error{err}
		}
		return nil
	}

	snapshot := controlPlane.Spec.ETCDSnapshotCreate

	if err := p.startOrRestartEtcdSnapshotCreate(controlPlane, snapshot); err != nil {
		return []error{err}
	}

	switch controlPlane.Status.ETCDSnapshotCreatePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		var stateSet bool
		var finErrs []error
		if errs := p.runEtcdSnapshotCreate(controlPlane, clusterPlan, snapshot); len(errs) > 0 {
			for _, err := range errs {
				if err == nil {
					continue
				}
				finErrs = append(finErrs, err)
				var errWaiting ErrWaiting
				if !errors.As(err, &errWaiting) {
					// we have a failed snapshot from a node.
					if !stateSet {
						if err := p.setEtcdSnapshotCreateState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseFailed); err != nil {
							finErrs = append(finErrs, err)
						} else {
							stateSet = true
						}
					}
				}
			}
			return finErrs
		}
		if err := p.setEtcdSnapshotCreateState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseFinished); err != nil {
			return []error{err}
		}
		return nil
	case rkev1.ETCDSnapshotPhaseFailed:
		fallthrough
	case rkev1.ETCDSnapshotPhaseFinished:
		return nil
	default:
		if err := p.setEtcdSnapshotCreateState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseStarted); err != nil {
			return []error{err}
		}
		return nil
	}
}
