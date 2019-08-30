package rkenodeconfigserver

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/taints"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestAppendKubeletArgs(t *testing.T) {
	type testCase struct {
		name             string
		currentCommand   []string
		taints           []v3.RKETaint
		expectedTaintSet map[string]struct{}
	}
	testCases := []testCase{
		testCase{
			name:           "taints args not exists",
			currentCommand: []string{"kubelet", "--register-node"},
			taints: []v3.RKETaint{
				v3.RKETaint{
					Key:    "test1",
					Value:  "value1",
					Effect: v1.TaintEffectNoSchedule,
				},
				v3.RKETaint{
					Key:    "test2",
					Value:  "value2",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			expectedTaintSet: map[string]struct{}{
				"test1=value1:NoSchedule": struct{}{},
				"test2=value2:NoSchedule": struct{}{},
			},
		},
		testCase{
			name:           "taints args exists",
			currentCommand: []string{"kubelet", "--register-node", "--register-with-taints=node-role.kubernetes.io/controlplane=true:NoSchedule"},
			taints: []v3.RKETaint{
				v3.RKETaint{
					Key:    "test1",
					Value:  "value1",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			expectedTaintSet: map[string]struct{}{
				"node-role.kubernetes.io/controlplane=true:NoSchedule": struct{}{},
				"test1=value1:NoSchedule":                              struct{}{},
			},
		},
	}
	for _, tc := range testCases {
		processes := getKubeletProcess(tc.currentCommand)
		afterAppend := appendTaintsToKubeletArgs(processes, tc.taints)
		appendedCommand := getCommandFromProcesses(afterAppend)
		assert.Equal(t, tc.expectedTaintSet, appendedCommand, "", "")
	}
}

func getKubeletProcess(commands []string) map[string]v3.Process {
	return map[string]v3.Process{
		"kubelet": v3.Process{
			Name:    "kubelet",
			Command: commands,
		},
	}
}

func getCommandFromProcesses(processes map[string]v3.Process) map[string]struct{} {
	kubelet, ok := processes["kubelet"]
	if !ok {
		return nil
	}
	rtn := map[string]struct{}{}
	var tmp map[string]int
	for _, command := range kubelet.Command {
		if strings.HasPrefix(command, "--register-with-taints=") {
			tmp = taints.GetTaintSet(taints.GetTaintsFromStrings(strings.Split(strings.TrimPrefix(command, "--register-with-taints="), ",")))
		}
	}
	for key := range tmp {
		rtn[key] = struct{}{}
	}
	return rtn
}
