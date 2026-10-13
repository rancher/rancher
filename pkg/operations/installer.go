package operations

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

// installerImage returns the system-agent installer image for kubernetesVersion, resolved against
// the cluster's private registry when it has one. Mirrors Planner.getInstallerImage
// (pkg/capr/planner/planner.go). Pass a nil control plane for cluster types that have no
// RKEControlPlane, which falls back to the global system-default-registry.
func installerImage(kubernetesVersion string, cp *rkev1.RKEControlPlane) string {
	runtime := capr.GetRuntime(kubernetesVersion)
	img := settings.SystemAgentInstallerImage.Get() + runtime + ":" + strings.ReplaceAll(kubernetesVersion, "+", "-")
	return image.ResolveWithControlPlane(img, cp)
}

// installInstruction builds the distro install instruction for kubernetesVersion with the distro's
// start suppressed, so an operation can lay down a specific version without handing control of the
// service back to it. Mirrors Planner.generateInstallInstructionWithSkipStart
// (pkg/capr/planner/instructions.go).
//
// secret identifies the node being installed, which decides whether the installer lays down the
// server or the agent: a node with neither the etcd nor the control-plane role runs the agent, and
// installing without saying so would give it a server unit it never starts. Mirrors the isOnlyWorker
// branch of Planner.generateInstallInstruction.
//
// Windows is not handled: the callers filter Windows secrets out before they get here.
func installInstruction(kubernetesVersion, dataDir string, cp *rkev1.RKEControlPlane, agentEnvVars []corev1.EnvVar, secret *corev1.Secret) plan.OneTimeInstruction {
	runtimeEnv := capr.GetRuntimeEnv(kubernetesVersion)

	env := []string{
		fmt.Sprintf("INSTALL_%s_SKIP_START=true", runtimeEnv),
		fmt.Sprintf("%s_DATA_DIR=%s", runtimeEnv, dataDir),
	}
	for _, v := range agentEnvVars {
		if v.Value == "" {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", v.Name, v.Value))
	}
	// A node with neither the etcd nor the control-plane role runs the agent. This has to agree with
	// how the caller picks the unit it restarts — installing a server and restarting rke2-agent (or
	// the reverse) would leave the node's unit and its binaries out of step. A nil secret carries no
	// role information at all, so it installs the server rather than guessing.
	if secret != nil && !IsEtcd(secret) && !IsControlPlane(secret) {
		env = append(env, fmt.Sprintf("INSTALL_%s_EXEC=agent", runtimeEnv))
	}

	return plan.OneTimeInstruction{
		CommonInstruction: plan.CommonInstruction{
			Name:    "install",
			Image:   installerImage(kubernetesVersion, cp),
			Command: "sh",
			Args:    []string{"-c", "run.sh"},
			Env:     env,
		},
	}
}

// toCoreEnvVars adapts rkev1's EnvVar to the core type, so installInstruction takes one shape for
// every cluster type: the RKEControlPlane declares agent env vars as []rkev1.EnvVar while the mgmt
// v3 Cluster declares them as []corev1.EnvVar.
func toCoreEnvVars(vars []rkev1.EnvVar) []corev1.EnvVar {
	out := make([]corev1.EnvVar, 0, len(vars))
	for _, v := range vars {
		out = append(out, corev1.EnvVar{Name: v.Name, Value: v.Value})
	}
	return out
}
