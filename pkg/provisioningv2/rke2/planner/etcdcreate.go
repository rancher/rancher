package planner

import (
	"errors"
	"fmt"

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
	servers := collect(clusterPlan, func(entry *planEntry) bool {
		if !isEtcd(entry) || entry.Machine.Status.NodeRef == nil {
			return false
		}
		return snapshot.NodeName == "" ||
			entry.Machine.Status.NodeRef.Name == snapshot.NodeName
	})

	if len(servers) == 0 {
		return []error{errors.New("failed to find node to perform etcd snapshot")}
	}

	var errs []error

	for _, server := range servers {
		createPlan, err := p.generateEtcdSnapshotCreatePlan(controlPlane, snapshot, server.Machine.Status.NodeRef.Name)
		if err != nil {
			return []error{err}
		}
		if err := assignAndCheckPlan(p.store, "etcd snapshot", server, createPlan, 3); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (p *Planner) generateEtcdSnapshotCreatePlan(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshotCreate, nodeName string) (plan.NodePlan, error) {
	args := []string{
		"etcd-snapshot",
	}

	if snapshot.Name != "" {
		args = append(args, fmt.Sprintf("--name=%s", snapshot.Name))
	}
	if nodeName != "" {
		args = append(args, fmt.Sprintf("--node-name=%s", nodeName))
	}

	s3Args, s3Env, s3Files, err := p.etcdS3Args.ToArgs(snapshot.S3, controlPlane)
	if err != nil {
		return plan.NodePlan{}, err
	}

	return p.commonNodePlan(controlPlane, plan.NodePlan{
		Files: s3Files,
		Instructions: []plan.Instruction{{
			Name:    "create",
			Command: rke2.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
			Env:     s3Env,
			Args:    append(args, s3Args...),
		}},
	})
}

func (p *Planner) createEtcdSnapshot(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) []error {
	if !rke2.Provisioned.IsTrue(controlPlane) && controlPlane.Status.ETCDSnapshotCreatePhase == "" {
		return nil
	}

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
