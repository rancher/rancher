package hardening

import (
	"os/user"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
)

func HardenNodes(nodes []*nodes.Node, nodeRoles []string, kubeVersion string) error {
	logrus.Infof("Starting to harden nodes")
	for key, node := range nodes {
		logrus.Infof("Setting kernel parameters on node %s", node.NodeID)
		_, err := node.ExecuteCommand("sudo bash -c 'echo vm.panic_on_oom=0 >> /etc/sysctl.conf'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic=10 >> /etc/sysctl.conf'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.panic_on_oops=1 >> /etc/sysctl.conf'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'echo kernel.keys.root_maxbytes=25000000 >> /etc/sysctl.conf'")
		if err != nil {
			return err
		}

		_, err = node.ExecuteCommand("sudo bash -c 'sysctl -p /etc/sysctl.conf'")
		if err != nil {
			return err
		}

		if strings.Contains(nodeRoles[key], "--controlplane") {
			logrus.Infof("Copying over files to node %s", node.NodeID)
			user, err := user.Current()
			if err != nil {
				return nil
			}

			dirPath := filepath.Join(user.HomeDir, "go/src/github.com/rancher/rancher/tests/framework/extensions/hardening/k3s")
			err = node.SCPFileToNode(dirPath+"/audit.yaml", "/home/"+node.SSHUser+"/audit.yaml")
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/audit.yaml /var/lib/rancher/k3s/server/audit.yaml'")
			if err != nil {
				return err
			}

			if kubeVersion <= string(provisioninginput.PSPKubeVersionLimit) {
				err = node.SCPFileToNode(dirPath+"/psp.yaml", "/home/"+node.SSHUser+"/psp.yaml")
				if err != nil {
					return err
				}

				err = node.SCPFileToNode(dirPath+"/system-policy.yaml", "/home/"+node.SSHUser+"/system-policy.yaml")
				if err != nil {
					return err
				}

				logrus.Infof("Applying hardened YAML files to node: %s", node.NodeID)
				_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/psp.yaml /var/lib/rancher/k3s/psp.yaml'")
				if err != nil {
					return err
				}

				_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/system-policy.yaml /var/lib/rancher/k3s/system-policy.yaml'")
				if err != nil {
					return err
				}

				_, err = node.ExecuteCommand("sudo bash -c 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && kubectl apply -f /var/lib/rancher/k3s/psp.yaml'")
				if err != nil {
					return err
				}

				_, err = node.ExecuteCommand("sudo bash -c 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml && kubectl apply -f /var/lib/rancher/k3s/system-policy.yaml'")
				if err != nil {
					return err
				}
			} else {
				err = node.SCPFileToNode(dirPath+"/psa.yaml", "/home/"+node.SSHUser+"/psa.yaml")
				if err != nil {
					return err
				}

				_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/psa.yaml /var/lib/rancher/k3s/server/psa.yaml'")
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
