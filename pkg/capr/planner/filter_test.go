package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterDrainData(t *testing.T) {
	// shared nested maps for cases that verify nested keys are preserved
	nestedWithServer := map[string]any{"server": "nested-should-stay", "foo": "bar"}
	nestedWithClusterInit := map[string]any{"cluster-init": true, "foo": "bar"}

	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{
			name: "nil input returns empty non-nil map",
			in:   nil,
			want: map[string]any{},
		},
		{
			name: "remove top-level \"server\" only, preserve others and nested",
			in: map[string]any{
				"server": "https://1.2.3.4:9345",
				"foo":    42,
				"nested": nestedWithServer,
			},
			want: map[string]any{
				"foo":    42,
				"nested": nestedWithServer,
			},
		},
		{
			name: "remove top-level \"cluster-init\" only, preserve others and nested",
			in: map[string]any{
				"cluster-init": true,
				"foo":          42,
				"nested":       nestedWithClusterInit,
			},
			want: map[string]any{
				"foo":    42,
				"nested": nestedWithClusterInit,
			},
		},
		{
			name: "remove both top-level \"server\" and \"cluster-init\"",
			in: map[string]any{
				"cluster-init": true,
				"server":       "https://1.2.3.4:9345",
				"foo":          42,
			},
			want: map[string]any{
				"foo": 42,
			},
		},
		{
			name: "no filtered keys returns identical map",
			in:   map[string]any{"foo": "bar", "baz": 1},
			want: map[string]any{"foo": "bar", "baz": 1},
		},
	}

	for _, tc := range tests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			got := filterDrainData(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}
