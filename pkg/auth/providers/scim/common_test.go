package scim

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirst(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		var s []string
		assert.Equal(t, "", first(s))

		s = []string{"a", "b", "c"}
		assert.Equal(t, "a", first(s))
	})

	t.Run("ints", func(t *testing.T) {
		var s []int
		assert.Equal(t, 0, first(s))

		s = []int{1, 2, 3}
		assert.Equal(t, 1, first(s))
	})

	t.Run("structs", func(t *testing.T) {
		type T struct {
			A string
			b []int
		}
		var s []T
		var zero T
		assert.Equal(t, zero, first(s))

		s = []T{
			{A: "first", b: []int{1, 2}},
			{A: "second", b: []int{3, 4}},
		}
		assert.Equal(t, T{A: "first", b: []int{1, 2}}, first(s))
	})
}
