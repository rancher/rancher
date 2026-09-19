package taints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestMergeTaints(t *testing.T) {
	type testCase struct {
		name         string
		t1           []v1.Taint
		t2           []v1.Taint
		mergedTaints []v1.Taint
	}
	testCases := []testCase{
		testCase{
			name: "merge with unique key and effect",
			t1: []v1.Taint{
				v1.Taint{
					Key:    "t1",
					Value:  "t1",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			t2: []v1.Taint{
				v1.Taint{
					Key:    "t2",
					Value:  "t2",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			mergedTaints: []v1.Taint{
				v1.Taint{
					Key:    "t1",
					Value:  "t1",
					Effect: v1.TaintEffectNoSchedule,
				},
				v1.Taint{
					Key:    "t2",
					Value:  "t2",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
		testCase{
			name: "override values",
			t1: []v1.Taint{
				v1.Taint{
					Key:    "t1",
					Value:  "t1",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			t2: []v1.Taint{
				v1.Taint{
					Key:    "t1",
					Value:  "v3",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			mergedTaints: []v1.Taint{
				v1.Taint{
					Key:    "t1",
					Value:  "v3",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
	}
	for _, tc := range testCases {
		merged := MergeTaints(tc.t1, tc.t2)
		mergedSet := getUniqueSet(GetTaintSet(merged))
		expectedSet := getUniqueSet(GetTaintSet(tc.mergedTaints))
		assert.Equal(t, expectedSet, mergedSet, "test case %s failed, expected merged taints %+v are different from merged taints %+v", tc.name, expectedSet, mergedSet)
	}
}

func TestIsTransient(t *testing.T) {
	testCases := []struct {
		key      string
		expected bool
	}{
		// node lifecycle taints, added and removed by a controller
		{key: "node.kubernetes.io/not-ready", expected: true},
		{key: "node.kubernetes.io/unreachable", expected: true},
		{key: "node.kubernetes.io/memory-pressure", expected: true},
		{key: "node.kubernetes.io/unschedulable", expected: true},
		{key: "node.cloudprovider.kubernetes.io/uninitialized", expected: true},
		{key: "node.cloudprovider.kubernetes.io/shutdown", expected: true},
		// durable taints, set by the role configuration or an administrator
		{key: "node-role.kubernetes.io/control-plane", expected: false},
		{key: "node-role.kubernetes.io/controlplane", expected: false},
		{key: "node-role.kubernetes.io/etcd", expected: false},
		{key: "node.cilium.io/agent-not-ready", expected: false},
		{key: "dedicated", expected: false},
		// the prefixes must not match partial path segments
		{key: "node.kubernetes.iofoo/bar", expected: false},
		{key: "node.cloudprovider.kubernetes.iofoo/bar", expected: false},
	}
	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			assert.Equal(t, tc.expected, IsTransient(v1.Taint{Key: tc.key, Effect: v1.TaintEffectNoSchedule}))
		})
	}
}

func TestSort(t *testing.T) {
	taints := []v1.Taint{
		{Key: "b", Value: "v2", Effect: v1.TaintEffectNoSchedule},
		{Key: "a", Value: "", Effect: v1.TaintEffectNoExecute},
		{Key: "b", Value: "v1", Effect: v1.TaintEffectNoSchedule},
		{Key: "a", Value: "", Effect: v1.TaintEffectNoSchedule},
	}
	expected := []v1.Taint{
		{Key: "a", Value: "", Effect: v1.TaintEffectNoExecute},
		{Key: "a", Value: "", Effect: v1.TaintEffectNoSchedule},
		{Key: "b", Value: "v1", Effect: v1.TaintEffectNoSchedule},
		{Key: "b", Value: "v2", Effect: v1.TaintEffectNoSchedule},
	}

	Sort(taints)
	assert.Equal(t, expected, taints)

	// sorting is idempotent, whatever the input order
	Sort(taints)
	assert.Equal(t, expected, taints)
}

func getUniqueSet(set map[string]int) map[string]struct{} {
	rtn := make(map[string]struct{}, len(set))
	for key := range set {
		rtn[key] = struct{}{}
	}
	return rtn
}
