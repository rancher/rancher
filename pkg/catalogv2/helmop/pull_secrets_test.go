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
		wantSupported   bool
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
			wantSupported: true,
		},
		{
			name: "supports global.imagePullSecrets",
			chartBaseValues: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{},
				},
			},
			wantSupported: true,
		},
		{
			name: "supports top-level imagePullSecrets",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			wantSupported: true,
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
			wantSupported: false,
		},
		{
			name:            "empty values",
			chartBaseValues: map[string]any{},
			wantSupported:   false,
		},
		{
			name:            "nil values",
			chartBaseValues: nil,
			wantSupported:   false,
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
			wantSupported: false,
		},
		{
			name: "imagePullSecrets declared with non-empty list value",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{
					map[string]any{"name": "my-secret"},
				},
			},
			wantSupported: true,
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
			wantSupported: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.chartSupportsImagePullSecrets(tt.chartBaseValues)
			assert.Equal(t, tt.wantSupported, got)
		})
	}
}

func Test_chartHasOnlyUserConfiguredPullSecrets(t *testing.T) {
	h := &Operations{}

	tests := []struct {
		name               string
		baseValues         map[string]any
		values             map[string]any
		managedSecretNames []string
		want               bool
	}{
		{
			name: "all user secrets at single supported path",
			baseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			values: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
			},
			managedSecretNames: []string{"release-foo"},
			want:               true,
		},
		{
			name: "path contains a managed secret",
			baseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			values: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "release-foo"}},
			},
			managedSecretNames: []string{"release-foo"},
			want:               false,
		},
		{
			name: "supported path is empty in values",
			baseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			values:             map[string]any{},
			managedSecretNames: []string{"release-foo"},
			want:               false,
		},
		{
			name: "chart has no supported pull-secret paths",
			baseValues: map[string]any{
				"someOtherKey": "value",
			},
			values: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
			},
			managedSecretNames: []string{},
			want:               false,
		},
		{
			name: "multiple supported paths, all user-only",
			baseValues: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{},
				},
				"imagePullSecrets": []any{},
			},
			values: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
				},
				"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
			},
			managedSecretNames: []string{"release-foo"},
			want:               true,
		},
		{
			name: "multiple supported paths, one path has a managed secret",
			baseValues: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{},
				},
				"imagePullSecrets": []any{},
			},
			values: map[string]any{
				"global": map[string]any{
					"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
				},
				"imagePullSecrets": []any{map[string]any{"name": "release-foo"}},
			},
			managedSecretNames: []string{"release-foo"},
			want:               false,
		},
		{
			name: "mixed path with user and managed secret",
			baseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			values: map[string]any{
				"imagePullSecrets": []any{
					map[string]any{"name": "user-secret"},
					map[string]any{"name": "release-foo"},
				},
			},
			managedSecretNames: []string{"release-foo"},
			want:               false,
		},
		{
			name: "string entry matching managed name is detected as managed",
			baseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			values: map[string]any{
				"imagePullSecrets": []any{"release-foo"},
			},
			managedSecretNames: []string{"release-foo"},
			want:               false,
		},
		{
			name: "string entry not matching any managed name is treated as user-owned",
			baseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			values: map[string]any{
				"imagePullSecrets": []any{"user-secret"},
			},
			managedSecretNames: []string{"release-foo"},
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.chartHasOnlyUserConfiguredPullSecrets(tt.baseValues, tt.values, tt.managedSecretNames)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_injectPullSecrets(t *testing.T) {
	h := &Operations{}
	tests := []struct {
		name              string
		chartBaseValues   map[string]any
		configuredValues  map[string]any
		expected          map[string]any
		pullSecretNames   []string
		knownManagedNames []string
		wantErr           bool
	}{
		{
			name:             "no paths declared in chart base values",
			chartBaseValues:  map[string]any{"someOtherKey": "value"},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{"my-secret"},
			expected:         map[string]any{},
		},
		{
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
			name: "skip injection when path has only user secrets",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
			},
			pullSecretNames:   []string{"injected-secret"},
			knownManagedNames: []string{"injected-secret"},
			expected: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "user-secret"}},
			},
		},
		{
			name: "injects at multiple declared paths",
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
			name: "no secrets and no existing values leaves path untouched",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{},
			pullSecretNames:  []string{},
			expected:         map[string]any{},
		},
		{
			name: "upgrade: managed secret already in values is kept",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "release-secret-a"}},
			},
			pullSecretNames:   []string{"release-secret-a"},
			knownManagedNames: []string{"release-secret-a"},
			expected: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "release-secret-a"}},
			},
		},
		{
			name: "stale managed secret cleared when cluster repo removes it",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "release-secret-a"}},
			},
			pullSecretNames:   []string{},
			knownManagedNames: []string{"release-secret-a"},
			expected: map[string]any{
				// All entries were managed and have been removed, path is set to null.
				"imagePullSecrets": nil,
			},
		},
		{
			name: "mixed path: user secrets preserved, stale managed replaced",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{
				"imagePullSecrets": []any{
					map[string]any{"name": "user-secret"},
					map[string]any{"name": "release-old"},
				},
			},
			pullSecretNames:   []string{"release-new"},
			knownManagedNames: []string{"release-old", "release-new"},
			expected: map[string]any{
				"imagePullSecrets": []any{
					map[string]any{"name": "user-secret"},
					map[string]any{"name": "release-new"},
				},
			},
		},
		{
			name: "user string entries normalized to LocalObjectReference",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{
				"imagePullSecrets": []any{"user-secret", map[string]any{"name": "release-managed"}},
			},
			pullSecretNames:   []string{"release-new"},
			knownManagedNames: []string{"release-managed", "release-new"},
			expected: map[string]any{
				"imagePullSecrets": []any{
					map[string]any{"name": "user-secret"},
					map[string]any{"name": "release-new"},
				},
			},
		},
		{
			name: "user-provided string matching managed name treated as managed",
			chartBaseValues: map[string]any{
				"imagePullSecrets": []any{},
			},
			configuredValues: map[string]any{
				"imagePullSecrets": []any{"release-managed"},
			},
			pullSecretNames:   []string{"release-managed"},
			knownManagedNames: []string{"release-managed"},
			expected: map[string]any{
				"imagePullSecrets": []any{map[string]any{"name": "release-managed"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawResult, err := h.injectPullSecrets("test", tt.chartBaseValues, tt.configuredValues, tt.pullSecretNames, tt.knownManagedNames)
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

func Test_extractUserPullSecrets(t *testing.T) {
	managed := map[string]struct{}{"release-foo": {}}

	tests := []struct {
		name             string
		secretList       []any
		wantUserEntries  []any
		wantManagedFound bool
	}{
		{
			name:             "string entry not in known is normalized to LocalObjectReference",
			secretList:       []any{"user-secret"},
			wantUserEntries:  []any{map[string]string{"name": "user-secret"}},
			wantManagedFound: false,
		},
		{
			name:             "string entry matching managed name is treated as managed",
			secretList:       []any{"release-foo"},
			wantUserEntries:  nil,
			wantManagedFound: true,
		},
		{
			name:             "map entry in known is treated as managed",
			secretList:       []any{map[string]any{"name": "release-foo"}},
			wantUserEntries:  nil,
			wantManagedFound: true,
		},
		{
			name:             "map entry not in known is preserved as-is",
			secretList:       []any{map[string]any{"name": "user-secret"}},
			wantUserEntries:  []any{map[string]any{"name": "user-secret"}},
			wantManagedFound: false,
		},
		{
			name: "mixed string user + managed map: string normalized, managed removed",
			secretList: []any{
				"user-secret",
				map[string]any{"name": "release-foo"},
			},
			wantUserEntries:  []any{map[string]string{"name": "user-secret"}},
			wantManagedFound: true,
		},
		{
			name: "string matching managed name alongside a user map entry",
			secretList: []any{
				"release-foo",
				map[string]any{"name": "user-secret"},
			},
			wantUserEntries:  []any{map[string]any{"name": "user-secret"}},
			wantManagedFound: true,
		},
		{
			name:             "empty list returns nil user entries and no managed found",
			secretList:       []any{},
			wantUserEntries:  nil,
			wantManagedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEntries, gotManaged := extractUserPullSecrets(tt.secretList, managed)
			assert.Equal(t, tt.wantUserEntries, gotEntries)
			assert.Equal(t, tt.wantManagedFound, gotManaged)
		})
	}
}
