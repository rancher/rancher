package plansecret

import (
	"testing"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
)

func TestPurgePeriodicInstructionOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		node                   *plan.Node
		expectedPeriodicOutput map[string]plan.PeriodicInstructionOutput
		expectedResult         bool
	}{
		{
			name: "nil node plan",
			node: nil,
		},
		{
			name:           "empty node plan",
			node:           &plan.Node{},
			expectedResult: false,
		},
		{
			name: "periodic instruction in output but not plan",
			node: &plan.Node{
				PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
					"test-instruction": {
						Name: "test-instruction",
					},
				},
			},
			expectedPeriodicOutput: map[string]plan.PeriodicInstructionOutput{},
			expectedResult:         true,
		},
		{
			name: "periodic instruction in plan but not output",
			node: &plan.Node{
				Plan: plan.NodePlan{
					PeriodicInstructions: []plan.PeriodicInstruction{
						{
							CommonInstruction: planapi.CommonInstruction{
								Name: "test-instruction",
							},
						},
					},
				},
			},
			expectedPeriodicOutput: nil,
			expectedResult:         false,
		},
		{
			name: "periodic instruction in plan and output",
			node: &plan.Node{
				Plan: plan.NodePlan{
					PeriodicInstructions: []plan.PeriodicInstruction{
						{
							CommonInstruction: planapi.CommonInstruction{
								Name: "test-instruction",
							},
						},
					},
				},
				PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
					"test-instruction": {
						Name: "test-instruction",
					},
				},
			},
			expectedPeriodicOutput: map[string]plan.PeriodicInstructionOutput{
				"test-instruction": {
					Name: "test-instruction",
				},
			},
			expectedResult: false,
		},
		{
			name: "multiple periodic instructions in output, first only in plan",
			node: &plan.Node{
				Plan: plan.NodePlan{
					PeriodicInstructions: []plan.PeriodicInstruction{
						{
							CommonInstruction: planapi.CommonInstruction{
								Name: "test-instruction",
							},
						},
					},
				},
				PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
					"test-instruction": {
						Name: "test-instruction",
					},
					"not-present": {
						Name: "not-present",
					},
				},
			},
			expectedPeriodicOutput: map[string]plan.PeriodicInstructionOutput{
				"test-instruction": {
					Name: "test-instruction",
				},
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		node := tt.node
		t.Run(tt.name, func(t *testing.T) {
			result := purgePeriodicInstructionOutput(node)
			assert.Equal(t, tt.expectedResult, result)
			if node != nil {
				assert.Equal(t, tt.expectedPeriodicOutput, node.PeriodicOutput)
			}
		})
	}
}
