package nodedriver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]bool
		expected []string
	}{
		{
			name:     "nil map returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map returns nil",
			input:    map[string]bool{},
			expected: nil,
		},
		{
			name:     "single field",
			input:    map[string]bool{"accessKey": true},
			expected: []string{"accessKey"},
		},
		{
			name:     "multiple fields are sorted",
			input:    map[string]bool{"username": true, "accessKey": true, "endpoint": true},
			expected: []string{"accessKey", "endpoint", "username"},
		},
		{
			name:     "empty string key is excluded",
			input:    map[string]bool{"": true, "accessKey": true},
			expected: []string{"accessKey"},
		},
		{
			name:     "false values are excluded",
			input:    map[string]bool{"password": true, "apiKey": false, "secretKey": true},
			expected: []string{"password", "secretKey"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := fieldList(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
