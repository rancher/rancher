package k3s

import (
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
			_, err = node.ExecuteCommand(`sudo bash -c 'cat << EOF > /home/` + node.SSHUser + `/audit.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
EOF'`)
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
