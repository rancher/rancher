package provisioning

// This file contains all tests that require to ssh into a node to run commands to check things
// such as any stats, benchmarks, etc. For example, ssh is required to check the cpu usage of a
// process running on an individual node.

import (
	"context"
	"errors"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	extnodes "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	cpuUsageTolerance = 100 // this value represents 100 core usage which should not happen at any time.
	rancherDir        = "/var/lib/rancher/"

	checkCPUCommand   = "ps -A -o '%c %C' --no-header"
	rebootNodeCommand = "sudo reboot"

	checkCPU   provisioninginput.SSHTestCase = "CheckCPU"
	nodeReboot provisioninginput.SSHTestCase = "NodeReboot"
	auditLog   provisioninginput.SSHTestCase = "AuditLog"

	activeState                = "active"
	runningState               = "running"
	fleetNamespace             = "fleet-default"
	controlPlaneLabel          = "node-role.kubernetes.io/control-plane"
	clusterManagementSteveType = "management.cattle.io.cluster"
	providerLabel              = "provider.cattle.io"
)

// CallSSHTestByName tests the ssh tests specified in the provisioninginput config clusterSSHTests field.
func CallSSHTestByName(testCase provisioninginput.SSHTestCase, node *nodes.Node, client *rancher.Client, clusterID string, machineName string) error {
	switch testCase {
	//checks the cpu usage of all processes on the node. If the usage is too high the function will return a warning.
	case checkCPU:
		logrus.Infof("Running CheckCPU test on node %s", node.PublicIPAddress)
		output, err := node.ExecuteCommand(checkCPUCommand)
		if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
			return errors.New(err.Error() + output)
		}

		lines := strings.Split(output, "\n")
		logrus.Info("Checking all node processes CPU usage")
		for _, line := range lines {
			processFields := strings.Fields(line)
			if len(processFields) > 0 {
				CPUUsageInt, err := strconv.ParseFloat(strings.TrimSpace(processFields[1]), 32)
				if err != nil {
					return errors.New(err.Error() + output)
				}

				if CPUUsageInt >= cpuUsageTolerance {
					logrus.Warnf("Process: %s | CPUUsage: %f", processFields[0], CPUUsageInt)
				}
			}
		}

	//This test reboots the node and verifies it comes back up in the correct state.
	case nodeReboot:
		logrus.Infof("Running NodeReboot test on node %s", node.PublicIPAddress)
		output, err := node.ExecuteCommand(rebootNodeCommand)
		if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
			return errors.New(err.Error() + output)
		}

		err = wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, defaults.FiveMinuteTimeout, true,
			func(ctx context.Context) (done bool, err error) {
				newNode, err := client.Steve.SteveType(machineSteveResourceType).ByID(fleetNamespace + "/" + machineName)
				if err != nil {
					return false, err
				}

				if newNode.State.Name == runningState {
					return false, nil
				}
				return true, nil
			})

		if err != nil {
			logrus.Errorf("Node %s was unable to reboot successfully | Cluster %s is still in active state", node.PublicIPAddress, clusterID)
			return err
		}

		err = extnodes.AllMachineReady(client, clusterID, defaults.TenMinuteTimeout)
		if err != nil {
			logrus.Errorf("Node %s failed to reboot successfully", node.PublicIPAddress)
			return err
		}

		return err

	//This test checks if the audit log file is properly created on the node (skipped if its not a control plane node).
	case auditLog:
		if node.NodeLabels[controlPlaneLabel] != "true" {
			logrus.Infof("Node %s is not a control-plane node, skipping", node.PublicIPAddress)
			return nil
		}

		logrus.Infof("Running audit log test on node %s", node.PublicIPAddress)
		cluster, err := client.Steve.SteveType(clusterManagementSteveType).ByID(clusterID)
		if err != nil {
			return err
		}

		auditLogPath := rancherDir + cluster.Labels[providerLabel]
		auditLogFile := ""
		if cluster.Labels[providerLabel] == "rke2" {
			auditLogFile = "audit-policy-file"
			auditLogPath = auditLogPath + "/etc/config-files/" + auditLogFile
		}

		if cluster.Labels[providerLabel] == "k3s" {
			logrus.Info("Enabling audit logging")
			auditLogFile = "audit.log"
			auditLogPath = auditLogPath + "/server/logs/" + auditLogFile

			user, err := user.Current()
			if err != nil {
				return err
			}

			dirPath := filepath.Join(user.HomeDir, "go/src/github.com/susesgartner/rancher/tests/framework/extensions/provisioning")
			err = node.SCPFileToNode(dirPath+"/enable_auditing_k3s.sh", "/home/"+node.SSHUser+"/enable_auditing_k3s.sh")
			if err != nil {
				return err
			}

			err = node.SCPFileToNode(dirPath+"/audit.yaml", "/home/"+node.SSHUser+"/audit.yaml")
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo bash -c 'mv /home/" + node.SSHUser + "/audit.yaml /var/lib/rancher/k3s/server/audit.yaml'")
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo chmod o+x /home/" + node.SSHUser + "/enable_auditing_k3s.sh")
			if err != nil {
				return err
			}

			_, err = node.ExecuteCommand("sudo bash /home/" + node.SSHUser + "/enable_auditing_k3s.sh")
			if err != nil {
				return err
			}

		}

		checkAuditLogCommand := "ls " + auditLogPath
		logrus.Infof("Checking for audit log file at %s", auditLogPath)
		output, err := node.ExecuteCommand(checkAuditLogCommand)
		if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
			return errors.New(err.Error() + output)
		}

		strOutput := output[:strings.IndexByte(output, '\n')]
		if !strings.Contains(strings.TrimSpace(strOutput), auditLogFile) {
			return errors.New("no audit log file found")
		}

		logrus.Infof("Successfully found audit log file %s", strOutput)

	default:
		err := errors.New("Invalid SSH test: " + string(testCase) + " is spelled incorrectly or does not exist.")
		return err
	}
	return nil
}
