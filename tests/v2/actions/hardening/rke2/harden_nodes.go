package rke2

import (
	"strings"

	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
)

const (
	kubeletConf = "/etc/sysctl.d/90-kubelet.conf"
)

// HardenRKE2Nodes hardens the nodes by setting kernel parameters and creating the etcd user
func HardenRKE2Nodes(nodes []*nodes.Node, nodeRoles []string) error {
	for _, node := range nodes {
		logrus.Infof("Setting kernel parameters on node: %s", node.NodeID)
		_, err := node.ExecuteCommand("sudo bash -c 'echo vm.panic_on_oom=0 >> " + kubeletConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo vm.overcommit_memory=1 >> " + kubeletConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic=10 >> " + kubeletConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic_on_oops=1 >> " + kubeletConf + "'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'sysctl -p " + kubeletConf + "'")
		if err != nil {
			return err
		}

		logrus.Infof("Creating etcd user on node: %s", node.NodeID)
		_, err = node.ExecuteCommand("sudo useradd -r -c \"etcd user\" -s /sbin/nologin -M etcd -U")
		if err != nil {
			return err
		}
	}

	return nil
}

// PostRKE2HardeningConfig updates the default service account to disable automountServiceAccountToken and
// patches the default service account in each namespace to disable automountServiceAccountToken
func PostRKE2HardeningConfig(nodes []*nodes.Node, nodeRoles []string) error {
	for key, node := range nodes {
		if strings.Contains(nodeRoles[key], "--controlplane") {
			_, err := node.ExecuteCommand(`sudo bash -c 'cat << EOF > /home/` + node.SSHUser + `/account-update.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
automountServiceAccountToken: false
EOF'`)
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/account-update.yaml /var/lib/rancher/rke2/server/account-update.yaml'")
			if err != nil {
				return err
			}

			command := `for namespace in $(kubectl get namespaces -A -o=jsonpath="{.items[*]['metadata.name']}"); do 
    						echo -n "Patching namespace $namespace - "; 
    						kubectl patch serviceaccount default -n ${namespace} -p "$(cat /var/lib/rancher/rke2/server/account-update.yaml)"; 
						done`

			_, err = node.ExecuteCommand("sudo bash -c '" + command + "'")
			if err != nil {
				return err
			}
		}
	}

	return nil
}
