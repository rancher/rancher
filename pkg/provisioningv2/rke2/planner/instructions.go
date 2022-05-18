package planner

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
)

const (
	captureAddressInstructionName = "capture-address"
	etcdNameInstructionName       = "etcd-name"
)

// generateInstallInstruction generates the instruction necessary to install the desired tool.
func generateInstallInstruction(controlPlane *rkev1.RKEControlPlane, entry *planEntry, env []string) plan.OneTimeInstruction {
	var instruction plan.OneTimeInstruction
	image := getInstallerImage(controlPlane)
	cattleOS := entry.Metadata.Labels[rke2.CattleOSLabel]
	for _, arg := range controlPlane.Spec.AgentEnvVars {
		if arg.Value == "" {
			continue
		}
		switch cattleOS {
		case windows:
			env = append(env, fmt.Sprintf("$env:%s=\"%s\"", arg.Name, arg.Value))
		default:
			env = append(env, fmt.Sprintf("%s=%s", arg.Name, arg.Value))
		}
	}

	switch cattleOS {
	case windows:
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
		case windows:
			instruction.Env = append(instruction.Env, fmt.Sprintf("$env:INSTALL_%s_EXEC=\"agent\"", rke2.GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		default:
			instruction.Env = append(instruction.Env, fmt.Sprintf("INSTALL_%s_EXEC=agent", rke2.GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		}
	}

	return instruction
}

// addInstallInstructionWithRestartStamp will generate an instruction and append it to the node plan that executes the `run.sh` or `run.ps1`
// from the installer image based on the control plane configuration. It will generate a restart stamp based on the
// passed in configuration to determine whether it needs to start/restart the service being managed.
func (p *Planner) addInstallInstructionWithRestartStamp(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) plan.NodePlan {
	var restartStampEnv string
	stamp := restartStamp(nodePlan, controlPlane, getInstallerImage(controlPlane))
	switch entry.Metadata.Labels[rke2.CattleOSLabel] {
	case windows:
		restartStampEnv = "$env:RESTART_STAMP=\"" + stamp + "\""
	default:
		restartStampEnv = "RESTART_STAMP=" + stamp
	}
	instEnv := []string{restartStampEnv}
	nodePlan.Instructions = append(nodePlan.Instructions, generateInstallInstruction(controlPlane, entry, instEnv))
	return nodePlan
}

// addApplyClusterAgentPeriodicInstruction removes the cattle-cluster-agent manifest, if it exists.
// After it is installed once, the cattle-cluster-agent should be managed by Rancher.
func (p *Planner) addApplyClusterAgentPeriodicInstruction(controlPlane *rkev1.RKEControlPlane, nodePlan plan.NodePlan, entry *planEntry) (plan.NodePlan, error) {
	if !controlPlane.Status.AgentDeployed && isControlPlane(entry) {
		runtime := rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)
		var kubectlCmd string
		if runtime == rke2.RuntimeK3S {
			kubectlCmd = "k3s kubectl"
		} else {
			kubectlCmd = "/var/lib/rancher/rke2/bin/kubectl"
		}

		manifest, err := p.generateClusterAgentManifest(controlPlane, entry)
		if err != nil {
			return nodePlan, err
		}

		nodePlan.PeriodicInstructions = append(nodePlan.PeriodicInstructions, plan.PeriodicInstruction{
			Name:    "apply-cluster-agent-manifest",
			Command: "sh",
			Args: []string{
				"-c",
				// We need to check for the deployment to ensure that this command doesn't overwrite the deployment when the Rancher controllers take over.
				// We have to do the printf "%%b" thing here because the manifest has new lines in it. They need to be unescaped when running the command.
				fmt.Sprintf(`if ! $(%s -n cattle-system get deployment cattle-cluster-agent); then printf "%%b\n" %q | %[1]s apply -f - || true; else exit 0; fi`, kubectlCmd, manifest),
			},
			Env:           []string{fmt.Sprintf("KUBECONFIG=/etc/rancher/%s/%[1]s.yaml", runtime)},
			PeriodSeconds: 15,
		})
	}

	return nodePlan, nil
}

// addRemoveOldClusterAgentManifestInstruction will add an instruction to the plan of a control plane node after the agent is deployed to remove the old
// manifest file on the downstream node. This is to ensure that the old agent version isn't redeployed when the node is restarted.
func addRemoveOldClusterAgentManifestInstruction(controlPlane *rkev1.RKEControlPlane, nodePlan plan.NodePlan, entry *planEntry) plan.NodePlan {
	runtime := rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)
	if controlPlane.Status.AgentDeployed && isControlPlane(entry) {
		nodePlan.Instructions = append(nodePlan.Instructions, plan.OneTimeInstruction{
			Name:    "remove-old-cluster-agent-manifest",
			Command: "sh",
			Args: []string{
				"-c",
				fmt.Sprintf("rm -f /var/lib/rancher/%s/server/manifests/rancher/cluster-agent.yaml", runtime),
			},
			Env: []string{fmt.Sprintf("KUBECONFIG=/etc/rancher/%s/%[1]s.yaml", runtime)},
		})
	}

	return nodePlan
}

// generateInstallInstructionWithSkipStart will generate an instruction that executes the `run.sh` or `run.ps1`
// from the installer image based on the control plane configuration. It will add a `SKIP_START` environment variable to prevent
// the service from being started/restarted.
func (p *Planner) generateInstallInstructionWithSkipStart(controlPlane *rkev1.RKEControlPlane, entry *planEntry) plan.OneTimeInstruction {
	var skipStartEnv string
	switch entry.Metadata.Labels[rke2.CattleOSLabel] {
	case windows:
		skipStartEnv = fmt.Sprintf("$env:INSTALL_%s_SKIP_START=\"true\"", strings.ToUpper(rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)))
	default:
		skipStartEnv = fmt.Sprintf("INSTALL_%s_SKIP_START=true", strings.ToUpper(rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)))
	}
	instEnv := []string{skipStartEnv}
	return generateInstallInstruction(controlPlane, entry, instEnv)
}

func (p *Planner) addInitNodePeriodicInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane) plan.NodePlan {
	nodePlan.PeriodicInstructions = append(nodePlan.PeriodicInstructions, []plan.PeriodicInstruction{
		{
			Name:    captureAddressInstructionName,
			Command: "sh",
			Args: []string{
				"-c",
				// the grep here is to make the command fail if we don't get the output we expect, like empty string.
				fmt.Sprintf("curl -f --retry 100 --retry-delay 5 --cacert "+
					"/var/lib/rancher/%s/server/tls/server-ca.crt https://localhost:%d/db/info | grep 'clientURLs'",
					rke2.GetRuntime(controlPlane.Spec.KubernetesVersion),
					rke2.GetRuntimeSupervisorPort(controlPlane.Spec.KubernetesVersion)),
			},
			PeriodSeconds: 600,
		},
		{
			Name:    etcdNameInstructionName,
			Command: "sh",
			Args: []string{
				"-c",
				fmt.Sprintf("cat /var/lib/rancher/%s/server/db/etcd/name", rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)),
			},
			PeriodSeconds: 600,
		},
	}...)
	return nodePlan
}

func (p *Planner) addEtcdSnapshotListLocalPeriodicInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane) plan.NodePlan {
	nodePlan.PeriodicInstructions = append(nodePlan.PeriodicInstructions, plan.PeriodicInstruction{
		Name:    "etcd-snapshot-list-local",
		Command: "sh",
		Args: []string{
			"-c",
			// the grep here is to make the command fail if we don't get the output we expect, like empty string.
			fmt.Sprintf("%s etcd-snapshot list --etcd-s3=false 2>/dev/null",
				rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)),
		},
		PeriodSeconds: 600,
	})
	return nodePlan
}

func (p *Planner) addEtcdSnapshotListS3PeriodicInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane) plan.NodePlan {
	nodePlan.PeriodicInstructions = append(nodePlan.PeriodicInstructions, plan.PeriodicInstruction{
		Name:    "etcd-snapshot-list-s3",
		Command: "sh",
		Args: []string{
			"-c",
			// the grep here is to make the command fail if we don't get the output we expect, like empty string.
			fmt.Sprintf("%s etcd-snapshot list --etcd-s3 2>/dev/null",
				rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)),
		},
		PeriodSeconds: 600,
	})
	return nodePlan
}
