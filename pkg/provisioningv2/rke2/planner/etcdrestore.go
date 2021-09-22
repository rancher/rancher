package planner

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

type etcdRestore struct {
	controlPlane rkecontroller.RKEControlPlaneClient
	secrets      corecontrollers.SecretCache
	s3Args       *s3Args
	store        *PlanStore
}

func newETCDRestore(clients *wrangler.Context, store *PlanStore) *etcdRestore {
	return &etcdRestore{
		controlPlane: clients.RKE.RKEControlPlane(),
		secrets:      clients.Core.Secret().Cache(),
		store:        store,
		s3Args: &s3Args{
			secretCache: clients.Core.Secret().Cache(),
			prefix:      "etcd-",
			env:         true,
		},
	}
}

func (e *etcdRestore) setState(controlPlane *rkev1.RKEControlPlane, status *rkev1.ETCDSnapshotRestore, phase rkev1.ETCDSnapshotPhase) error {
	controlPlane = controlPlane.DeepCopy()
	controlPlane.Status.ETCDSnapshotRestorePhase = phase
	controlPlane.Status.ETCDSnapshotRestore = status
	_, err := e.controlPlane.UpdateStatus(controlPlane)
	if err != nil {
		return err
	}
	return ErrWaiting("refreshing etcd restore state")
}

func (e *etcdRestore) resetEtcdRestoreState(controlPlane *rkev1.RKEControlPlane) error {
	if controlPlane.Status.ETCDSnapshotRestore == nil && controlPlane.Status.ETCDSnapshotRestorePhase == "" {
		return nil
	}
	return e.setState(controlPlane, nil, "")
}

func (e *etcdRestore) startOrRestartRestore(controlPlane *rkev1.RKEControlPlane) error {
	if controlPlane.Status.ETCDSnapshotRestore == nil || !equality.Semantic.DeepEqual(*controlPlane.Spec.ETCDSnapshotRestore, *controlPlane.Status.ETCDSnapshotRestore) {
		return e.setState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseStarted)
	}
	return nil
}

func (e *etcdRestore) etcdRestore(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) error {
	servers := collect(clusterPlan, isEtcd)

	for _, server := range servers {
		if controlPlane.Spec.ETCDSnapshotRestore.S3 != nil ||
			(server.Machine.Status.NodeRef != nil &&
				server.Machine.Status.NodeRef.Name == controlPlane.Spec.ETCDSnapshotRestore.NodeName) {
			restorePlan, err := e.restorePlan(controlPlane, controlPlane.Spec.ETCDSnapshotRestore)
			if err != nil {
				return err
			}
			return assignAndCheckPlan(e.store, "etcd restore", server, restorePlan, 0)
		}
	}

	return ErrWaiting("failed to find etcd node to restore on")
}

func ensureInstalledInstruction(controlPlane *rkev1.RKEControlPlane) plan.Instruction {
	return plan.Instruction{
		Name:  "install",
		Image: getInstallerImage(controlPlane),
		Env: []string{
			fmt.Sprintf("INSTALL_%s_SKIP_START=true", strings.ToUpper(runtime.GetRuntime(controlPlane.Spec.KubernetesVersion))),
		},
	}
}

func (e *etcdRestore) restorePlan(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshotRestore) (plan.NodePlan, error) {
	args := []string{
		"server",
		"--cluster-reset",
	}

	if snapshot.S3 == nil {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshot.Name))
	} else {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=%s", snapshot.Name))
	}

	s3Args, s3Env, s3Files, err := e.s3Args.ToArgs(snapshot.S3, controlPlane)
	if err != nil {
		return plan.NodePlan{}, err
	}

	stopPlan, err := e.stopEtcdPlan(controlPlane)
	if err != nil {
		return plan.NodePlan{}, err
	}

	planInstructions := append(stopPlan.Instructions, plan.Instruction{
		Name:    "remove-tombstone",
		Command: "rm",
		Args: []string{
			"-f",
			fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd/tombstone", GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
		},
	})

	nodePlan := plan.NodePlan{
		Files: s3Files,
		Instructions: append(planInstructions, plan.Instruction{
			Name:    "restore",
			Env:     s3Env,
			Args:    append(args, s3Args...),
			Command: GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
		}),
	}

	return commonNodePlan(e.secrets, controlPlane, nodePlan)
}

func (e *etcdRestore) stopControlPlanePlan(controlPlane *rkev1.RKEControlPlane) (plan.NodePlan, error) {
	nodePlan := plan.NodePlan{
		Instructions: []plan.Instruction{
			ensureInstalledInstruction(controlPlane),
			{
				Name:    "shutdown",
				Command: "systemctl",
				Args: []string{
					"stop", GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion),
				},
			},
			{
				Name:    "shutdown",
				Command: GetRuntimeCommand(controlPlane.Spec.KubernetesVersion) + "-killall.sh",
			},
		},
	}
	return commonNodePlan(e.secrets, controlPlane, nodePlan)
}

func (e *etcdRestore) stopEtcdPlan(controlPlane *rkev1.RKEControlPlane) (plan.NodePlan, error) {
	nodePlan := plan.NodePlan{
		Instructions: []plan.Instruction{
			ensureInstalledInstruction(controlPlane),
			{
				Name:    "shutdown",
				Command: "systemctl",
				Args: []string{
					"stop", GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion),
				},
			},
			{
				Name:    "shutdown",
				Command: fmt.Sprintf("%s-killall.sh", GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
			},
			{
				Name:    "tombstone",
				Command: "touch",
				Args: []string{
					fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd/tombstone", GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
				},
			},
		},
	}
	return commonNodePlan(e.secrets, controlPlane, nodePlan)
}

func (e *etcdRestore) etcdShutdown(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) error {
	servers := collect(clusterPlan, isControlPlaneEtcd)

	stopControlPlanePlan, err := e.stopControlPlanePlan(controlPlane)
	if err != nil {
		return err
	}

	stopEtcdPlan, err := e.stopEtcdPlan(controlPlane)
	if err != nil {
		return err
	}

	updated := false
	for _, server := range servers {
		stopPlan := stopControlPlanePlan
		if isEtcd(server.Machine) {
			stopPlan = stopEtcdPlan
		}
		if server.Plan == nil || !equality.Semantic.DeepEqual(server.Plan.Plan, stopPlan) {
			if err := e.store.UpdatePlan(server.Machine, stopPlan, 0); err != nil {
				return err
			}
			updated = true
		}
	}

	if updated {
		return ErrWaiting("shutting down control plane and etcd")
	}

	for _, server := range servers {
		if !server.Plan.InSync {
			if server.Machine.Status.NodeRef == nil {
				return ErrWaiting(fmt.Sprintf("waiting to shutdown down control plane machine [%s]", server.Machine.Name))
			}
			return ErrWaiting(fmt.Sprintf("waiting to shutdown down control plane node [%s]", server.Machine.Status.NodeRef.Name))
		}
	}

	return nil
}

func (e *etcdRestore) Restore(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) error {
	if controlPlane.Spec.ETCDSnapshotRestore == nil {
		return e.resetEtcdRestoreState(controlPlane)
	}

	if err := e.startOrRestartRestore(controlPlane); err != nil {
		return err
	}

	switch controlPlane.Status.ETCDSnapshotRestorePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		return e.setState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseShutdown)
	case rkev1.ETCDSnapshotPhaseShutdown:
		if err := e.etcdShutdown(controlPlane, clusterPlan); err != nil {
			return err
		}
		return e.setState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseRestore)
	case rkev1.ETCDSnapshotPhaseRestore:
		if err := e.etcdRestore(controlPlane, clusterPlan); err != nil {
			return err
		}
		controlPlane := controlPlane.DeepCopy()
		controlPlane.Status.ConfigGeneration++
		return e.setState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseFinished)
	case rkev1.ETCDSnapshotPhaseFinished:
		return nil
	default:
		return e.setState(controlPlane, controlPlane.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseStarted)
	}
}
