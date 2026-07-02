package workload

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveWorkloadID_ShortNames(t *testing.T) {
	tests := []struct {
		name         string
		schemaID     string
		namespaceID  string
		workloadName string
		expected     string
	}{
		{
			name:         "simple deployment",
			schemaID:     "deployment",
			namespaceID:  "default",
			workloadName: "nginx",
			expected:     "deployment-default-nginx",
		},
		{
			name:         "statefulset with longer names",
			schemaID:     "statefulset",
			namespaceID:  "kube-system",
			workloadName: "etcd-cluster",
			expected:     "statefulset-kube-system-etcd-cluster",
		},
		{
			name:         "exactly at limit (63 chars)",
			schemaID:     "deployment",
			namespaceID:  "test-ns-1234567890123456789012345678901234", // 42 chars
			workloadName: "dep1",                                       // 4 chars
			// Total: deployment(10) + -(1) + namespace(42) + -(1) + name(4) = 58 chars
			expected: "deployment-test-ns-1234567890123456789012345678901234-dep1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labelValue, fullID := resolveWorkloadID(tt.schemaID, tt.namespaceID, tt.workloadName)

			assert.Equal(t, tt.expected, labelValue)
			assert.LessOrEqual(t, len(labelValue), MaxLabelValueLength, "workload ID should not exceed max label value length")

			// Verify no fullID returned for short names (no annotation needed)
			assert.Empty(t, fullID, "short names should not return fullID")
		})
	}
}

func TestResolveWorkloadID_LongNames(t *testing.T) {
	tests := []struct {
		name         string
		schemaID     string
		namespaceID  string
		workloadName string
	}{
		{
			name:         "namespace + deployment > 46 chars",
			schemaID:     "deployment",
			namespaceID:  "very-long-namespace-name-with-many-characters-12345678",
			workloadName: "very-long-deployment-name-with-many-characters",
		},
		{
			name:         "both at max length (63 + 63)",
			schemaID:     "deployment",
			namespaceID:  strings.Repeat("a", 63),
			workloadName: strings.Repeat("b", 63),
		},
		{
			name:         "replicationcontroller with long names",
			schemaID:     "replicationcontroller",
			namespaceID:  "namespace-with-a-moderately-long-name",
			workloadName: "workload-name-that-is-also-quite-long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labelValue, fullID := resolveWorkloadID(tt.schemaID, tt.namespaceID, tt.workloadName)

			// Verify result is within Kubernetes limits
			assert.LessOrEqual(t, len(labelValue), MaxLabelValueLength, "workload ID must not exceed 63 characters")

			// Verify fullID was returned (for annotation)
			assert.NotEmpty(t, fullID, "long names should return fullID for annotation")

			expectedFullID := fmt.Sprintf("%s-%s-%s", tt.schemaID, tt.namespaceID, tt.workloadName)
			assert.Equal(t, expectedFullID, fullID, "fullID should contain original full workload ID")

			// Verify label value starts with schemaID
			assert.True(t, strings.HasPrefix(labelValue, tt.schemaID+"-"), "label should start with schemaID")
		})
	}
}

func TestResolveWorkloadID_Determinism(t *testing.T) {
	// Verify that the same input always produces the same result
	schemaID := "deployment"
	namespaceID := strings.Repeat("long-namespace-", 10)
	workloadName := strings.Repeat("long-workload-", 10)

	result1, fullID1 := resolveWorkloadID(schemaID, namespaceID, workloadName)
	result2, fullID2 := resolveWorkloadID(schemaID, namespaceID, workloadName)

	assert.Equal(t, result1, result2, "same input should always produce same label value")
	assert.Equal(t, fullID1, fullID2, "same input should always produce same fullID")
}

func TestResolveWorkloadID_Uniqueness(t *testing.T) {
	// Verify that different inputs produce different results
	schemaID := "deployment"

	result1, _ := resolveWorkloadID(schemaID, strings.Repeat("namespace-a-", 10), strings.Repeat("workload-1-", 10))
	result2, _ := resolveWorkloadID(schemaID, strings.Repeat("namespace-b-", 10), strings.Repeat("workload-2-", 10))

	assert.NotEqual(t, result1, result2, "different inputs should produce different label values")
}

func TestResolveWorkloadID_BoundaryCase(t *testing.T) {
	// Test the exact boundary where we switch from full name to truncated+hash
	schemaID := "deployment" // 10 chars

	// Create a workload ID that's exactly 63 chars (should NOT truncate)
	// deployment(10) + -(1) + namespace(26) + -(1) + name(25) = 63
	namespaceID63 := strings.Repeat("a", 26)
	name63 := strings.Repeat("b", 25)

	labelValue63, fullID63 := resolveWorkloadID(schemaID, namespaceID63, name63)
	expected63 := fmt.Sprintf("deployment-%s-%s", namespaceID63, name63)

	assert.Equal(t, expected63, labelValue63, "63-char workload ID should use full format")
	assert.Equal(t, 63, len(labelValue63))
	assert.Empty(t, fullID63, "63-char workload ID should not return fullID")

	// Create a workload ID that's 64 chars (should truncate+hash)
	// deployment(10) + -(1) + namespace(26) + -(1) + name(26) = 64
	namespaceID64 := strings.Repeat("a", 26)
	name64 := strings.Repeat("b", 26)

	labelValue64, fullID64 := resolveWorkloadID(schemaID, namespaceID64, name64)

	assert.NotEqual(t, labelValue64, labelValue63, "64-char workload ID should use truncated+hash format")
	assert.LessOrEqual(t, len(labelValue64), 63)
	assert.NotEmpty(t, fullID64, "64-char workload ID should return fullID for annotation")
}

func TestResolveWorkloadID_DifferentTypes(t *testing.T) {
	// Verify it works for all workload types
	namespaceID := strings.Repeat("a", 30)
	name := strings.Repeat("b", 30)

	deployment, _ := resolveWorkloadID("deployment", namespaceID, name)
	statefulset, _ := resolveWorkloadID("statefulset", namespaceID, name)
	daemonset, _ := resolveWorkloadID("daemonset", namespaceID, name)

	// All should be within limit
	assert.LessOrEqual(t, len(deployment), 63)
	assert.LessOrEqual(t, len(statefulset), 63)
	assert.LessOrEqual(t, len(daemonset), 63)

	// All should start with their respective type
	assert.True(t, strings.HasPrefix(deployment, "deployment-"))
	assert.True(t, strings.HasPrefix(statefulset, "statefulset-"))
	assert.True(t, strings.HasPrefix(daemonset, "daemonset-"))

	// Different types should produce different results
	assert.NotEqual(t, deployment, statefulset)
	assert.NotEqual(t, deployment, daemonset)
}
