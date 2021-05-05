package planner

import (
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
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

func (e *etcdCreate) etcdCreate(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan, snapshot *rkev1.ETCDSnapshotCreate) error {
	servers := collect(clusterPlan, func(machine *capi.Machine) bool {
		if !isEtcd(machine) || machine.Status.NodeRef == nil {
			return false
		}
		return snapshot.NodeName == "" ||
			machine.Status.NodeRef.Name == snapshot.NodeName
	})

	if len(servers) == 0 {
		return fmt.Errorf("failed to find node to perform etcd snapshot")
	}

	server := servers[0]
	createPlan, err := e.createPlan(controlPlane, snapshot, server.Machine.Status.NodeRef.Name)
	if err != nil {
		return err
	}

	return assignAndCheckPlan(e.store, "etcd snapshot", server, createPlan)
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
			Image:   getInstallerImage(controlPlane),
			Command: GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
			Env:     s3Env,
			Args:    append(args, s3Args...),
		}},
	})
}

func (e *etcdCreate) Create(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) error {
	if controlPlane.Spec.ETCDSnapshotCreate == nil {
		return e.resetEtcdCreateState(controlPlane)
	}

	return e.createSnapshot(controlPlane, clusterPlan, controlPlane.Spec.ETCDSnapshotCreate)
}

func (e *etcdCreate) createSnapshot(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan, snapshot *rkev1.ETCDSnapshotCreate) error {
	if err := e.startOrRestartCreate(controlPlane, snapshot); err != nil {
		return err
	}

	switch controlPlane.Status.ETCDSnapshotCreatePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		if err := e.etcdCreate(controlPlane, clusterPlan, snapshot); err != nil {
			return err
		}
		return e.setState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseFinished)
	case rkev1.ETCDSnapshotPhaseFinished:
		return nil
	default:
		return e.setState(controlPlane, snapshot, rkev1.ETCDSnapshotPhaseStarted)
	}
}
