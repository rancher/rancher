package mapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeAddressMapper_FromInternal(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "dual-stack internal and external",
			in: map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{"type": "InternalIP", "address": "10.0.0.1"},
					map[string]interface{}{"type": "InternalIP", "address": "2001:db8::1"},
					map[string]interface{}{"type": "ExternalIP", "address": "203.0.113.10"},
					map[string]interface{}{"type": "ExternalIP", "address": "2001:db8::10"},
					map[string]interface{}{"type": "Hostname", "address": "node-1"},
				},
			},
			want: map[string]interface{}{
				"ipAddress":           "2001:db8::1",
				"ipv4Address":         "10.0.0.1",
				"ipv6Address":         "2001:db8::1",
				"externalIpAddress":   "2001:db8::10",
				"externalIpv4Address": "203.0.113.10",
				"externalIpv6Address": "2001:db8::10",
				"hostname":            "node-1",
			},
		},
		{
			name: "ipv6 listed before ipv4 keeps first of each family",
			in: map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{"type": "InternalIP", "address": "2001:db8::2"},
					map[string]interface{}{"type": "InternalIP", "address": "10.0.0.2"},
					map[string]interface{}{"type": "InternalIP", "address": "10.0.0.3"},
					map[string]interface{}{"type": "InternalIP", "address": "2001:db8::3"},
				},
			},
			want: map[string]interface{}{
				"ipAddress":   "2001:db8::3",
				"ipv4Address": "10.0.0.2",
				"ipv6Address": "2001:db8::2",
			},
		},
		{
			name: "ipv4 only",
			in: map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{"type": "InternalIP", "address": "192.168.1.10"},
					map[string]interface{}{"type": "ExternalIP", "address": "198.51.100.10"},
				},
			},
			want: map[string]interface{}{
				"ipAddress":           "192.168.1.10",
				"ipv4Address":         "192.168.1.10",
				"externalIpAddress":   "198.51.100.10",
				"externalIpv4Address": "198.51.100.10",
			},
		},
		{
			name: "ipv6 only",
			in: map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{"type": "InternalIP", "address": "2001:db8::a"},
					map[string]interface{}{"type": "ExternalIP", "address": "2001:db8::b"},
				},
			},
			want: map[string]interface{}{
				"ipAddress":           "2001:db8::a",
				"ipv6Address":         "2001:db8::a",
				"externalIpAddress":   "2001:db8::b",
				"externalIpv6Address": "2001:db8::b",
			},
		},
		{
			name: "ipv4-mapped ipv6 treated as ipv4",
			in: map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{"type": "InternalIP", "address": "::ffff:192.168.1.1"},
				},
			},
			want: map[string]interface{}{
				"ipAddress":   "::ffff:192.168.1.1",
				"ipv4Address": "192.168.1.1",
			},
		},
		{
			name: "ignores invalid addresses",
			in: map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{"type": "InternalIP", "address": "not-an-ip"},
					map[string]interface{}{"type": "InternalIP", "address": "10.1.2.3"},
					map[string]interface{}{"type": "Hostname", "address": "host"},
				},
			},
			want: map[string]interface{}{
				"ipAddress":   "10.1.2.3",
				"ipv4Address": "10.1.2.3",
				"hostname":    "host",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper := NodeAddressMapper{}
			mapper.FromInternal(tt.in)
			for key, want := range tt.want {
				assert.Equal(t, want, tt.in[key], key)
			}
			// Ensure family fields are not spuriously set when absent from want.
			for _, key := range []string{ipv4Field, ipv6Field, extIPv4Field, extIPv6Field} {
				if _, expected := tt.want[key]; !expected {
					assert.Nil(t, tt.in[key], key)
				}
			}
		})
	}
}
