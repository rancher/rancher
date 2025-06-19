package management

import (
	"github.com/rancher/rancher/pkg/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sshKeyCleanup(management *config.ManagementContext) error {
	nodeClient := management.Management.Nodes("")
	nodes, err := nodeClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		if node.Status.NodeConfig != nil && node.Status.NodeConfig.SSHKey != "" {
			node.Status.NodeConfig.SSHKey = ""
			_, err = nodeClient.Update(&node)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
