package clusterregistrationtoken

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/sirupsen/logrus"
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
		name := envVar.Name
		if envType == PowerShell {
			// FormatWindowsEnvVar accepts the "$env:" prefix.
			name = strings.TrimPrefix(name, "$env:")
		}
		if !capr.IsValidEnvVarName(name) {
			logrus.Warnf("skipping invalid agent environment variable name: %q", envVar.Name)
			continue
		}
		switch envType {
		case Docker:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("-e %s=%s", envVar.Name, capr.ShellSingleQuote(envVar.Value)))
		case PowerShell:
			agentEnvVars = append(agentEnvVars, capr.FormatWindowsEnvVar(envVar, false)+";")
		default:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("%s=%s", envVar.Name, capr.ShellSingleQuote(envVar.Value)))
		}
	}
	return strings.Join(agentEnvVars, " ")
}
