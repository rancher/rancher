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
		if envVar.Value == "" {
			continue
		}
		switch envType {
		case Docker:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("-e \"%s=%s\"", envVar.Name, envVar.Value))
		case PowerShell:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("$env:%s=\"%s\";", envVar.Name, envVar.Value))
		default:
			agentEnvVars = append(agentEnvVars, fmt.Sprintf("%s=\"%s\"", envVar.Name, envVar.Value))
		}
	}
	return strings.Join(agentEnvVars, " ")
}
