package rkenodeconfigserver

import (
	"strings"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/taints"
	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestAppendKubeletArgs(t *testing.T) {
	type testCase struct {
		name             string
		currentCommand   []string
		taints           []rketypes.RKETaint
		expectedTaintSet map[string]struct{}
	}
	testCases := []testCase{
		testCase{
			name:           "taints args not exists",
			currentCommand: []string{"kubelet", "--register-node"},
			taints: []rketypes.RKETaint{
				rketypes.RKETaint{
					Key:    "test1",
					Value:  "value1",
					Effect: v1.TaintEffectNoSchedule,
				},
				rketypes.RKETaint{
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
			taints: []rketypes.RKETaint{
				rketypes.RKETaint{
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
		afterAppend := AppendTaintsToKubeletArgs(processes, tc.taints)
		appendedCommand := getCommandFromProcesses(afterAppend)
		assert.Equal(t, tc.expectedTaintSet, appendedCommand, "", "")
	}
}

func TestShareMntArgs(t *testing.T) {
	augmentedProcesses := getAugmentedKubeletProcesses()
	args := augmentedProcesses["share-mnt"].Args
	// args should be "--", "share-root.sh", "node command in one string", "one argument per shared bind in kubelet process"
	// By default, arg count is 3, plus 2 shared binds we use in the test
	assert.Equal(t, 5, len(args), "args count for share-mnt should be the same")
}

func getKubeletProcess(commands []string) map[string]rketypes.Process {
	return map[string]rketypes.Process{
		"kubelet": rketypes.Process{
			Name:    "kubelet",
			Command: commands,
		},
	}
}

func getAugmentedKubeletProcesses() map[string]rketypes.Process {
	var cluster v3.Cluster
	command := []string{"dummy"}
	binds := []string{"/var/lib/kubelet:/var/lib/kubelet:shared,z", "/var/lib/rancher:/var/lib/rancher:shared,z"}
	processes := map[string]rketypes.Process{
		"kubelet": rketypes.Process{
			Name:    "kubelet",
			Command: command,
			Binds:   binds,
		},
	}

	return AugmentProcesses("token", processes, true, "dummynode", &cluster)
}

func getCommandFromProcesses(processes map[string]rketypes.Process) map[string]struct{} {
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
