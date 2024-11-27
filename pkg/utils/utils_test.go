package utils

import "testing"

func TestIsPlainIPV6(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"::1", true},                         // Loopback IPv6
		{"2001:cafe:43:1::3ec8", true},        // IPv6 address
		{"::ffff:7f00:1", true},               // IPv6 address representing IPv4
		{"192.168.1.1", false},                // IPv4 address
		{"192.168.1.1:443", false},            // IPv4 address with port
		{"[2001:cafe:43:1::3ec8]", false},     // Encapsulated IPv6 address
		{"[2001:cafe:43:1::3ec8]:443", false}, // IPv6 address with port
		{"hostname", false},                   // Hostname
		{"invalid ip", false},                 // Invalid IP
		{"", false},                           // Empty string
	}

	for _, tc := range testCases {
		result := IsPlainIPV6(tc.ip)
		if result != tc.expected {
			t.Errorf("For IP %s, expected %t but got %t", tc.ip, tc.expected, result)
		}
	}
}
