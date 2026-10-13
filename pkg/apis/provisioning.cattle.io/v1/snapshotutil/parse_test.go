package snapshotutil

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCompressInterface(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{
			name:  "int",
			value: &[]int{1}[0],
		},
		{
			name:  "string",
			value: &[]string{"test"}[0],
		},
		{
			name: "struct",
			value: &struct {
				TestInt    int
				TestString string
			}{
				TestInt:    1,
				TestString: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompressInterface(tt.value)
			assert.Nil(t, err)
			assert.True(t, result != "")

			target := reflect.New(reflect.ValueOf(tt.value).Elem().Type()).Interface()

			err = DecompressInterface(result, target)
			assert.Nil(t, err)
			assert.Equal(t, tt.value, target)
		})
	}
}

// TestDecompressInterfaceNumberTypes pins the number convention DecompressInterface decodes with.
// Consumers that decode into a map[string]any compare the result against, and write it into,
// unstructured objects, where an integral number is an int64; encoding/json would hand back a
// float64 that silently never compares equal to it.
func TestDecompressInterfaceNumberTypes(t *testing.T) {
	payload, err := CompressInterface(map[string]any{
		"integral":    30,
		"negative":    -1,
		"zero":        0,
		"fractional":  1.5,
		"exponential": 1e3,
		"large":       int64(math.MaxInt64),
		"nested":      map[string]any{"deep": 7},
		"slice":       []any{300, map[string]any{"deeper": 8}},
		"string":      "30",
		"bool":        true,
	})
	require.NoError(t, err)

	decoded := map[string]any{}
	require.NoError(t, DecompressInterface(payload, &decoded))

	assert.Equal(t, int64(30), decoded["integral"])
	assert.Equal(t, int64(-1), decoded["negative"])
	assert.Equal(t, int64(0), decoded["zero"])
	assert.Equal(t, 1.5, decoded["fractional"])
	// Marshals as 1000, so it is integral on the wire regardless of its Go type.
	assert.Equal(t, int64(1000), decoded["exponential"])
	assert.Equal(t, int64(math.MaxInt64), decoded["large"])
	assert.Equal(t, int64(7), decoded["nested"].(map[string]any)["deep"])
	assert.Equal(t, int64(300), decoded["slice"].([]any)[0])
	assert.Equal(t, int64(8), decoded["slice"].([]any)[1].(map[string]any)["deeper"])
	// Types other than numbers are untouched.
	assert.Equal(t, "30", decoded["string"])
	assert.Equal(t, true, decoded["bool"])

	// unstructured.SetNestedField rejects any value it cannot represent, so a decoded tree that
	// writes cleanly is also a tree applyRestoreMode can apply.
	for k, v := range decoded {
		require.NoError(t, unstructured.SetNestedField(map[string]any{}, v, "spec", k), "field %q", k)
	}
}

func TestParseSnapshotClusterSpecOrError(t *testing.T) {
	// Constants assumed from the implementation context
	const metaKey = "provisioning-cluster-spec"

	// Helper to create the outer metadata base64 string
	createMetadata := func(data map[string]string) string {
		jsonBytes, err := json.Marshal(data)
		require.NoError(t, err)
		return base64.StdEncoding.EncodeToString(jsonBytes)
	}

	// Valid Data Setup
	validSpec := &provv1.ClusterSpec{
		KubernetesVersion: "v1.26.0",
	}
	validCompressed, err := CompressInterface(validSpec)
	require.NoError(t, err)

	tests := []struct {
		name          string
		snapshot      *rkev1.ETCDSnapshot
		expectedSpec  *provv1.ClusterSpec
		expectedError string
	}{
		{
			name:          "nil snapshot",
			snapshot:      nil,
			expectedSpec:  nil,
			expectedError: "snapshot was nil",
		},
		{
			name: "empty metadata string",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: "",
				},
			},
			expectedSpec:  nil,
			expectedError: "metadata map is empty",
		},
		{
			name: "outer layer invalid base64",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: "!!! not base64 !!!",
				},
			},
			expectedSpec:  nil,
			expectedError: "base64 decode failed",
		},
		{
			name: "outer layer valid base64 but invalid json",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: base64.StdEncoding.EncodeToString([]byte(`{invalid-json`)),
				},
			},
			expectedSpec:  nil,
			expectedError: "JSON unmarshal failed",
		},
		{
			name: "outer layer json missing required key",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						"wrong-key": "some-value",
					}),
				},
			},
			expectedSpec:  nil,
			expectedError: "key not found or empty",
		},
		{
			name: "outer layer json has key but empty value",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						metaKey: "   ",
					}),
				},
			},
			expectedSpec:  nil,
			expectedError: "key not found or empty",
		},
		{
			name: "inner layer invalid base64 (payload corruption)",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						metaKey: "!!! not inner base64 !!!",
					}),
				},
			},
			expectedSpec: nil,
			// Matches the wrap: "reading snapshot metadata into ClusterSpec: ... base64 decode failed"
			expectedError: "reading snapshot metadata into ClusterSpec",
		},
		{
			name: "inner layer valid base64 but not gzip (gzip header missing)",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						// Valid base64, but just a raw string, not gzipped
						metaKey: base64.StdEncoding.EncodeToString([]byte("not-gzipped-data")),
					}),
				},
			},
			expectedSpec: nil,
			// Matches the wrap in decompressInterface: "gzip decompress failed"
			expectedError: "gzip decompress failed",
		},
		{
			name: "success valid snapshot",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						metaKey: validCompressed,
					}),
				},
			},
			expectedSpec:  validSpec,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSnapshotClusterSpecOrError(tt.snapshot)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSpec, result)
			}
		})
	}
}
