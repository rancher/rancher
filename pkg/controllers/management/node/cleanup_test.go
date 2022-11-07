package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveFinalizers(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		input    []string
		expected []string
	}{
		{
			name:     "empty input",
			prefix:   "test_prefix",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "nil input",
			prefix:   "test_prefix",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty prefix",
			prefix:   "",
			input:    []string{"testing.cattle.io/a_test"},
			expected: nil,
		},
		{
			name:     "match",
			prefix:   "testing.cattle.io/a_",
			input:    []string{"testing.cattle.io/a_test"},
			expected: nil,
		},
		{
			name:     "no match",
			prefix:   "testing.cattle.io/b_",
			input:    []string{"testing.cattle.io/a_test"},
			expected: []string{"testing.cattle.io/a_test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			actual := removeFinalizerWithPrefix(tt.input, tt.prefix)
			a.Equal(tt.expected, actual)
		})
	}
}

func TestRemoveAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "empty input",
			prefix:   "test_prefix",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "nil input",
			prefix:   "test_prefix",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty prefix",
			prefix:   "",
			input:    map[string]string{"testing.cattle.io/a_test": "true"},
			expected: map[string]string{},
		},
		{
			name:     "match",
			prefix:   "testing.cattle.io/a_",
			input:    map[string]string{"testing.cattle.io/a_test": "true"},
			expected: map[string]string{},
		},
		{
			name:     "no match",
			prefix:   "testing.cattle.io/b_",
			input:    map[string]string{"testing.cattle.io/a_test": "true"},
			expected: map[string]string{"testing.cattle.io/a_test": "true"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			actual := removeAnnotationWithPrefix(tt.input, tt.prefix)
			a.Equal(tt.expected, actual)
		})
	}
}
