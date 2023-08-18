package provisioning

// This file contains all tests that require to ssh into a node to run commands to check things
// such as any stats, benchmarks, etc. For example, ssh is required to check the cpu usage of a
// process running on an individual node.

import (
	"strconv"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	cpuUsageVar = 100 // 100 is just a placeholder until we can determine an actual number. Even with cpu usage spiking it should not go past 100% cpu usage and previous issues concerning this were hitting around 130% and above
	checkCPU    = "CheckCPU"
)

// This func checks the cpu usage of the cluster agent. If the usage is too high the func will return a warning.
func CallSSHTestByName(t *testing.T, testName string, client *rancher.Client, nodes *nodes.Node) {
	switch testName {
	case checkCPU:
		command := "ps -C agent -o %cpu --no-header"
		output, err := nodes.ExecuteCommand(command)
		require.NoError(t, err)
		str_output := output[:strings.IndexByte(output, '\n')]

		output_int, err := strconv.ParseFloat(strings.TrimSpace(str_output), 32)
		if output_int > cpuUsageVar {
			logrus.Warn("warning: cluster agent cpu usage is too high. Current cpu usage is: " + str_output)
		}
		require.NoError(t, err)
	}
}
