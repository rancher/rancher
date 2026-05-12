package planner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/stretchr/testify/assert"
)

func TestPrimaryAddressFamily(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected int
	}{
		{
			name:     "returns 0 when service-cidr is missing",
			config:   map[string]interface{}{},
			expected: 0,
		},
		{
			name: "returns 4 for IPv4-first CIDR",
			config: map[string]interface{}{
				"service-cidr": "10.43.0.0/16,2001:db8:43::/112",
			},
			expected: 4,
		},
		{
			name: "returns 6 for IPv6-first CIDR",
			config: map[string]interface{}{
				"service-cidr": "2001:db8:43::/112,10.43.0.0/16",
			},
			expected: 6,
		},
		{
			name: "handles whitespace and multiple list entries",
			config: map[string]interface{}{
				"service-cidr": []string{"   ", "  2001:db8:43::/112  ,10.43.0.0/16"},
			},
			expected: 6,
		},
		{
			name: "returns 0 for invalid first CIDR",
			config: map[string]interface{}{
				"service-cidr": "not-a-cidr,10.43.0.0/16",
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, primaryAddressFamily(tt.config))
		})
	}
}

func TestReorderAddressesByFamily(t *testing.T) {
	tests := []struct {
		name           string
		addresses      []string
		primaryFamily  int
		expected       []string
	}{
		{
			name:          "no-op when fewer than two addresses",
			addresses:     []string{"10.0.0.1"},
			primaryFamily: 4,
			expected:      []string{"10.0.0.1"},
		},
		{
			name:          "no-op when primary family is unset",
			addresses:     []string{"2001:db8::1", "10.0.0.1"},
			primaryFamily: 0,
			expected:      []string{"2001:db8::1", "10.0.0.1"},
		},
		{
			name:          "reorders IPv4 first and keeps group order stable",
			addresses:     []string{"2001:db8::1", "10.0.0.1", "eth0", "10.0.0.2", "iface0", "2001:db8::2"},
			primaryFamily: 4,
			expected:      []string{"10.0.0.1", "10.0.0.2", "2001:db8::1", "2001:db8::2", "eth0", "iface0"},
		},
		{
			name:          "reorders IPv6 first and keeps non-IP values at the end",
			addresses:     []string{"10.0.0.2", "2001:db8::b", "eth0", "2001:db8::a", "10.0.0.1"},
			primaryFamily: 6,
			expected:      []string{"2001:db8::b", "2001:db8::a", "10.0.0.2", "10.0.0.1", "eth0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, reorderAddressesByFamily(tt.addresses, tt.primaryFamily))
		})
	}
}

func TestUpdateConfigWithAddresses(t *testing.T) {
	tests := []struct {
		name                    string
		initialConfig           map[string]interface{}
		info                    *machineNetworkInfo
		expectedNodeIPs         []string
		expectedNodeExternalIPs []string
	}{
		{
			name: "AWS dual-stack node",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"service-cidr":        "10.43.0.0/16,2001:db8:43::/112",
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"1.2.3.4"},
				IPv6Address:       "2001:db8::1",
				DriverName:        management.Amazonec2driver,
			},
			expectedNodeIPs:         []string{"10.0.0.5", "2001:db8::1"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
		{
			name: "AWS dual-stack node with IPv6-first service-cidr",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"service-cidr":        "2001:db8:43::/112,10.43.0.0/16",
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"1.2.3.4"},
				IPv6Address:       "2001:db8::1",
				DriverName:        management.Amazonec2driver,
			},
			expectedNodeIPs:         []string{"2001:db8::1", "10.0.0.5"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
		{
			name: "AWS IPv4-only node",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"1.2.3.4"},
				DriverName:        management.Amazonec2driver,
			},
			expectedNodeIPs:         []string{"10.0.0.5"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
		{
			name: "AWS IPv6-only node",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				IPv6Address: "2001:db8::1",
				DriverName:  management.Amazonec2driver,
			},
			expectedNodeIPs:         []string{"2001:db8::1"},
			expectedNodeExternalIPs: []string{},
		},
		{
			name: "DigitalOcean IPv4-only with internal IP",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.1.2.3"},
				ExternalAddresses: []string{"203.0.113.1"},
				DriverName:        management.DigitalOceandriver,
			},
			expectedNodeIPs:         []string{"10.1.2.3"},
			expectedNodeExternalIPs: []string{"203.0.113.1"},
		},
		{
			name: "DigitalOcean driver IPv4-only with no internal IP",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{},
				ExternalAddresses: []string{"203.0.113.1"},
				DriverName:        management.DigitalOceandriver,
			},
			expectedNodeIPs:         []string{"203.0.113.1"},
			expectedNodeExternalIPs: []string{},
		},
		{
			name: "DigitalOcean driver dual-stack with internal IP",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.1.2.3"},
				ExternalAddresses: []string{"203.0.113.1"},
				IPv6Address:       "2001:db8::1",
				DriverName:        management.DigitalOceandriver,
			},
			expectedNodeIPs:         []string{"10.1.2.3", "2001:db8::1"},
			expectedNodeExternalIPs: []string{"203.0.113.1"},
		},
		{
			name: "Pod driver skips node-ip assignment",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.10.10.5"},
				ExternalAddresses: []string{"172.16.1.5"},
				DriverName:        management.PodDriver,
			},
			expectedNodeIPs:         []string{},
			expectedNodeExternalIPs: []string{},
		},
		{
			name: "Cloud provider set disables external IP assignment",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "aws",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.7"},
				ExternalAddresses: []string{"203.0.113.5"},
				IPv6Address:       "2001:db8::7",
				DriverName:        "amazonec2",
			},
			expectedNodeIPs:         []string{"10.0.0.7", "2001:db8::7"},
			expectedNodeExternalIPs: []string{},
		},
		{
			name: "Multiple internal and external IPs",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5", "10.0.0.6"},
				ExternalAddresses: []string{"1.2.3.4", "1.2.3.5"},
				IPv6Address:       "2001:db8::1",
			},
			expectedNodeIPs:         []string{"10.0.0.5", "10.0.0.6", "2001:db8::1"},
			expectedNodeExternalIPs: []string{"1.2.3.4", "1.2.3.5"},
		},
		{
			name: "Multiple internal IPs, one is IPv6",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"2001:db8::2", "10.0.0.6"},
				ExternalAddresses: []string{"1.2.3.4"},
			},
			expectedNodeIPs:         []string{"2001:db8::2", "10.0.0.6"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
		{
			name: "Multiple internal IPs, no IPv4",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"2001:db8::2", "2001:db8::3"},
				ExternalAddresses: []string{"1.2.3.4"},
			},
			expectedNodeIPs:         []string{"2001:db8::2", "2001:db8::3"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
		{
			name: "Duplicated internal and external IPs",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"2001:db8::2", "10.0.0.6"},
				ExternalAddresses: []string{"2001:db8::2", "10.0.0.6"},
			},
			expectedNodeIPs:         []string{"2001:db8::2", "10.0.0.6"},
			expectedNodeExternalIPs: []string{},
		},
		{
			name: "Duplicated internal and external and IPv6",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"1.2.3.4", "1.2.3.5", "1.2.3.7"},
				ExternalAddresses: []string{"1.2.3.4", "1.2.3.5", "1.2.3.6"},
				IPv6Address:       "2001:db8::1",
			},
			expectedNodeIPs:         []string{"1.2.3.4", "1.2.3.5", "1.2.3.7", "2001:db8::1"},
			expectedNodeExternalIPs: []string{"1.2.3.6"},
		},
		{
			name: "Interface names as addresses",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"eth0"},
				ExternalAddresses: []string{"wt0"},
			},
			expectedNodeIPs:         []string{"eth0"},
			expectedNodeExternalIPs: []string{"wt0"},
		},
		{
			name: "Mixed interface names and IPs",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"eth0"},
				ExternalAddresses: []string{"1.2.3.4"},
				IPv6Address:       "2001:db8::1",
				DriverName:        management.Amazonec2driver,
			},
			expectedNodeIPs:         []string{"eth0", "2001:db8::1"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
		{
			name: "IPv6 link-local external address is ignored",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"fe80::250:56ff:fe87:84dc"},
				IPv6Address:       "fe80::250:56ff:fe87:84dc",
			},
			expectedNodeIPs:         []string{"10.0.0.5", "fe80::250:56ff:fe87:84dc"},
			expectedNodeExternalIPs: []string{},
		},
		{
			name: "Global Unicast IPv6 should still be allowed",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"2001:db8::dead:beef"},
			},
			expectedNodeIPs:         []string{"10.0.0.5"},
			expectedNodeExternalIPs: []string{"2001:db8::dead:beef"},
		},
		{
			name: "node-external-ip is reordered to match IPv6-first service-cidr",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"service-cidr":        "2001:db8:43::/112,10.43.0.0/16",
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"1.2.3.4", "2001:db8::dead:beef"},
			},
			expectedNodeIPs:         []string{"10.0.0.5"},
			expectedNodeExternalIPs: []string{"2001:db8::dead:beef", "1.2.3.4"},
		},
		{
			name: "invalid service-cidr preserves existing node-ip order",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"service-cidr":        "not-a-cidr,10.43.0.0/16",
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"1.2.3.4"},
				IPv6Address:       "2001:db8::1",
				DriverName:        management.Amazonec2driver,
			},
			expectedNodeIPs:         []string{"10.0.0.5", "2001:db8::1"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
		// Azure (and other drivers that only populate Driver.IPAddress with no PrivateIPAddress):
		// node-ip and node-external-ip must both remain unset when there is no internal IP.
		{
			name: "Azure default config: only external IP, no internal IP - neither node-ip nor node-external-ip set",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{},
				ExternalAddresses: []string{"20.1.2.3"},
				DriverName:        management.Azuredriver,
			},
			expectedNodeIPs:         []string{},
			expectedNodeExternalIPs: []string{},
		},
		// Other drivers without PrivateIPAddress (e.g. vSphere, Google) also hit the default
		// case and must not have node-external-ip set when InternalAddresses is empty.
		{
			name: "Generic driver with only external IP and no internal IP - neither node-ip nor node-external-ip set",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{},
				ExternalAddresses: []string{"10.0.0.5"},
				DriverName:        "vsphere",
			},
			expectedNodeIPs:         []string{},
			expectedNodeExternalIPs: []string{},
		},
		// When InternalAddresses is empty but an IPv6 address is present, nodeIPs is non-empty
		// after the IPv6 append, so node-external-ip must still be set from ExternalAddresses.
		{
			name: "No internal IP but IPv6 address present: node-external-ip still set",
			initialConfig: map[string]interface{}{
				"node-ip":             []string{},
				"node-external-ip":    []string{},
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{},
				ExternalAddresses: []string{"20.1.2.3"},
				IPv6Address:       "2001:db8::1",
				DriverName:        management.Azuredriver,
			},
			expectedNodeIPs:         []string{"2001:db8::1"},
			expectedNodeExternalIPs: []string{"20.1.2.3"},
		},
		// When all external IPs are already present in node-ip (duplicates), node-external-ip
		// must not be written to the config at all (new len(nodeExternalIPs)>0 guard).
		{
			name: "All external IPs are duplicates of node-ip: node-external-ip key not written",
			initialConfig: map[string]interface{}{
				"cloud-provider-name": "",
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.5"},
				ExternalAddresses: []string{"10.0.0.5"},
			},
			expectedNodeIPs:         []string{"10.0.0.5"},
			expectedNodeExternalIPs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := make(map[string]interface{}, len(tt.initialConfig))
			for k, v := range tt.initialConfig {
				config[k] = v
			}
			updateConfigWithAddresses(config, tt.info)

			gotIPs := convert.ToStringSlice(config["node-ip"])
			if len(tt.expectedNodeIPs) > 0 {
				if !reflect.DeepEqual(gotIPs, tt.expectedNodeIPs) {
					t.Errorf("node-ip mismatch:\n  got  %v\n  want %v", gotIPs, tt.expectedNodeIPs)
				}
			} else {
				if len(gotIPs) > 0 {
					t.Errorf("unexpected node-ip: %v", gotIPs)
				}
			}

			gotExternal := convert.ToStringSlice(config["node-external-ip"])
			if len(tt.expectedNodeExternalIPs) > 0 {
				if !reflect.DeepEqual(gotExternal, tt.expectedNodeExternalIPs) {
					t.Errorf("node-external-ip mismatch:\n  got  %v\n  want %v", gotExternal, tt.expectedNodeExternalIPs)
				}
			} else {
				if len(gotExternal) > 0 {
					t.Errorf("unexpected node-external-ip: %v", gotExternal)
				}
			}
		})
	}
}

func TestUpdateConfigWithAdvertiseAddresses(t *testing.T) {
	controlPlaneEntry := &planEntry{
		Metadata: &plan.Metadata{
			Labels:      map[string]string{capr.ControlPlaneRoleLabel: "true"},
			Annotations: nil,
		},
	}
	workerEntry := &planEntry{
		Metadata: &plan.Metadata{
			Labels:      map[string]string{capr.WorkerRoleLabel: "true"},
			Annotations: nil,
		},
	}

	tests := []struct {
		name           string
		initialConfig  map[string]interface{}
		info           *machineNetworkInfo
		expectAddrSet  bool
		expectedConfig map[string]interface{}
	}{
		{
			name: "control-plane node with different node-ip and node-external-ip",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"10.0.0.1"},
				"node-external-ip": []string{"192.168.1.1"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "10.0.0.1", "tls-san": []string{"192.168.1.1"}},
		},
		{
			name: "control-plane node with same node-ip and node-external-ip",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"192.168.1.1"},
				"node-external-ip": []string{"192.168.1.1"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{"node-ip": []string{"192.168.1.1"}, "node-external-ip": []string{"192.168.1.1"}},
		},
		{
			name: "control-plane node with same set of node-ip and node-external-ip in different order",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"192.168.1.1", "192.168.1.2"},
				"node-external-ip": []string{"192.168.1.2", "192.168.1.1"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{"node-ip": []string{"192.168.1.1", "192.168.1.2"}, "node-external-ip": []string{"192.168.1.2", "192.168.1.1"}},
		},
		{
			name: "control-plane node with empty node-ip returns early",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{},
				"node-external-ip": []string{"192.168.1.1"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{"node-ip": []string{}, "node-external-ip": []string{"192.168.1.1"}},
		},
		{
			name: "control-plane node with empty node-external-ip returns early",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"10.0.0.1"},
				"node-external-ip": []string{},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{"node-ip": []string{"10.0.0.1"}, "node-external-ip": []string{}},
		},
		{
			name:           "control-plane node with absent node-ip returns early",
			initialConfig:  map[string]interface{}{},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name: "worker-only node should be skipped",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"10.0.0.1"},
				"node-external-ip": []string{"192.168.1.1"},
			},
			info:           &machineNetworkInfo{Entry: workerEntry},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{"node-ip": []string{"10.0.0.1"}, "node-external-ip": []string{"192.168.1.1"}},
		},
		{
			name: "control-plane node with multiple different IPs uses first node-ip",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"10.0.0.2", "10.0.0.1"},
				"node-external-ip": []string{"192.168.1.2", "192.168.1.1"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "10.0.0.2", "tls-san": []string{"192.168.1.2", "192.168.1.1"}},
		},
		{
			name: "control-plane node with interface names uses first node-ip",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"eth0"},
				"node-external-ip": []string{"eth1"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "eth0", "tls-san": []string{"eth1"}},
		},
		{
			name: "control-plane node with pre-existing tls-san appends node-external-ip",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"10.0.0.1"},
				"node-external-ip": []string{"192.168.1.1", "192.168.1.2"},
				"tls-san":          []string{"existing-san", "192.168.1.1"},
			},
			info:          &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet: true,
			expectedConfig: map[string]interface{}{
				"advertise-address": "10.0.0.1",
				"tls-san":           []string{"existing-san", "192.168.1.1", "192.168.1.2"},
			},
		},
		{
			name: "IPv6-first node-ip uses first (IPv6) as advertise-address",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"2001:db8::1", "10.0.0.1"},
				"node-external-ip": []string{"1.2.3.4"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "2001:db8::1", "tls-san": []string{"1.2.3.4"}},
		},
		{
			name: "IPv4-first node-ip uses first (IPv4) as advertise-address",
			initialConfig: map[string]interface{}{
				"node-ip":          []string{"10.0.0.1", "2001:db8::1"},
				"node-external-ip": []string{"1.2.3.4"},
			},
			info:           &machineNetworkInfo{Entry: controlPlaneEntry},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "10.0.0.1", "tls-san": []string{"1.2.3.4"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.initialConfig
			updateConfigWithAdvertiseAddresses(config, tt.info)
			_, ok := config["advertise-address"]
			if tt.expectAddrSet {
				assert.True(t, ok, "expected advertise-address to be set")
				// DeepEqual does not work well with mixed slice types (e.g. []string vs []interface{}), so we test tls-san separately.
				expectedTlsSan, ok := tt.expectedConfig["tls-san"]
				if ok {
					assert.ElementsMatch(t, expectedTlsSan, config["tls-san"])
				}
				assert.Equal(t, tt.expectedConfig["advertise-address"], config["advertise-address"])
			} else {
				assert.False(t, ok, "expected advertise-address not to be set")
				assert.Equal(t, len(tt.expectedConfig), len(config))
			}
		})
	}
}

func TestExtractConfigJSON(t *testing.T) {
	tests := []struct {
		name           string
		extracted      string
		expected       map[string]interface{}
		errContains    string
		assertResultFn func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "extracts machine config json from tar.gz archive",
			extracted: buildExtractConfigArchive(t, map[string][]byte{
				"state/machines/node-1/config.json": []byte(`{"DriverName":"amazonec2","Driver":{"IPAddress":"1.2.3.4","PrivateIPAddress":"10.0.0.10"}}`),
				"state/machines/node-1/other.txt":   []byte("ignored"),
			}),
			assertResultFn: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "amazonec2", result["DriverName"])
				driver := result["Driver"].(map[string]interface{})
				assert.Equal(t, "1.2.3.4", driver["IPAddress"])
				assert.Equal(t, "10.0.0.10", driver["PrivateIPAddress"])
			},
		},
		{
			name: "returns empty result when archive has no matching config.json",
			extracted: buildExtractConfigArchive(t, map[string][]byte{
				"state/machines/node-1/not-config.json": []byte(`{"ignored":true}`),
				"state/cluster/config.json":             []byte(`{"ignored":true}`),
			}),
			expected: map[string]interface{}{},
		},
		{
			name:        "fails on invalid base64",
			extracted:   "not-valid-base64",
			errContains: "base64.DecodeString",
		},
		{
			name:        "fails on non-gzip payload",
			extracted:   base64.StdEncoding.EncodeToString([]byte("plain text")),
			errContains: "gzip",
		},
		{
			name:        "fails on invalid tar payload",
			extracted:   buildExtractConfigGzipPayload(t, []byte("invalid tar payload")),
			errContains: "tarRead.Next",
		},
		{
			name: "fails when matching config json is invalid",
			extracted: buildExtractConfigArchive(t, map[string][]byte{
				"state/machines/node-1/config.json": []byte(`{"Driver":`),
			}),
			errContains: "failed to read config.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractConfigJSON(tt.extracted)
			if tt.errContains != "" {
				assert.Error(t, err)
				if err != nil {
					assert.True(t, strings.Contains(err.Error(), tt.errContains), fmt.Sprintf("expected error to contain %q, got %q", tt.errContains, err.Error()))
				}
				return
			}

			assert.NoError(t, err)
			if tt.assertResultFn != nil {
				tt.assertResultFn(t, result)
				return
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func buildExtractConfigArchive(t *testing.T, files map[string][]byte) string {
	t.Helper()

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gz)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func buildExtractConfigGzipPayload(t *testing.T, payload []byte) string {
	t.Helper()

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if _, err := gz.Write(payload); err != nil {
		t.Fatalf("failed to write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip payload writer: %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
