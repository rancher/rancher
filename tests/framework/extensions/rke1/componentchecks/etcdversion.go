package componentchecks

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
)

// CheckETCDVersion will check the etcd version on the etcd node in the provisioned RKE1 cluster.
func CheckETCDVersion(client *rancher.Client, nodes []*nodes.Node, nodeRoles []string) ([]string, error) {
	var etcdResult []string

	for key, node := range nodes {
		if strings.Contains(nodeRoles[key], "--etcd") {
			command := fmt.Sprintf("docker exec etcd etcdctl version")
			output, err := node.ExecuteCommand(command)
			if err != nil {
				return []string{}, err
			}

			etcdResult = append(etcdResult, output)
			logrus.Infof(output)
		}
	}

	return etcdResult, nil
}
