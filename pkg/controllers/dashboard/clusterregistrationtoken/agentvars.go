package clusterregistrationtoken

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	corev1 "k8s.io/api/core/v1"
)

type EnvType int

const (
	Linux EnvType = iota
	PowerShell
	Docker
)

func AgentEnvVars(cluster *v3.Cluster, envType EnvType) string {
	var agentEnvVars []string
	if cluster == nil {
		return ""
	}
	for _, envVar := range cluster.Spec.AgentEnvVars {
		if envVar.Value == "" {
			continue
		}
		switch envType {
		case Docker:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("-e \"%s=%s\"", envVar.Name, envVar.Value))
		case PowerShell:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("$env:%s=%s;", envVar.Name, powerShellValue(envVar)))
		default:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("%s=%s", envVar.Name, shellValue(envVar)))
		}
	}
	return strings.Join(agentEnvVars, " ")
}

// needsQuoting reports whether envVar holds a system agent data directory that double quotes cannot protect.
// The cluster controller copies spec.rkeConfig.dataDirectories.systemAgent into this env var, so a shell
// must not parse the value. Other values and valid data directories keep the double quotes, so the command
// stays the same for them.
func needsQuoting(envVar corev1.EnvVar) bool {
	return envVar.Name == capr.SystemAgentDataDirEnvVar && capr.ShellQuote(envVar.Value) != envVar.Value
}

// shellValue returns the value of envVar as one word for a POSIX shell.
func shellValue(envVar corev1.EnvVar) string {
	if needsQuoting(envVar) {
		return capr.ShellQuote(envVar.Value)
	}
	return fmt.Sprintf("\"%s\"", envVar.Value)
}

// powerShellValue returns the value of envVar as one string for PowerShell.
func powerShellValue(envVar corev1.EnvVar) string {
	if needsQuoting(envVar) {
		return capr.PowerShellQuote(envVar.Value)
	}
	return fmt.Sprintf("\"%s\"", envVar.Value)
}
