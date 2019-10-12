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
		{
			name: "merge with unique key and effect",
			t1: []v1.Taint{
				{
					Key:    "t1",
					Value:  "t1",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			t2: []v1.Taint{
				{
					Key:    "t2",
					Value:  "t2",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			mergedTaints: []v1.Taint{
				{
					Key:    "t1",
					Value:  "t1",
					Effect: v1.TaintEffectNoSchedule,
				},
				{
					Key:    "t2",
					Value:  "t2",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
		{
			name: "override values",
			t1: []v1.Taint{
				{
					Key:    "t1",
					Value:  "t1",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			t2: []v1.Taint{
				{
					Key:    "t1",
					Value:  "v3",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			mergedTaints: []v1.Taint{
				{
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

func getUniqueSet(set map[string]int) map[string]struct{} {
	rtn := make(map[string]struct{}, len(set))
	for key := range set {
		rtn[key] = struct{}{}
	}
	return rtn
}
