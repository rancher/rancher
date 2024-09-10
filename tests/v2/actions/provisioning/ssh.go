package provisioning

// This file contains all tests that require to ssh into a node to run commands to check things
// such as any stats, benchmarks, etc. For example, ssh is required to check the cpu usage of a
// process running on an individual node.

import (
	"errors"
	"strconv"
	"strings"

	"time"

	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
	extnodes "github.com/rancher/shepherd/extensions/nodes"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	cpuUsageVar                                   = 100 // 100 is just a placeholder until we can determine an actual number. Even with cpu usage spiking it should not go past 100% cpu usage and previous issues concerning this were hitting around 130% and above
	checkCPU        provisioninginput.SSHTestCase = "CheckCPU"
	checkCPUCommand                               = "ps -C agent -o %cpu --no-header"
	nodeReboot      provisioninginput.SSHTestCase = "NodeReboot"
	activeState                                   = "active"
	runningState                                  = "running"
	fleetNamespace                                = "fleet-default"
)

// CallSSHTestByName tests the ssh tests specified in the provisioninginput config clusterSSHTests field.
// For example CheckCPU checks the cpu usage of the cluster agent. If the usage is too high the func will return a warning.
func CallSSHTestByName(testCase provisioninginput.SSHTestCase, node *nodes.Node, client *rancher.Client, clusterID string, machineName string) error {
	switch testCase {
	case checkCPU:
		logrus.Infof("Running CheckCPU test on node %s", node.PublicIPAddress)
		output, err := node.ExecuteCommand(checkCPUCommand)
		if err != nil {
			return err
		}
		strOutput := output[:strings.IndexByte(output, '\n')]
		logrus.Info("CheckCPU test on node " + node.PublicIPAddress + " | Cluster agent cpu usage is: " + strOutput + "%")

		outputInt, err := strconv.ParseFloat(strings.TrimSpace(strOutput), 32)
		if outputInt > cpuUsageVar {
			logrus.Warn("Cluster agent cpu usage is too high on node" + node.PublicIPAddress + " | Current cpu usage is: " + strOutput + "%")
		}
		if err != nil {
			return err
		}
	case nodeReboot:
		logrus.Infof("Running NodeReboot test on node %s", node.PublicIPAddress)
		command := "sudo reboot"
		_, err := node.ExecuteCommand(command)
		if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
			return err
		}
		// Verify machine shuts down within five minutes, shutting down should not take longer than that depending on the ami
		err = wait.Poll(1*time.Second, defaults.FiveMinuteTimeout, func() (bool, error) {
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
	default:
		err := errors.New("Invalid SSH test: " + string(testCase) + " is spelled incorrectly or does not exist.")
		return err
	}
	return nil
}
