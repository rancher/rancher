package management

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
)

func TestDerivePublicFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]v32.Field
		expected []string
	}{
		{
			name:     "nil map returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map returns nil",
			input:    map[string]v32.Field{},
			expected: nil,
		},
		{
			name: "password fields are excluded",
			input: map[string]v32.Field{
				"accessKeyId":     {Type: "string"},
				"accessKeySecret": {Type: "password"},
			},
			expected: []string{"accessKeyId"},
		},
		{
			name: "all password fields returns empty",
			input: map[string]v32.Field{
				"secret1": {Type: "password"},
				"secret2": {Type: "password"},
			},
			expected: nil,
		},
		{
			name: "all non-password fields are included and sorted",
			input: map[string]v32.Field{
				"username": {Type: "string"},
				"endpoint": {Type: "string"},
				"port":     {Type: "int"},
			},
			expected: []string{"endpoint", "port", "username"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := derivePublicFields(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDerivePrivateFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]v32.Field
		expected []string
	}{
		{
			name:     "nil map returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map returns nil",
			input:    map[string]v32.Field{},
			expected: nil,
		},
		{
			name: "only password fields are included",
			input: map[string]v32.Field{
				"accessKeyId":     {Type: "string"},
				"accessKeySecret": {Type: "password"},
			},
			expected: []string{"accessKeySecret"},
		},
		{
			name: "no password fields returns nil",
			input: map[string]v32.Field{
				"username": {Type: "string"},
				"endpoint": {Type: "string"},
			},
			expected: nil,
		},
		{
			name: "multiple password fields are sorted",
			input: map[string]v32.Field{
				"username":  {Type: "string"},
				"password":  {Type: "password"},
				"secretKey": {Type: "password"},
				"apiToken":  {Type: "password"},
			},
			expected: []string{"apiToken", "password", "secretKey"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := derivePrivateFields(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
