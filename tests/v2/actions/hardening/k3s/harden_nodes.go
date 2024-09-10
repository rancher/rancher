package k3s

import (
	"os/user"
	"path/filepath"
	"strings"

	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
)

const (
	sysctlConf = "/etc/sysctl.conf"
)

// HardenK3SNodes hardens the nodes by setting kernel parameters and creating the etcd user
func HardenK3SNodes(nodes []*nodes.Node, nodeRoles []string, kubeVersion string) error {
	for key, node := range nodes {
		logrus.Infof("Setting kernel parameters on node: %s", node.NodeID)
		_, err := node.ExecuteCommand("sudo bash -c 'echo vm.panic_on_oom=0 >> " + sysctlConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic=10 >> " + sysctlConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic_on_oops=1 >> " + sysctlConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.keys.root_maxbytes=25000000 >> " + sysctlConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'sysctl -p " + sysctlConf + "'")
		if err != nil {
			return err
		}

		if strings.Contains(nodeRoles[key], "--controlplane") {
			logrus.Infof("Copying over files to node %s", node.NodeID)
			user, err := user.Current()
			if err != nil {
				return nil
			}

			dirPath := filepath.Join(user.HomeDir, "go/src/github.com/rancher/rancher/tests/v2/actions/hardening/k3s")
			err = node.SCPFileToNode(dirPath+"/audit.yaml", "/home/"+node.SSHUser+"/audit.yaml")
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/audit.yaml /var/lib/rancher/k3s/server/audit.yaml'")
			if err != nil {
				return err
			}
		}
	}

	return nil
}
