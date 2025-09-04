package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterDrainData(t *testing.T) {
	t.Run("when input is nil should return an empty non-nil map", func(t *testing.T) {
		got := filterDrainData(nil)
		assert.Equal(t, make(map[string]any), got)
	})

	t.Run("when top-level server exists should remove only that key and keep others and nested", func(t *testing.T) {
		nested := map[string]any{"server": "nested-should-stay", "foo": "bar"}
		in := map[string]any{
			"server": "https://1.2.3.4:9345",
			"foo":    42,
			"nested": nested,
		}
		expected := map[string]any{
			"foo":    42,
			"nested": nested,
		}

		out := filterDrainData(in)

		assert.Equal(t, expected, out)
	})

	t.Run("when top-level cluster-init exists should remove only that key and keep others and nested", func(t *testing.T) {
		nested := map[string]any{"cluster-init": true, "foo": "bar"}
		in := map[string]any{
			"cluster-init": true,
			"foo":          42,
			"nested":       nested,
		}
		expected := map[string]any{
			"foo":    42,
			"nested": nested,
		}
		out := filterDrainData(in)

		assert.Equal(t, expected, out)
	})

	t.Run("when top-level cluster-init and server exists should remove only those keys and keep others", func(t *testing.T) {
		in := map[string]any{
			"cluster-init": true,
			"server":       "https://1.2.3.4:9345",
			"foo":          42,
		}
		expected := map[string]any{
			"foo": 42,
		}

		out := filterDrainData(in)

		assert.Equal(t, expected, out)
	})

	t.Run("when no \"server\" or \"cluster-init\" key exists should return an identical map", func(t *testing.T) {
		in := map[string]any{"foo": "bar", "baz": 1}
		out := filterDrainData(in)

		assert.Equal(t, out, in)
	})
}
