package provisioning

// This file contains all tests that require to ssh into a node to run commands to check things
// such as any stats, benchmarks, etc. For example, ssh is required to check the cpu usage of a
// process running on an individual node.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
)

const (
	cpuUsageVar = 100 // 100 is just a placeholder until we can determine an actual number. Even with cpu usage spiking it should not go past 100% cpu usage and previous issues concerning this were hitting around 130% and above
	checkCPU    = "CheckCPU"
)

// CallSSHTestByName tests the ssh tests specified in the provisioninginput config clusterSSHTests field.
// For example CheckCPU checks the cpu usage of the cluster agent. If the usage is too high the func will return a warning.
func CallSSHTestByName(testName string, node *nodes.Node) error {
	switch testName {
	case checkCPU:
		command := "ps -C agent -o %cpu --no-header"
		output, err := node.ExecuteCommand(command)
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
	default:
		err := errors.New("SSHTest: " + testName + " is spelled incorrectly or does not exist.")
		return err
	}
	return nil
}
