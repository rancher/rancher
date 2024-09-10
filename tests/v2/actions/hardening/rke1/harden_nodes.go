package rke1

import (
	"os/user"
	"path/filepath"
	"strings"

	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
)

const (
	kubeletConf = "/etc/sysctl.d/90-kubelet.conf"
	GID         = "52034"
	UID         = "52034"
)

// HardenRKE1Nodes hardens the nodes by setting kernel parameters and creating the etcd user and group
func HardenRKE1Nodes(nodes []*nodes.Node, nodeRoles []string) error {
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

		logrus.Infof("Creating etcd group and user on node: %s", node.NodeID)
		_, err = node.ExecuteCommand("sudo groupadd --gid " + GID + " etcd")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo useradd --uid " + UID + " --gid " + GID + " etcd --shell /usr/sbin/nologin")
		if err != nil {
			return err
		}
	}

	return nil
}

// PostRKE1HardeningConfig updates the default service account to disable automountServiceAccountToken and
// patches the default service account in each namespace to disable automountServiceAccountToken
func PostRKE1HardeningConfig(nodes []*nodes.Node, nodeRoles []string) error {
	for key, node := range nodes {
		if strings.Contains(nodeRoles[key], "--etcd") {
			_, err := node.ExecuteCommand("sudo bash -c 'chmod 600 /etc/kubernetes/ssl/kube-ca.pem'")
			if err != nil {
				return err
			}
		}

		if strings.Contains(nodeRoles[key], "--controlplane") {
			logrus.Infof("Copying over files to node %s", node.NodeID)
			user, err := user.Current()
			if err != nil {
				return nil
			}

			dirPath := filepath.Join(user.HomeDir, "go/src/github.com/rancher/rancher/tests/v2/actions/hardening/rke1")
			err = node.SCPFileToNode(dirPath+"/account-update.yaml", "/home/"+node.SSHUser+"/account-update.yaml")
			if err != nil {
				return err
			}

			err = node.SCPFileToNode(dirPath+"/account-update.sh", "/home/"+node.SSHUser+"/account-update.sh")
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo bash -c 'chmod +x /home/" + node.SSHUser + "/account-update.sh'")
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo bash -c '/home/" + node.SSHUser + "/account-update.sh'")
			if err != nil {
				return err
			}
		}
	}

	return nil
}
