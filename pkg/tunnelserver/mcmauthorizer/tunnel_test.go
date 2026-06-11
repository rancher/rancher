package mcmauthorizer

import (
	"testing"
)

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "bare IPv6 with port",
			address:  "2001:cafe:43::1:443",
			expected: "[2001:cafe:43::1]:443",
		},
		{
			name:     "bare IPv6 loopback with port",
			address:  "::1:6443",
			expected: "[::1]:6443",
		},
		{
			name:     "already bracketed IPv6",
			address:  "[2001:cafe:43::1]:443",
			expected: "[2001:cafe:43::1]:443",
		},
		{
			name:     "IPv4 with port",
			address:  "192.168.1.1:6443",
			expected: "192.168.1.1:6443",
		},
		{
			name:     "hostname with port",
			address:  "my-cluster.example.com:6443",
			expected: "my-cluster.example.com:6443",
		},
		{
			name:     "no port",
			address:  "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "full IPv6 with port",
			address:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334:443",
			expected: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:443",
		},
		{
			name:     "empty string",
			address:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAddress(tt.address)
			if result != tt.expected {
				t.Errorf("formatAddress(%q) = %q, want %q", tt.address, result, tt.expected)
			}
		})
	}
}
