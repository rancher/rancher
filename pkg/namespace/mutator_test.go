package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopy(t *testing.T) {
	cases := []struct {
		name string

		src map[string]string
		dst map[string]string

		expected map[string]string
		updated  bool
	}{
		{
			name: "copy into empty dst",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			updated: true,
		},
		{
			name: "copy into non-empty map without overlap",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{
				"a": "A",
				"b": "B",
			},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},
			updated: true,
		},
		{
			name: "copy into non-empty map with overlap with update",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{
				"baz": "something-else",
				"a":   "A",
				"b":   "B",
			},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},
			updated: true,
		},
		{
			name: "copy into non-empty map with overlap without update",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			updated := copy(c.dst, c.src)

			assert.Equal(t, c.expected, c.dst)
			assert.Equal(t, c.updated, updated)
		})
	}
}
