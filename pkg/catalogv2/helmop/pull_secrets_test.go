package helmop

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getValueAtPath(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]any
		path      []string
		wantValue any
		wantFound bool
	}{
		{
			name:      "single key exists",
			data:      map[string]any{"foo": "bar"},
			path:      []string{"foo"},
			wantValue: "bar",
			wantFound: true,
		},
		{
			name:      "single key missing",
			data:      map[string]any{"foo": "bar"},
			path:      []string{"baz"},
			wantValue: nil,
			wantFound: false,
		},
		{
			name: "nested key exists",
			data: map[string]any{
				"global": map[string]any{
					"cattle": map[string]any{
						"imagePullSecrets": []any{},
					},
				},
			},
			path:      []string{"global", "cattle", "imagePullSecrets"},
			wantValue: []any{},
			wantFound: true,
		},
		{
			name: "nested key missing at intermediate level",
			data: map[string]any{
				"global": map[string]any{},
			},
			path:      []string{"global", "cattle", "imagePullSecrets"},
			wantValue: nil,
			wantFound: false,
		},
		{
			name: "intermediate value is not a map",
			data: map[string]any{
				"global": "not-a-map",
			},
			path:      []string{"global", "cattle"},
			wantValue: nil,
			wantFound: false,
		},
		{
			name:      "nil value stored at key",
			data:      map[string]any{"foo": nil},
			path:      []string{"foo"},
			wantValue: nil,
			wantFound: true,
		},
		{
			name:      "empty data map",
			data:      map[string]any{},
			path:      []string{"foo"},
			wantValue: nil,
			wantFound: false,
		},
		{
			name: "two-level path exists",
			data: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": "value",
				},
			},
			path:      []string{"global", "imagePullSecrets"},
			wantValue: "value",
			wantFound: true,
		},
		{
			name: "path deeper than data structure",
			data: map[string]any{
				"a": map[string]any{
					"b": "leaf",
				},
			},
			path:      []string{"a", "b", "c"},
			wantValue: nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := getValueAtPath(tt.data, tt.path...)
			assert.Equal(t, tt.wantFound, gotFound)
			assert.Equal(t, tt.wantValue, gotValue)
		})
	}
}

func Test_setValueAtPath(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]any
		path     []string
		value    any
		expected map[string]any
	}{
		{
			name:    "set top-level key",
			initial: map[string]any{},
			path:    []string{"foo"},
			value:   "bar",
			expected: map[string]any{
				"foo": "bar",
			},
		},
		{
			name:    "set nested key, intermediate maps created",
			initial: map[string]any{},
			path:    []string{"global", "cattle", "imagePullSecrets"},
			value:   []any{map[string]string{"name": "my-secret"}},
			expected: map[string]any{
				"global": map[string]any{
					"cattle": map[string]any{
						"imagePullSecrets": []any{map[string]string{"name": "my-secret"}},
					},
				},
			},
		},
		{
			name: "overwrite existing top-level key",
			initial: map[string]any{
				"foo": "old",
			},
			path:  []string{"foo"},
			value: "new",
			expected: map[string]any{
				"foo": "new",
			},
		},
		{
			name: "overwrite existing nested key",
			initial: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{},
				},
			},
			path:  []string{"global", "imagePullSecrets"},
			value: []any{map[string]string{"name": "secret-a"}},
			expected: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{map[string]string{"name": "secret-a"}},
				},
			},
		},
		{
			name: "intermediate is not a map, replaced with new map",
			initial: map[string]any{
				"global": "not-a-map",
			},
			path:  []string{"global", "imagePullSecrets"},
			value: "secrets",
			expected: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": "secrets",
				},
			},
		},
		{
			name: "set nil value",
			initial: map[string]any{
				"foo": "bar",
			},
			path:  []string{"foo"},
			value: nil,
			expected: map[string]any{
				"foo": nil,
			},
		},
		{
			name: "preserves sibling keys",
			initial: map[string]any{
				"global": map[string]any{
					"existing": "preserved",
				},
			},
			path:  []string{"global", "imagePullSecrets"},
			value: "secrets",
			expected: map[string]any{
				"global": map[string]any{
					"existing":         "preserved",
					"imagePullSecrets": "secrets",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValueAtPath(tt.initial, tt.value, tt.path...)
			assert.Equal(t, tt.expected, tt.initial)
		})
	}
}

func Test_chartSupportsImagePullSecrets(t *testing.T) {
	h := &Operations{}

	tests := []struct {
		name            string
		chartBaseValues map[string]any
		wantSupports    bool
		wantErr         bool
	}{
		{
			name: "supports global.cattle.imagePullSecrets",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"cattle": map[string]any{
						"imagePullSecrets": []any{},
					},
				},
			},
			wantSupports: true,
		},
		{
			name: "supports global.imagePullSecrets",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{},
				},
			},
			wantSupports: true,
		},
		{
			name: "supports top-level imagePullSecrets",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			wantSupports: true,
		},
		{
			name: "supports none of the paths",
			chartBaseValues: map[string]any{
				"some": map[string]any{
					"other": map[string]any{
						"key": "value",
					},
				},
			},
			wantSupports: false,
		},
		{
			name:            "empty values",
			chartBaseValues: map[string]any{},
			wantSupports:    false,
		},
		{
			name:            "nil values",
			chartBaseValues: nil,
			wantSupports:    false,
		},
		{
			name: "global key exists but imagePullSecrets missing underneath",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"cattle": map[string]any{
						"someOtherKey": "value",
					},
				},
			},
			wantSupports: false,
		},
		{
			name: "imagePullSecrets declared with non-empty list value",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{
					map[string]any{"name": "my-secret"},
				},
			},
			wantSupports: true,
		},
		{
			name: "multiple paths declared, first match wins",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"cattle": map[string]any{
						"imagePullSecrets": []any{},
					},
					"imagePullSecrets": []any{},
				},
				"imagePullSecrets": []any{},
			},
			wantSupports: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.chartSupportsImagePullSecrets(tt.chartBaseValues)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantSupports, got)
		})
	}
}

func Test_injectPullSecrets(t *testing.T) {
	h := &Operations{}
	tests := []struct {
		name             string
		chartBaseValues  map[string]any
		configuredValues map[string]any
		expected         map[string]any
		pullSecretNames  []string
		wantErr          bool
	}{
		{
			// No imagePullSecrets paths declared in base values; secrets should not be injected.
			name:             "no paths declared in chart base values",
			chartBaseValues:  map[string]any{"someOtherKey": "value"},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{"my-secret"},
			expected:         map[string]any{},
		},
		{
			// The chart declares global.cattle.imagePullSecrets; secrets should be injected there.
			name: "inject at global.cattle.imagePullSecrets",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"cattle": map[string]any{
						"imagePullSecrets": []any{},
					},
				},
			},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{"secret-a"},
			expected: map[string]any{
				"global": map[string]any{
					"cattle": map[string]any{
						"imagePullSecrets": []any{map[string]any{"name": "secret-a"}},
					},
				},
			},
		},
		{
			// The chart declares global.imagePullSecrets; secrets should be injected there.
			name: "inject at global.imagePullSecrets",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{},
				},
			},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{"secret-b"},
			expected: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{map[string]any{"name": "secret-b"}},
				},
			},
		},
		{
			// The chart declares imagePullSecrets at root; secrets should be injected there.
			name: "inject at root imagePullSecrets",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{"secret-c"},
			expected: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "secret-c"}},
			},
		},
		{
			// User has already configured secrets at the path; injection should be skipped to preserve them.
			name: "skip injection when user secrets already configured",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
			},
			pullSecretNames: []string{"injected-secret"},
			expected: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
			},
		},
		{
			// Chart declares multiple paths; all declared paths without existing user secrets should be injected.
			name: "inject at multiple declared paths",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{},
				},
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{"multi-secret"},
			expected: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{map[string]any{"name": "multi-secret"}},
				},
				"imagePullSecrets": []any{map[string]any{"name": "multi-secret"}},
			},
		},
		{
			// No secret names provided; an empty list should be injected at declared paths.
			name: "empty secret names injects empty list",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{},
			expected: map[string]any{
				"imagePullSecrets": []any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawResult, err := h.injectPullSecrets("test", tt.chartBaseValues, tt.configuredValues, tt.pullSecretNames)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Unmarshal the JSON result for a type-safe comparison against the expected map.
			var got map[string]any
			assert.NoError(t, json.Unmarshal(rawResult, &got))
			assert.Equal(t, tt.expected, got)
		})
	}
}
