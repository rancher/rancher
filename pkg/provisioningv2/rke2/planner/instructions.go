package planner

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	rancherruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// generateInstallInstruction generates the instruction necessary to install the desired tool.
func generateInstallInstruction(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine, env []string) plan.Instruction {
	var instruction plan.Instruction
	image := getInstallerImage(controlPlane)
	cattleOS := machine.Labels[CattleOSLabel]

	for _, arg := range controlPlane.Spec.AgentEnvVars {
		if arg.Value == "" {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", arg.Name, arg.Value))
	}

	switch cattleOS {
	case windows:
		instruction = plan.Instruction{
			Name:    "install",
			Image:   image,
			Command: "powershell.exe",
			Args:    []string{"-File", "run.ps1"},
			Env:     env,
		}
	default:
		instruction = plan.Instruction{
			Name:    "install",
			Image:   image,
			Command: "sh",
			Args:    []string{"-c", "run.sh"},
			Env:     env,
		}
	}

	if isOnlyWorker(machine) {
		switch cattleOS {
		case windows:
			instruction.Env = append(instruction.Env, fmt.Sprintf("$env:INSTALL_%s_EXEC=agent", rancherruntime.GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		default:
			instruction.Env = append(instruction.Env, fmt.Sprintf("INSTALL_%s_EXEC=agent", rancherruntime.GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		}
	}

	return instruction
}

// addInstallInstructionWithRestartStamp will generate an instruction and append it to the node plan that executes the `run.sh` or `run.ps1`
// from the installer image based on the control plane configuration. It will generate a restart stamp based on the
// passed in configuration to determine whether it needs to start/restart the service being managed.
func (p *Planner) addInstallInstructionWithRestartStamp(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	var restartStampEnv string
	stamp := restartStamp(nodePlan, controlPlane, getInstallerImage(controlPlane))
	switch machine.Labels[CattleOSLabel] {
	case windows:
		restartStampEnv = "$env:RESTART_STAMP=" + stamp
	default:
		restartStampEnv = "RESTART_STAMP=" + stamp
	}
	instEnv := []string{restartStampEnv}
	nodePlan.Instructions = append(nodePlan.Instructions, generateInstallInstruction(controlPlane, machine, instEnv))
	return nodePlan, nil
}

// generateInstallInstructionWithSkipStart will generate an instruction that executes the `run.sh` or `run.ps1`
// from the installer image based on the control plane configuration. It will add a `SKIP_START` environment variable to prevent
// the service from being started/restarted.
func (p *Planner) generateInstallInstructionWithSkipStart(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) plan.Instruction {
	var skipStartEnv string
	switch machine.Labels[CattleOSLabel] {
	case windows:
		skipStartEnv = fmt.Sprintf("$env:INSTALL_%s_SKIP_START=true", strings.ToUpper(rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)))
	default:
		skipStartEnv = fmt.Sprintf("INSTALL_%s_SKIP_START=true", strings.ToUpper(rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)))
	}
	instEnv := []string{skipStartEnv}
	return generateInstallInstruction(controlPlane, machine, instEnv)
}

func (p *Planner) addInitNodeInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	nodePlan.Instructions = append(nodePlan.Instructions, plan.Instruction{
		Name:       "capture-address",
		Command:    "sh",
		SaveOutput: true,
		Args: []string{
			"-c",
			// the grep here is to make the command fail if we don't get the output we expect, like empty string.
			fmt.Sprintf("curl -f --retry 100 --retry-delay 5 --cacert "+
				"/var/lib/rancher/%s/server/tls/server-ca.crt https://localhost:%d/db/info | grep 'clientURLs'",
				rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion),
				rancherruntime.GetRuntimeSupervisorPort(controlPlane.Spec.KubernetesVersion)),
		},
	})
	return nodePlan, nil
}
