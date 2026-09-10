// Package restoremode resolves etcd snapshot restore modes.
//
// A snapshot taken while the etcd snapshot extra metadata ConfigMap was in place carries two
// metadata keys, written by the snapshotextrametadata controller:
//
//   - resources: a base64/gzip JSON map of resource type -> the object captured at snapshot time,
//     e.g. {"cluster.provisioning.cattle.io": {"metadata": ..., "spec": ...}}.
//   - restoreModes: a JSON map of mode name -> a JSONPath selector rooted at the resources map,
//     naming the fields that mode restores.
//
// Two controllers consume that pair. snapshotbackpopulate resolves every selector to decide which
// modes a snapshot can offer, and the etcdsnapshotrestore operations controller resolves the
// requested mode's selector to decide which fields to write back. This package is the single
// implementation both use, so they cannot disagree about what a mode means.
package restoremode

import (
	"encoding/json"
	"fmt"
	"strings"

	jsonpath "github.com/rancher/jsonpath/pkg"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
)

// Match is a single field a restore mode selects, resolved against a resources payload.
type Match struct {
	// ResourceKey is the resources entry the field lives in, i.e. the first selector segment.
	ResourceKey string

	// Path is the field's location within that entry, i.e. the remaining selector segments.
	Path []string

	// Value is the captured value at Path.
	Value any
}

// String renders the match as the selector that would address it, for log messages.
func (m Match) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "$['%s']", m.ResourceKey)
	for _, segment := range m.Path {
		fmt.Fprintf(&b, "['%s']", segment)
	}
	return b.String()
}

// WritablePaths is the upstream guard on what a restore is allowed to change, keyed by resource
// type. Selectors come from the downstream cluster, so without this a downstream could ask Rancher
// to overwrite any field of its own upstream cluster object. Filtering through this list means a
// downstream payload can only ever narrow what is restored, never widen it.
var WritablePaths = map[string][][]string{
	// The fields provisioningcluster.reconcileClusterSpecEtcdRestore writes, which is the legacy
	// definition of what a restore is permitted to change for a v2prov cluster.
	rkev1.SnapshotResourceProvCluster: {
		{"spec", "kubernetesVersion"},
		{"spec", "rkeConfig", "machineGlobalConfig"},
		{"spec", "rkeConfig", "machineSelectorConfig"},
		{"spec", "rkeConfig", "chartValues"},
		{"spec", "rkeConfig", "registries"},
		{"spec", "rkeConfig", "upgradeStrategy"},
		{"spec", "rkeConfig", "additionalManifest"},
		{"spec", "rkeConfig", "networking"},
		{"spec", "defaultPodSecurityAdmissionConfigurationTemplateName"},
		{"spec", "clusterAgentDeploymentCustomization"},
		{"spec", "fleetAgentDeploymentCustomization"},
		{"spec", "webhookDeploymentCustomization"},
	},
	rkev1.SnapshotResourceRKE2ControlPlane: {
		{"spec", "version"},
	},
	// SnapshotResourceMgmtCluster has no entry on purpose. clusterprovisioner writes
	// spec.rke2Config.kubernetesVersion *from* the cluster's reported status, so restoring it would
	// be overwritten on the next reconcile. Imported clusters support "none" only.
}

// Resources decompresses the resources payload out of a snapshot's metadata. A snapshot with no
// resources key yields an empty map rather than an error: it predates the extra metadata ConfigMap.
func Resources(metadata map[string]string) (map[string]any, error) {
	resources := map[string]any{}

	payload := metadata[rkev1.SnapshotMetadataResourcesKey]
	if payload == "" {
		return resources, nil
	}

	if err := snapshotutil.DecompressInterface(payload, &resources); err != nil {
		return nil, fmt.Errorf("decoding %q: %w", rkev1.SnapshotMetadataResourcesKey, err)
	}

	return resources, nil
}

// Modes parses the restoreModes payload out of a snapshot's metadata, returning mode name ->
// selector. A snapshot with no restoreModes key yields an empty map.
func Modes(metadata map[string]string) (map[string]string, error) {
	modes := map[string]string{}

	payload := metadata[rkev1.SnapshotMetadataRestoreModesKey]
	if payload == "" {
		return modes, nil
	}

	if err := json.Unmarshal([]byte(payload), &modes); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", rkev1.SnapshotMetadataRestoreModesKey, err)
	}

	return modes, nil
}

// AvailableModes splits the comma-joined restore-mode-options annotation snapshotbackpopulate stamps
// on an ETCDSnapshot. Empty entries are dropped so a trailing or doubled comma cannot admit "".
func AvailableModes(annotation string) []string {
	var modes []string
	for _, mode := range strings.Split(annotation, ",") {
		if mode = strings.TrimSpace(mode); mode != "" {
			modes = append(modes, mode)
		}
	}
	return modes
}

// Resolve returns every populated field selector picks out of resources.
//
// An empty selector resolves to no matches without error: that is the "none" mode, which restores
// nothing. The wildcard selector expands to every entry of WritablePaths that was captured, which is
// what gives "all" a definition — without it the wildcard would mean every leaf of every object,
// including metadata and fields no restore may touch.
func Resolve(selector string, resources map[string]any) ([]Match, error) {
	switch selector {
	case "":
		return nil, nil
	case rkev1.RestoreModeSelectorWildcard:
		return resolveWildcard(resources), nil
	}

	// Selectors address resources by resource type, e.g.
	// $['cluster.provisioning.cattle.io']['spec']['kubernetesVersion']. Only rancher/jsonpath
	// handles a bracket-quoted key containing dots; client-go's jsonpath splits it on the dots.
	// Note also that rancher/jsonpath rejects digits in dot-notation identifiers, so every segment
	// of a selector this repo produces is bracket-quoted.
	path, err := jsonpath.Parse(selector)
	if err != nil {
		return nil, err
	}

	var matches []Match
	root := jsonpath.PathBuilder{}.WithRootNode()

	for key, resource := range resources {
		obj, ok := resource.(map[string]any)
		if !ok {
			continue
		}

		at := root.WithChildNode(key)

		// The resource entry itself can be the match, e.g. for a selector naming only the key.
		if exact(path, at, root) && IsPopulated(obj) {
			matches = append(matches, Match{ResourceKey: key, Value: obj})
		}

		matches = append(matches, matchesIn(path, at, key, nil, obj)...)
	}

	return matches, nil
}

// exact reports whether path selects here and not merely one of its ancestors.
//
// jsonpath.Matches stops once every selector has been consumed and ignores whatever is left of the
// concrete path, so a selector like $['a']['b'] also "matches" $['a']['b']['c']. That is harmless
// when all a caller wants is "did this selector find anything", but a restore writes the matched
// field back, so it needs the field the selector actually names. A path is exact when the selector
// matches it but does not already match its parent.
func exact(path *jsonpath.JSONPath, here, parent jsonpath.PathBuilder) bool {
	return path.Matches(here.Build()) && !path.Matches(parent.Build())
}

// resolveWildcard returns every WritablePaths entry that was captured in resources. This is
// deliberately driven by the allowlist rather than by the payload's shape: "all" means "everything
// Rancher is willing to restore that this snapshot has a value for".
func resolveWildcard(resources map[string]any) []Match {
	var matches []Match
	for key, paths := range WritablePaths {
		obj, ok := resources[key].(map[string]any)
		if !ok {
			continue
		}
		for _, p := range paths {
			value, found := nested(obj, p)
			if !found || !IsPopulated(value) {
				continue
			}
			matches = append(matches, Match{ResourceKey: key, Path: p, Value: value})
		}
	}
	return matches
}

// matchesIn walks obj depth-first collecting every populated value path matches. rancher/jsonpath
// only exposes matching against a concrete path, so the walk mirrors the one its Set implementation
// performs.
//
// Only maps are descended into. A match inside a slice has no field path unstructured.SetNestedField
// could write it back to, and nothing in WritablePaths lives inside a slice, so a selector pointing
// into one resolves to nothing rather than to a path the applier cannot use. This keeps the
// invariant every caller depends on: a Match's Path is always writable in shape.
func matchesIn(path *jsonpath.JSONPath, at jsonpath.PathBuilder, key string, prefix []string, obj map[string]any) []Match {
	var matches []Match
	for k, v := range obj {
		here := at.WithChildNode(k)
		fieldPath := append(append([]string{}, prefix...), k)

		if exact(path, here, at) && IsPopulated(v) {
			matches = append(matches, Match{ResourceKey: key, Path: fieldPath, Value: v})
		}

		if child, ok := v.(map[string]any); ok {
			matches = append(matches, matchesIn(path, here, key, fieldPath, child)...)
		}
	}
	return matches
}

// Writable splits matches into those WritablePaths permits and those it does not. A denied match is
// a downstream cluster asking Rancher to restore a field it will not restore; callers should surface
// them rather than drop them silently.
func Writable(matches []Match) (allowed, denied []Match) {
	for _, m := range matches {
		if permitted(m) {
			allowed = append(allowed, m)
			continue
		}
		denied = append(denied, m)
	}
	return allowed, denied
}

// permitted reports whether m names a field WritablePaths allows for its resource type. A match with
// no path (the whole resource entry) is never permitted: restoring an entire object would clobber
// metadata and every field the allowlist exists to protect.
func permitted(m Match) bool {
	if len(m.Path) == 0 {
		return false
	}
	for _, p := range WritablePaths[m.ResourceKey] {
		if slicesEqual(p, m.Path) {
			return true
		}
	}
	return false
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// nested reads the value at path out of obj, following maps only.
func nested(obj map[string]any, path []string) (any, bool) {
	var current any = obj
	for _, segment := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// IsPopulated reports whether v carries a value worth restoring. A field that round-tripped through
// JSON as null, an empty string, or an empty container tells us nothing was captured for it, so the
// mode that would restore it is not offered.
func IsPopulated(v any) bool {
	switch v := v.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case map[string]any:
		return len(v) > 0
	case []any:
		return len(v) > 0
	}
	return true
}
