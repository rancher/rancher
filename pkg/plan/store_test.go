package plan

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to build a mock secret pointer
func mockSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name     string
		results  []PlanStatus
		expected string
	}{
		{
			name:     "Empty results slice",
			results:  []PlanStatus{},
			expected: "",
		},
		{
			name: "Nil secret items are skipped safely",
			results: []PlanStatus{
				{Secret: nil, Pending: true},
			},
			expected: "",
		},
		{
			name: "Single node waiting for plan applied",
			results: []PlanStatus{
				{Secret: mockSecret("node-alpha"), InProgress: true},
			},
			expected: "waiting for plan applied for node-alpha",
		},
		{
			name: "Two nodes waiting for plan picked up (Verifies exact suffix '1 other node')",
			results: []PlanStatus{
				// Out of order to test lexicographical sorting picks node-alpha as primary
				{Secret: mockSecret("node-beta"), Pending: true},
				{Secret: mockSecret("node-alpha"), Pending: true},
			},
			expected: "waiting for plan to be picked up for node-alpha & 1 other node",
		},
		{
			name: "Four nodes waiting for probes (Verifies plural scaling suffix)",
			results: []PlanStatus{
				{Secret: mockSecret("node-d"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-b"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-a"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-c"), Applied: true, ProbesPassed: false},
			},
			expected: "waiting for probes for node-a & 3 other nodes",
		},
		{
			name: "Strictly failed or completely successful nodes are excluded",
			results: []PlanStatus{
				{Secret: mockSecret("node-good"), Applied: true, ProbesPassed: true},
				{Secret: mockSecret("node-dead"), Failed: true},
			},
			expected: "",
		},
		{
			name: "Mixed messages with priority ordering (Failing -> Pending -> InProgress -> Probes)",
			results: []PlanStatus{
				{Secret: mockSecret("node-probes"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-failing"), Failing: true, Failed: false},
				{Secret: mockSecret("node-progress"), InProgress: true},
				{Secret: mockSecret("node-pending"), Pending: true},
			},
			expected: "failing plan for node-failing, waiting for plan to be picked up for node-pending, waiting for plan applied for node-progress, waiting for probes for node-probes",
		},
		{
			name: "Mixed messages with duplicate nodes per tier",
			results: []PlanStatus{
				{Secret: mockSecret("node-p1"), Pending: true},
				{Secret: mockSecret("node-p2"), Pending: true},
				{Secret: mockSecret("node-f1"), Failing: true, Failed: false},
				{Secret: mockSecret("node-f2"), Failing: true, Failed: false},
				{Secret: mockSecret("node-f3"), Failing: true, Failed: false},
			},
			expected: "failing plan for node-f1 & 2 other nodes, waiting for plan to be picked up for node-p1 & 1 other node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Message(tt.results)
			if actual != tt.expected {
				t.Errorf("\nExpected: %q\nGot:      %q", tt.expected, actual)
			}
		})
	}
}