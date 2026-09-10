package restoremode

import (
	"encoding/json"
	"testing"

	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const kubernetesVersion = "v1.34.1+rke2r1"

// selector renders a bracket-quoted JSONPath the same way snapshotextrametadata does.
func selector(key string, fields ...string) string {
	m := Match{ResourceKey: key, Path: fields}
	return m.String()
}

// provResources is what snapshotextrametadata publishes for a healthy v2prov cluster, plus a few
// shapes the resolver has to treat as "nothing was captured".
func provResources() map[string]any {
	return map[string]any{
		rkev1.SnapshotResourceProvCluster: map[string]any{
			"metadata": map[string]any{"name": "example"},
			"spec": map[string]any{
				"kubernetesVersion": kubernetesVersion,
				"rkeConfig": map[string]any{
					"additionalManifest": "# manifest",
					"machineGlobalConfig": map[string]any{
						"cni": "calico",
					},
				},
				"emptyString": "",
				"emptyMap":    map[string]any{},
				"emptyList":   []any{},
				"null":        nil,
				"machinePools": []any{
					map[string]any{"name": "pool1"},
				},
			},
		},
	}
}

func compress(t *testing.T, v any) string {
	t.Helper()

	payload, err := snapshotutil.CompressInterface(v)
	require.NoError(t, err)
	return payload
}

func TestMatchString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "$['cluster.provisioning.cattle.io']['spec']['kubernetesVersion']",
		Match{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: []string{"spec", "kubernetesVersion"}}.String())
	assert.Equal(t, "$['cluster.management.cattle.io']",
		Match{ResourceKey: rkev1.SnapshotResourceMgmtCluster}.String())
}

func TestResolve(t *testing.T) {
	t.Parallel()

	resources := provResources()

	tests := []struct {
		name     string
		selector string
		// expected is the set of paths, joined by "/", the selector should resolve to.
		expected []string
		wantErr  bool
	}{
		{
			name:     "empty selector resolves to nothing without error",
			selector: "",
		},
		{
			name:     "bracket notation through a dotted resource key",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "kubernetesVersion"),
			expected: []string{"spec/kubernetesVersion"},
		},
		{
			name:     "nested path",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "rkeConfig", "machineGlobalConfig"),
			expected: []string{"spec/rkeConfig/machineGlobalConfig"},
		},
		{
			name:     "dot notation after a bracket-quoted resource key",
			selector: "$['" + rkev1.SnapshotResourceProvCluster + "'].spec.kubernetesVersion",
			expected: []string{"spec/kubernetesVersion"},
		},
		{
			// Why snapshotextrametadata.selector bracket-quotes every segment rather than only the
			// resource key: rancher/jsonpath rejects digits in dot-notation identifiers, so a dotted
			// "rke2Config" is unparsable.
			name:     "dot notation identifier containing a digit",
			selector: "$['" + rkev1.SnapshotResourceMgmtCluster + "'].spec.rke2Config.kubernetesVersion",
			wantErr:  true,
		},
		{
			name:     "selector without a root identifier",
			selector: "spec.kubernetesVersion",
			wantErr:  true,
		},
		{
			name:     "missing field",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "missing"),
		},
		{
			name:     "missing resource",
			selector: selector(rkev1.SnapshotResourceMgmtCluster, "spec", "displayName"),
		},
		{
			name:     "empty string is not captured",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "emptyString"),
		},
		{
			name:     "empty map is not captured",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "emptyMap"),
		},
		{
			name:     "empty list is not captured",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "emptyList"),
		},
		{
			name:     "null is not captured",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "null"),
		},
		{
			// A match inside a slice has no field path SetNestedField could write, and nothing in
			// WritablePaths lives inside a slice, so the walk does not descend into one.
			name:     "selector pointing into a list resolves to nothing",
			selector: "$['" + rkev1.SnapshotResourceProvCluster + "']['spec']['machinePools'][0]['name']",
		},
		{
			name:     "the list itself still resolves",
			selector: selector(rkev1.SnapshotResourceProvCluster, "spec", "machinePools"),
			expected: []string{"spec/machinePools"},
		},
		{
			// rancher/jsonpath only accepts dot-notation identifiers after "..", so a recursive
			// descent cannot address a segment containing a dot or a digit.
			name:     "recursive descent",
			selector: "$..kubernetesVersion",
			expected: []string{"spec/kubernetesVersion"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matches, err := Resolve(tt.selector, resources)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var got []string
			for _, m := range matches {
				got = append(got, join(m.Path))
			}
			assert.ElementsMatch(t, tt.expected, got)
		})
	}

	t.Run("resolves the captured value, not just the path", func(t *testing.T) {
		t.Parallel()

		matches, err := Resolve(selector(rkev1.SnapshotResourceProvCluster, "spec", "kubernetesVersion"), resources)
		require.NoError(t, err)
		require.Len(t, matches, 1)

		assert.Equal(t, rkev1.SnapshotResourceProvCluster, matches[0].ResourceKey)
		assert.Equal(t, []string{"spec", "kubernetesVersion"}, matches[0].Path)
		assert.Equal(t, kubernetesVersion, matches[0].Value)
	})

	t.Run("a selector naming only the resource key yields a pathless match", func(t *testing.T) {
		t.Parallel()

		matches, err := Resolve("$['"+rkev1.SnapshotResourceProvCluster+"']", resources)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Empty(t, matches[0].Path)

		// Writable rejects it: restoring a whole object would clobber metadata and every field the
		// allowlist exists to protect.
		allowed, denied := Writable(matches)
		assert.Empty(t, allowed)
		assert.Len(t, denied, 1)
	})
}

func TestResolveWildcard(t *testing.T) {
	t.Parallel()

	t.Run("expands to the writable paths that were captured", func(t *testing.T) {
		t.Parallel()

		matches, err := Resolve(rkev1.RestoreModeSelectorWildcard, provResources())
		require.NoError(t, err)

		var got []string
		for _, m := range matches {
			assert.Equal(t, rkev1.SnapshotResourceProvCluster, m.ResourceKey)
			got = append(got, join(m.Path))
		}

		// Only the three writable fields provResources populates, and nothing else it contains
		// (metadata, machinePools, the empty shapes).
		assert.ElementsMatch(t, []string{
			"spec/kubernetesVersion",
			"spec/rkeConfig/additionalManifest",
			"spec/rkeConfig/machineGlobalConfig",
		}, got)
	})

	t.Run("everything the wildcard yields is writable", func(t *testing.T) {
		t.Parallel()

		matches, err := Resolve(rkev1.RestoreModeSelectorWildcard, provResources())
		require.NoError(t, err)

		allowed, denied := Writable(matches)
		assert.Len(t, allowed, len(matches))
		assert.Empty(t, denied)
	})

	t.Run("resolves to nothing when no resources were captured", func(t *testing.T) {
		t.Parallel()

		matches, err := Resolve(rkev1.RestoreModeSelectorWildcard, map[string]any{})
		require.NoError(t, err)
		assert.Empty(t, matches)
	})

	t.Run("resolves to nothing when no writable field was captured", func(t *testing.T) {
		t.Parallel()

		matches, err := Resolve(rkev1.RestoreModeSelectorWildcard, map[string]any{
			rkev1.SnapshotResourceProvCluster: map[string]any{
				"metadata": map[string]any{"name": "example"},
				"spec":     map[string]any{"rkeConfig": map[string]any{}},
			},
		})
		require.NoError(t, err)
		assert.Empty(t, matches)
	})

	t.Run("resolves an imported cluster's desired version and agent customizations", func(t *testing.T) {
		t.Parallel()

		// An imported RKE2 cluster: k3sbasedupgrade drives the downstream upgrade from
		// spec.rke2Config.kubernetesVersion, so it is restorable. The k3sConfig path is listed too
		// but resolves to nothing here, since only the config matching Status.Driver is populated.
		matches, err := Resolve(rkev1.RestoreModeSelectorWildcard, map[string]any{
			rkev1.SnapshotResourceMgmtCluster: map[string]any{
				"spec": map[string]any{
					"rke2Config":                          map[string]any{"kubernetesVersion": kubernetesVersion},
					"clusterAgentDeploymentCustomization": map[string]any{"appendTolerations": []any{map[string]any{"key": "a"}}},
				},
			},
		})
		require.NoError(t, err)

		var got []string
		for _, m := range matches {
			got = append(got, join(m.Path))
		}
		assert.ElementsMatch(t, []string{
			"spec/rke2Config/kubernetesVersion",
			"spec/clusterAgentDeploymentCustomization",
		}, got)
	})

	t.Run("skips a resource key with no writable paths", func(t *testing.T) {
		t.Parallel()

		matches, err := Resolve(rkev1.RestoreModeSelectorWildcard, map[string]any{
			"secret.v1": map[string]any{"data": map[string]any{"token": "shhh"}},
		})
		require.NoError(t, err)
		assert.Empty(t, matches)
	})
}

func TestWritable(t *testing.T) {
	t.Parallel()

	allowed, denied := Writable(restoreModeMatches())

	var allowedPaths, deniedPaths []string
	for _, m := range allowed {
		allowedPaths = append(allowedPaths, join(m.Path))
	}
	for _, m := range denied {
		deniedPaths = append(deniedPaths, join(m.Path))
	}

	assert.ElementsMatch(t, []string{"spec/kubernetesVersion", "spec/rkeConfig/registries"}, allowedPaths)
	assert.ElementsMatch(t, []string{"spec/cloudCredentialSecretName", "spec/rkeConfig/machinePools", "metadata/name", ""}, deniedPaths)
}

// restoreModeMatches mixes writable paths with ones a downstream must not be able to restore.
func restoreModeMatches() []Match {
	return []Match{
		{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: []string{"spec", "kubernetesVersion"}, Value: kubernetesVersion},
		{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: []string{"spec", "rkeConfig", "registries"}, Value: map[string]any{}},
		{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: []string{"spec", "cloudCredentialSecretName"}, Value: "cattle-global-data:cc-abc"},
		{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: []string{"spec", "rkeConfig", "machinePools"}, Value: []any{}},
		{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: []string{"metadata", "name"}, Value: "other"},
		{ResourceKey: rkev1.SnapshotResourceProvCluster, Value: map[string]any{}},
	}
}

func TestResources(t *testing.T) {
	t.Parallel()

	t.Run("decompresses the payload", func(t *testing.T) {
		t.Parallel()

		resources, err := Resources(map[string]string{
			rkev1.SnapshotMetadataResourcesKey: compress(t, provResources()),
		})
		require.NoError(t, err)
		assert.Contains(t, resources, rkev1.SnapshotResourceProvCluster)
	})

	t.Run("a snapshot with no resources key yields an empty map", func(t *testing.T) {
		t.Parallel()

		resources, err := Resources(map[string]string{})
		require.NoError(t, err)
		assert.Empty(t, resources)
	})

	t.Run("errors on a corrupt payload", func(t *testing.T) {
		t.Parallel()

		_, err := Resources(map[string]string{
			rkev1.SnapshotMetadataResourcesKey: "not-base64-or-gzip-corrupt-data",
		})
		require.Error(t, err)
	})
}

func TestModes(t *testing.T) {
	t.Parallel()

	t.Run("parses the payload", func(t *testing.T) {
		t.Parallel()

		payload, err := json.Marshal(map[string]string{
			rkev1.RestoreRKEConfigNone:              "",
			rkev1.RestoreRKEConfigKubernetesVersion: selector(rkev1.SnapshotResourceProvCluster, "spec", "kubernetesVersion"),
			rkev1.RestoreRKEConfigAll:               rkev1.RestoreModeSelectorWildcard,
		})
		require.NoError(t, err)

		modes, err := Modes(map[string]string{rkev1.SnapshotMetadataRestoreModesKey: string(payload)})
		require.NoError(t, err)
		assert.Len(t, modes, 3)
		assert.Equal(t, rkev1.RestoreModeSelectorWildcard, modes[rkev1.RestoreRKEConfigAll])
	})

	t.Run("a snapshot with no restoreModes key yields an empty map", func(t *testing.T) {
		t.Parallel()

		modes, err := Modes(map[string]string{})
		require.NoError(t, err)
		assert.Empty(t, modes)
	})

	t.Run("errors on a payload that is not JSON", func(t *testing.T) {
		t.Parallel()

		_, err := Modes(map[string]string{rkev1.SnapshotMetadataRestoreModesKey: "{not json"})
		require.Error(t, err)
	})
}

func TestAvailableModes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"none", "kubernetesVersion", "all"}, AvailableModes("none,kubernetesVersion,all"))
	assert.Equal(t, []string{"none"}, AvailableModes("none"))
	assert.Empty(t, AvailableModes(""))
	// A trailing or doubled comma must not admit the empty mode, which would otherwise match a
	// request that left RestoreMode unset.
	assert.Equal(t, []string{"none", "all"}, AvailableModes("none,,all,"))
	assert.Equal(t, []string{"none", "all"}, AvailableModes(" none , all "))
}

func TestIsPopulated(t *testing.T) {
	t.Parallel()

	assert.False(t, IsPopulated(nil))
	assert.False(t, IsPopulated(""))
	assert.False(t, IsPopulated(map[string]any{}))
	assert.False(t, IsPopulated([]any{}))
	assert.True(t, IsPopulated("v1.34.1+rke2r1"))
	assert.True(t, IsPopulated(map[string]any{"a": 1}))
	assert.True(t, IsPopulated([]any{1}))
	assert.True(t, IsPopulated(false))
	assert.True(t, IsPopulated(int64(0)))
}

func join(path []string) string {
	out := ""
	for i, p := range path {
		if i > 0 {
			out += "/"
		}
		out += p
	}
	return out
}
