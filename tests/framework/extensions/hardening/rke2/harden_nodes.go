package hardening

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
	"strings"
)

func HardeningNodes(client *rancher.Client, hardened bool, nodes []*nodes.Node, nodeRoles []string) error {
	for key, node := range nodes {
		logrus.Infof("Setting kernel parameters on node %s", node.NodeID)
		_, err := node.ExecuteCommand("sudo bash -c 'echo vm.panic_on_oom=0 >> /etc/sysctl.d/90-kubelet.conf'")
		if err != nil {
			return err
		}
		_, err = node.ExecuteCommand("sudo bash -c 'echo vm.overcommit_memory=1 >> /etc/sysctl.d/90-kubelet.conf'")
		if err != nil {
			return err
		}
		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic=10 >> /etc/sysctl.d/90-kubelet.conf'")
		if err != nil {
			return err
		}
		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic_on_oops=1 >> /etc/sysctl.d/90-kubelet.conf'")
		if err != nil {
			return err
		}
		_, err = node.ExecuteCommand("sudo bash -c 'sysctl -p /etc/sysctl.d/90-kubelet.conf'")
		if err != nil {
			return err
		}
		if strings.Contains(nodeRoles[key], "--etcd") {
			_, err = node.ExecuteCommand("sudo useradd -r -c \"etcd user\" -s /sbin/nologin -M etcd -U")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func PostHardeningConfig(client *rancher.Client, hardened bool, nodes []*nodes.Node, nodeRoles []string) error {
	for key, node := range nodes {

		if strings.Contains(nodeRoles[key], "--controlplane") {
			dir_local := "/go/src/github.com/rancher/rancher/tests/framework/extensions/hardening/rke2"
			err := node.SCPFileToNode(dir_local+"/account-update.yaml", "/home/"+node.SSHUser+"/account-update.yaml")
			if err != nil {
				return err
			}
			err = node.SCPFileToNode(dir_local+"/account-update.sh", "/home/"+node.SSHUser+"/account-update.sh")
			if err != nil {
				return err
			}
			_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/account-update.yaml /var/lib/rancher/rke2/server/account-update.yaml'")
			if err != nil {
				return err
			}
			_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/account-update.sh /var/lib/rancher/rke2/server/account-update.sh'")
			if err != nil {
				return err
			}
			_, err = node.ExecuteCommand("sudo bash -c 'chmod +x /var/lib/rancher/rke2/server/account-update.sh'")
			if err != nil {
				return err
			}
			_, err = node.ExecuteCommand("sudo bash -c 'export KUBECONFIG=/etc/rancher/rke2/rke2.yaml && /var/lib/rancher/rke2/server/account-update.sh'")
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}
