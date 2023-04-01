package ssh

// create all tests here that use executable tests

import (
	"strconv"

	"github.com/rancher/rancher/tests/framework/pkg/nodes"
)

// Test to check cpu usage

func CheckCPU(node *nodes.Node) (string, error) {
	command := "ps -C agent -o %cpu --no-header"
	output, err := node.ExecuteCommand(command)
	if err != nil {
		return output, err
	}

	output_int, err := strconv.Atoi(output)
	if output_int > 30 {
		output = "ERROR: cluster agent cpu usage is too high. Current cpu usage is: " + output
	}

	return output, err
}
