package provisioning

import (
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

const (
	defaultRandStringLength = 5
	ConfigurationFileKey    = "provisioningInput"
)

type Config struct {
	NodesAndRoles      []machinepools.NodeRoles `json:"nodesAndRoles" yaml:"nodesAndRoles" default:"[]"`
	NodesAndRolesRKE1  []nodepools.NodeRoles    `json:"nodesAndRolesRKE1" yaml:"nodesAndRolesRKE1" default:"[]"`
	KubernetesVersions []string                 `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CNIs               []string                 `json:"cni" yaml:"cni"`
	Providers          []string                 `json:"providers" yaml:"providers"`
	NodeProviders      []string                 `json:"nodeProviders" yaml:"nodeProviders"`
}

func AppendRandomString(baseClusterName string) string {
	clusterName := "auto-" + baseClusterName + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	return clusterName
}
