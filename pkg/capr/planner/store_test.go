package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinURLFromAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		port     int
		expected string
	}{
		{
			name:     "ipv4",
			address:  "127.0.0.1",
			port:     9345,
			expected: "https://127.0.0.1:9345",
		},
		{
			name:     "ipv6",
			address:  "::ffff:7f00:1",
			port:     9345,
			expected: "https://[::ffff:7f00:1]:9345",
		},
		{
			name:     "hostname",
			address:  "testing.rancher.io",
			port:     9345,
			expected: "https://testing.rancher.io:9345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, joinURLFromAddress(tt.address, tt.port))
		})
	}
}
