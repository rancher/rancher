package provisioning

// This file contains all tests that require to ssh into a node to run commands to check things
// such as any stats, benchmarks, etc. For example, ssh is required to check the cpu usage of a process running on an individual node.

import (
	"strconv"

	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
)

const (
	cpuUsageVar = 30
)

// This func checks the cpu usage of the cluster agent. If the usage is too high the func will return a warning.
func CheckCPU(node *nodes.Node) (string, error) {
	command := "ps -C agent -o %cpu --no-header"
	output, err := node.ExecuteCommand(command)
	if err != nil {
		return output, err
	}

	output_int, err := strconv.Atoi(output)
	if output_int > cpuUsageVar {
		logrus.Infof("WARNING: cluster agent cpu usage is too high. Current cpu usage is: " + output)
	}

	return output, err
}
