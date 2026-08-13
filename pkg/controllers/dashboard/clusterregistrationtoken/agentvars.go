package clusterregistrationtoken

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
		value := strings.TrimRight(envVar.Value, "\r\n")
		if value == "" {
			continue
		}
		switch envType {
		case Docker:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("-e \"%s=%s\"", envVar.Name, value))
		case PowerShell:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("$env:%s=\"%s\";", envVar.Name, value))
		default:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("%s=\"%s\"", envVar.Name, value))
		}
	}
	return strings.Join(agentEnvVars, " ")
}
