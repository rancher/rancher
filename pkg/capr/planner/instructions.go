package planner

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
)

const (
	captureAddressInstructionName = "capture-address"
	etcdNameInstructionName       = "etcd-name"
)

// generateInstallInstruction generates the instruction necessary to install the desired tool.
func (p *Planner) generateInstallInstruction(controlPlane *rkev1.RKEControlPlane, entry *planEntry, env []string) plan.OneTimeInstruction {
	var instruction plan.OneTimeInstruction
	image := p.getInstallerImage(controlPlane)
	cattleOS := entry.Metadata.Labels[capr.CattleOSLabel]
	for _, arg := range controlPlane.Spec.AgentEnvVars {
		if arg.Value == "" {
			continue
		}
		switch cattleOS {
		case capr.WindowsMachineOS:
			env = append(env, fmt.Sprintf("$env:%s=\"%s\"", arg.Name, arg.Value))
		default:
			env = append(env, fmt.Sprintf("%s=%s", arg.Name, arg.Value))
		}
	}

	switch cattleOS {
	case capr.WindowsMachineOS:
		instruction = plan.OneTimeInstruction{
			Name:    "install",
			Image:   image,
			Command: "powershell.exe",
			Args:    []string{"-File", "run.ps1"},
			Env:     env,
		}
	default:
		instruction = plan.OneTimeInstruction{
			Name:    "install",
			Image:   image,
			Command: "sh",
			Args:    []string{"-c", "run.sh"},
			Env:     env,
		}
	}

	if isOnlyWorker(entry) {
		switch cattleOS {
		case capr.WindowsMachineOS:
			instruction.Env = append(instruction.Env, fmt.Sprintf("$env:INSTALL_%s_EXEC=\"agent\"", capr.GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		default:
			instruction.Env = append(instruction.Env, fmt.Sprintf("INSTALL_%s_EXEC=agent", capr.GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		}
	}

	return instruction
}

// addInstallInstructionWithRestartStamp will generate an instruction and append it to the node plan that executes the `run.sh` or `run.ps1`
// from the installer image based on the control plane configuration. It will generate a restart stamp based on the
// passed in configuration to determine whether it needs to start/restart the service being managed.
func (p *Planner) addInstallInstructionWithRestartStamp(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) (plan.NodePlan, error) {
	var restartStampEnv string
	stamp := restartStamp(nodePlan, controlPlane, p.getInstallerImage(controlPlane))
	switch entry.Metadata.Labels[capr.CattleOSLabel] {
	case capr.WindowsMachineOS:
		restartStampEnv = "$env:RESTART_STAMP=\"" + stamp + "\""
	default:
		restartStampEnv = "RESTART_STAMP=" + stamp
	}
	instEnv := []string{restartStampEnv}
	nodePlan.Instructions = append(nodePlan.Instructions, p.generateInstallInstruction(controlPlane, entry, instEnv))
	return nodePlan, nil
}

// generateInstallInstructionWithSkipStart will generate an instruction that executes the `run.sh` or `run.ps1`
// from the installer image based on the control plane configuration. It will add a `SKIP_START` environment variable to prevent
// the service from being started/restarted.
func (p *Planner) generateInstallInstructionWithSkipStart(controlPlane *rkev1.RKEControlPlane, entry *planEntry) plan.OneTimeInstruction {
	var skipStartEnv string
	switch entry.Metadata.Labels[capr.CattleOSLabel] {
	case capr.WindowsMachineOS:
		skipStartEnv = fmt.Sprintf("$env:INSTALL_%s_SKIP_START=\"true\"", strings.ToUpper(capr.GetRuntime(controlPlane.Spec.KubernetesVersion)))
	default:
		skipStartEnv = fmt.Sprintf("INSTALL_%s_SKIP_START=true", strings.ToUpper(capr.GetRuntime(controlPlane.Spec.KubernetesVersion)))
	}
	instEnv := []string{skipStartEnv}
	return p.generateInstallInstruction(controlPlane, entry, instEnv)
}

func (p *Planner) addInitNodePeriodicInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane) (plan.NodePlan, error) {
	nodePlan.PeriodicInstructions = append(nodePlan.PeriodicInstructions, []plan.PeriodicInstruction{
		{
			Name:    captureAddressInstructionName,
			Command: "sh",
			Args: []string{
				"-c",
				// the grep here is to make the command fail if we don't get the output we expect, like empty string.
				fmt.Sprintf("curl -f --retry 100 --retry-delay 5 --cacert "+
					"/var/lib/rancher/%s/server/tls/server-ca.crt https://localhost:%d/db/info | grep 'clientURLs'",
					capr.GetRuntime(controlPlane.Spec.KubernetesVersion),
					capr.GetRuntimeSupervisorPort(controlPlane.Spec.KubernetesVersion)),
			},
			PeriodSeconds: 600,
		},
		{
			Name:    etcdNameInstructionName,
			Command: "sh",
			Args: []string{
				"-c",
				fmt.Sprintf("cat /var/lib/rancher/%s/server/db/etcd/name", capr.GetRuntime(controlPlane.Spec.KubernetesVersion)),
			},
			PeriodSeconds: 600,
		},
	}...)
	return nodePlan, nil
}

func (p *Planner) addEtcdSnapshotListLocalPeriodicInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane) (plan.NodePlan, error) {
	nodePlan.PeriodicInstructions = append(nodePlan.PeriodicInstructions, plan.PeriodicInstruction{
		Name:    "etcd-snapshot-list-local",
		Command: "sh",
		Args: []string{
			"-c",
			// the grep here is to make the command fail if we don't get the output we expect, like empty string.
			fmt.Sprintf("%s etcd-snapshot list --etcd-s3=false 2>/dev/null",
				capr.GetRuntime(controlPlane.Spec.KubernetesVersion)),
		},
		PeriodSeconds: 600,
	})
	return nodePlan, nil
}

func (p *Planner) addEtcdSnapshotListS3PeriodicInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane) (plan.NodePlan, error) {
	nodePlan.PeriodicInstructions = append(nodePlan.PeriodicInstructions, plan.PeriodicInstruction{
		Name:    "etcd-snapshot-list-s3",
		Command: "sh",
		Args: []string{
			"-c",
			// the grep here is to make the command fail if we don't get the output we expect, like empty string.
			fmt.Sprintf("%s etcd-snapshot list --etcd-s3 2>/dev/null",
				capr.GetRuntime(controlPlane.Spec.KubernetesVersion)),
		},
		PeriodSeconds: 600,
	})
	return nodePlan, nil
}
