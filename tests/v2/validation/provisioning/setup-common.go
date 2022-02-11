package provisioning

import "github.com/rancher/rancher/tests/framework/pkg/namegenerator"

const (
	namespace               = "fleet-default"
	defaultRandStringLength = 5
)

func AppendRandomString(baseClusterName string) string {
	clusterName := "auto-" + baseClusterName + "-" + namegenerator.RandStringLower(5)
	return clusterName
}
