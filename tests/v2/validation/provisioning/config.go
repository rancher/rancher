package provisioning

import (
	"strings"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

const (
	namespace               = "fleet-default"
	defaultRandStringLength = 5
	ConfigurationFileKey    = "provisioningInput"
)

type Config struct {
	NodesAndRoles      string   `json:"nodesAndRoles" yaml:"nodesAndRoles" default:""`
	KubernetesVersions []string `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CNIs               []string `json:"cni" yaml:"cni"`
	Providers          []string `json:"providers" yaml:"providers"`
	NodeProviders      []string `json:"nodeProviders" yaml:"nodeProviders"`
}

func AppendRandomString(baseClusterName string) string {
	clusterName := "auto-" + baseClusterName + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	return clusterName
}

// NodesAndRolesInput is a helper function that reads the nodesAndRoles entry in the config file and returns it in the form
// of a list of map[string]bool
func NodesAndRolesInput() []map[string]bool {
	clustersConfig := new(Config)

	config.LoadConfig(ConfigurationFileKey, clustersConfig)
	nodeRolesBoolSliceMap := []map[string]bool{}

	rolesSlice := strings.Split(clustersConfig.NodesAndRoles, "|")
	for _, roles := range rolesSlice {
		nodeRoles := strings.Split(roles, ",")
		nodeRoleBoolMap := map[string]bool{}
		for _, nodeRole := range nodeRoles {
			nodeRoleBoolMap[nodeRole] = true

		}
		nodeRolesBoolSliceMap = append(nodeRolesBoolSliceMap, nodeRoleBoolMap)
	}

	return nodeRolesBoolSliceMap
}
