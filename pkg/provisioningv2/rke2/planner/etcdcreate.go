package planner

import (
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

type etcdCreate struct {
	controlPlane rkecontroller.RKEControlPlaneClient
	secrets      corecontrollers.SecretCache
	store        *PlanStore
	s3Args       *s3Args
}

func newETCDCreate(clients *wrangler.Context, store *PlanStore) *etcdCreate {
	return &etcdCreate{
		controlPlane: clients.RKE.RKEControlPlane(),
		secrets:      clients.Core.Secret().Cache(),
		store:        store,
		s3Args: &s3Args{
			secretCache: clients.Core.Secret().Cache(),
			env:         true,
		},
	}
}

func (e *etcdCreate) setState(controlPlane *rkev1.RKEControlPlane, spec *rkev1.ETCDSnapshotCreate, phase rkev1.ETCDSnapshotPhase) error {
	controlPlane = controlPlane.DeepCopy()
	controlPlane.Status.ETCDSnapshotCreatePhase = phase
	controlPlane.Status.ETCDSnapshotCreate = spec
	_, err := e.controlPlane.UpdateStatus(controlPlane)
	if err != nil {
		return err
	}
	return ErrWaiting("refreshing etcd create state")
}

func (e *etcdCreate) resetEtcdCreateState(controlPlane *rkev1.RKEControlPlane) error {
	if controlPlane.Status.ETCDSnapshotCreate == nil && controlPlane.Status.ETCDSnapshotCreatePhase == "" {
		return nil
	}
	return e.setState(controlPlane, nil, "")
}

func (e *etcdCreate) startOrRestartCreate(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshotCreate) error {
	if controlPlane.Status.ETCDSnapshotCreate == nil || !equality.Semantic.DeepEqual(*snapshot, *controlPlane.Status.ETCDSnapshotCreate) {
		return e.setState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseStarted)
	}
	return nil
}

func (e *etcdCreate) etcdCreate(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan, snapshot *rkev1.ETCDSnapshotCreate) []error {
	servers := collect(clusterPlan, func(machine *capi.Machine) bool {
		if !isEtcd(machine) || machine.Status.NodeRef == nil {
			return false
		}
		return snapshot.NodeName == "" ||
			machine.Status.NodeRef.Name == snapshot.NodeName
	})

	if len(servers) == 0 {
		return []error{errors.New("failed to find node to perform etcd snapshot")}
	}

	var errs []error

	for _, server := range servers {
		createPlan, err := e.createPlan(controlPlane, snapshot, server.Machine.Status.NodeRef.Name)
		if err != nil {
			return []error{err}
		}
		if err := assignAndCheckPlan(e.store, "etcd snapshot", server, createPlan, 3); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (e *etcdCreate) createPlan(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshotCreate, nodeName string) (plan.NodePlan, error) {
	args := []string{
		"etcd-snapshot",
	}

	if snapshot.Name != "" {
		args = append(args, fmt.Sprintf("--name=%s", snapshot.Name))
	}
	if nodeName != "" {
		args = append(args, fmt.Sprintf("--node-name=%s", nodeName))
	}

	s3Args, s3Env, s3Files, err := e.s3Args.ToArgs(snapshot.S3, controlPlane)
	if err != nil {
		return plan.NodePlan{}, err
	}

	return commonNodePlan(e.secrets, controlPlane, plan.NodePlan{
		Files: s3Files,
		Instructions: []plan.Instruction{{
			Name:    "create",
			Command: GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
			Env:     s3Env,
			Args:    append(args, s3Args...),
		}},
	})
}

func (e *etcdCreate) Create(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) []error {
	if !Provisioned.IsTrue(controlPlane) && controlPlane.Status.ETCDSnapshotCreatePhase == "" {
		return nil
	}

	if controlPlane.Spec.ETCDSnapshotCreate == nil {
		if err := e.resetEtcdCreateState(controlPlane); err != nil {
			return []error{err}
		}
		return nil
	}

	return e.createSnapshot(controlPlane, clusterPlan, controlPlane.Spec.ETCDSnapshotCreate)
}

func (e *etcdCreate) createSnapshot(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan, snapshot *rkev1.ETCDSnapshotCreate) []error {
	if err := e.startOrRestartCreate(controlPlane, snapshot); err != nil {
		return []error{err}
	}

	switch controlPlane.Status.ETCDSnapshotCreatePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		var stateSet bool
		var finErrs []error
		if errs := e.etcdCreate(controlPlane, clusterPlan, snapshot); len(errs) > 0 {
			for _, err := range errs {
				if err == nil {
					continue
				}
				finErrs = append(finErrs, err)
				var errWaiting ErrWaiting
				if !errors.As(err, &errWaiting) {
					// we have a failed snapshot from a node.
					if !stateSet {
						if err := e.setState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseFailed); err != nil {
							finErrs = append(finErrs, err)
						} else {
							stateSet = true
						}
					}
				}
			}
			return finErrs
		}
		if err := e.setState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseFinished); err != nil {
			return []error{err}
		}
		return nil
	case rkev1.ETCDSnapshotPhaseFailed:
		fallthrough
	case rkev1.ETCDSnapshotPhaseFinished:
		return nil
	default:
		if err := e.setState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseStarted); err != nil {
			return []error{err}
		}
		return nil
	}
}
