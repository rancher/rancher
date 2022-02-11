package provisioning

import (
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const ConfigurationFileKey = "provisioningInput"

type Config struct {
	NodesAndRoles            string                         `json:"nodesAndRoles" yaml:"nodesAndRoles" default:""`
	KubernetesVersions       []string                       `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CNIs                     []string                       `json:"cni" yaml:"cni"`
	Providers                []string                       `json:"providers" yaml:"providers"`
	LocalClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint" yaml:"localClusterAuthEndpoint"`
}

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
