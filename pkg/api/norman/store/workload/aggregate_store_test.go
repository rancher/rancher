package workload

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/norman/types/convert"
	"github.com/stretchr/testify/assert"
)

func TestResolveWorkloadID_ShortNames(t *testing.T) {
	tests := []struct {
		name        string
		schemaID    string
		namespaceID string
		workloadName string
		expected    string
	}{
		{
			name:        "simple deployment",
			schemaID:    "deployment",
			namespaceID: "default",
			workloadName: "nginx",
			expected:    "deployment-default-nginx",
		},
		{
			name:        "statefulset with longer names",
			schemaID:    "statefulset",
			namespaceID: "kube-system",
			workloadName: "etcd-cluster",
			expected:    "statefulset-kube-system-etcd-cluster",
		},
		{
			name:        "exactly at limit (63 chars)",
			schemaID:    "deployment",
			namespaceID: "test-ns-1234567890123456789012345678901234", // 42 chars
			workloadName: "dep1", // 4 chars
			// Total: deployment(10) + -(1) + namespace(42) + -(1) + name(4) = 58 chars
			expected:    "deployment-test-ns-1234567890123456789012345678901234-dep1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"namespaceId": tt.namespaceID,
				"name":        tt.workloadName,
			}

			result := resolveWorkloadID(tt.schemaID, data)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), MaxLabelValueLength, "workload ID should not exceed max label value length")

			// Verify no annotation was added for short names
			annotations := convert.ToMapInterface(data["workloadAnnotations"])
			if annotations != nil {
				_, hasAnnotation := annotations[WorkloadIDAnnotation]
				assert.False(t, hasAnnotation, "short names should not have annotation")
			}
		})
	}
}

func TestResolveWorkloadID_LongNames(t *testing.T) {
	tests := []struct {
		name        string
		schemaID    string
		namespaceID string
		workloadName string
	}{
		{
			name:        "namespace + deployment > 46 chars",
			schemaID:    "deployment",
			namespaceID: "very-long-namespace-name-with-many-characters-12345678",
			workloadName: "very-long-deployment-name-with-many-characters",
		},
		{
			name:        "both at max length (63 + 63)",
			schemaID:    "deployment",
			namespaceID: strings.Repeat("a", 63),
			workloadName: strings.Repeat("b", 63),
		},
		{
			name:        "replicationcontroller with long names",
			schemaID:    "replicationcontroller",
			namespaceID: "namespace-with-a-moderately-long-name",
			workloadName: "workload-name-that-is-also-quite-long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"namespaceId": tt.namespaceID,
				"name":        tt.workloadName,
			}

			result := resolveWorkloadID(tt.schemaID, data)

			// Verify result is within Kubernetes limits
			assert.LessOrEqual(t, len(result), MaxLabelValueLength, "workload ID must not exceed 63 characters")

			// Verify format is schemaID-hash
			parts := strings.Split(result, "-")
			assert.Equal(t, tt.schemaID, parts[0], "first part should be schemaID")
			assert.Equal(t, 12, len(parts[1]), "hash should be 12 characters")

			// Verify annotation was added with full workload ID
			annotations := convert.ToMapInterface(data["workloadAnnotations"])
			assert.NotNil(t, annotations, "annotations should be set for long names")

			fullID, ok := annotations[WorkloadIDAnnotation]
			assert.True(t, ok, "annotation should contain full workload ID")

			expectedFullID := fmt.Sprintf("%s-%s-%s", tt.schemaID, tt.namespaceID, tt.workloadName)
			assert.Equal(t, expectedFullID, fullID, "annotation should contain original full ID")
		})
	}
}

func TestResolveWorkloadID_HashDeterminism(t *testing.T) {
	// Verify that the same input always produces the same hash
	schemaID := "deployment"
	namespaceID := strings.Repeat("long-namespace-", 10)
	workloadName := strings.Repeat("long-workload-", 10)

	data1 := map[string]interface{}{
		"namespaceId": namespaceID,
		"name":        workloadName,
	}

	data2 := map[string]interface{}{
		"namespaceId": namespaceID,
		"name":        workloadName,
	}

	result1 := resolveWorkloadID(schemaID, data1)
	result2 := resolveWorkloadID(schemaID, data2)

	assert.Equal(t, result1, result2, "same input should always produce same hash")
}

func TestResolveWorkloadID_HashUniqueness(t *testing.T) {
	// Verify that different inputs produce different hashes
	schemaID := "deployment"

	data1 := map[string]interface{}{
		"namespaceId": strings.Repeat("namespace-a-", 10),
		"name":        strings.Repeat("workload-1-", 10),
	}

	data2 := map[string]interface{}{
		"namespaceId": strings.Repeat("namespace-b-", 10),
		"name":        strings.Repeat("workload-2-", 10),
	}

	result1 := resolveWorkloadID(schemaID, data1)
	result2 := resolveWorkloadID(schemaID, data2)

	assert.NotEqual(t, result1, result2, "different inputs should produce different hashes")
}

func TestResolveWorkloadID_BoundaryCase(t *testing.T) {
	// Test the exact boundary where we switch from full name to hash
	schemaID := "deployment" // 10 chars

	// Create a workload ID that's exactly 63 chars (should NOT use hash)
	// deployment(10) + -(1) + namespace(26) + -(1) + name(25) = 63
	data63 := map[string]interface{}{
		"namespaceId": strings.Repeat("a", 26),
		"name":        strings.Repeat("b", 25),
	}

	result63 := resolveWorkloadID(schemaID, data63)
	expected63 := fmt.Sprintf("deployment-%s-%s", strings.Repeat("a", 26), strings.Repeat("b", 25))
	assert.Equal(t, expected63, result63, "63-char workload ID should use full format")
	assert.Equal(t, 63, len(result63))

	// Create a workload ID that's 64 chars (should use hash)
	// deployment(10) + -(1) + namespace(26) + -(1) + name(26) = 64
	data64 := map[string]interface{}{
		"namespaceId": strings.Repeat("a", 26),
		"name":        strings.Repeat("b", 26),
	}

	result64 := resolveWorkloadID(schemaID, data64)
	assert.NotEqual(t, result64, result63, "64-char workload ID should use hash format")
	assert.LessOrEqual(t, len(result64), 63)

	// Verify hash format
	parts := strings.Split(result64, "-")
	assert.Equal(t, "deployment", parts[0])
	assert.Equal(t, 12, len(parts[1]))
}

func TestResolveWorkloadID_MatchesExpectedHash(t *testing.T) {
	// Verify the hash algorithm matches our expectations
	schemaID := "deployment"
	namespaceID := "very-long-namespace-name-that-exceeds-limits"
	workloadName := "very-long-workload-name-that-also-exceeds-limits"

	data := map[string]interface{}{
		"namespaceId": namespaceID,
		"name":        workloadName,
	}

	result := resolveWorkloadID(schemaID, data)

	// Calculate expected hash
	fullID := fmt.Sprintf("%s-%s-%s", schemaID, namespaceID, workloadName)
	hasher := sha256.New()
	hasher.Write([]byte(fullID))
	expectedHash := hex.EncodeToString(hasher.Sum(nil))[:12]
	expectedResult := fmt.Sprintf("%s-%s", schemaID, expectedHash)

	assert.Equal(t, expectedResult, result, "hash should match expected SHA256 output")
}

func TestResolveWorkloadID_PreservesExistingAnnotations(t *testing.T) {
	// Verify that existing annotations are preserved when adding workload ID annotation
	schemaID := "deployment"
	namespaceID := strings.Repeat("long-namespace-", 10)
	workloadName := strings.Repeat("long-workload-", 10)

	data := map[string]interface{}{
		"namespaceId": namespaceID,
		"name":        workloadName,
		"workloadAnnotations": map[string]interface{}{
			"existing.annotation/key": "existing-value",
		},
	}

	resolveWorkloadID(schemaID, data)

	annotations := convert.ToMapInterface(data["workloadAnnotations"])
	assert.NotNil(t, annotations)

	// Check existing annotation is preserved
	assert.Equal(t, "existing-value", annotations["existing.annotation/key"])

	// Check new annotation is added
	fullID := fmt.Sprintf("%s-%s-%s", schemaID, namespaceID, workloadName)
	assert.Equal(t, fullID, annotations[WorkloadIDAnnotation])
}
