package planner

import (
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/stretchr/testify/assert"
)

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
			name:          "control-plane node with different internal and external IPs",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.1"},
				ExternalAddresses: []string{"192.168.1.1"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "10.0.0.1", "tls-san": []string{"192.168.1.1"}},
		},
		{
			name:          "control-plane node with same internal and external IPs",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"192.168.1.1"},
				ExternalAddresses: []string{"192.168.1.1"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name:          "control-plane node with same set of internal and external IPs",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"192.168.1.1", "192.168.1.2"},
				ExternalAddresses: []string{"192.168.1.2", "192.168.1.1"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name:          "control-plane node with no internal IP",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{},
				ExternalAddresses: []string{"192.168.1.1"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name:          "control-plane node with no external IP",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.1"},
				ExternalAddresses: []string{},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name:          "control-plane node with no internal or external IPs",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{},
				ExternalAddresses: []string{},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name:          "worker-only node should be skipped",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.1"},
				ExternalAddresses: []string{"192.168.1.1"},
				Entry:             workerEntry,
			},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name:          "control-plane node with multiple different IPs",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.2", "10.0.0.1"},
				ExternalAddresses: []string{"192.168.1.2", "192.168.1.1"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "10.0.0.2", "tls-san": []string{"192.168.1.2", "192.168.1.1"}},
		},
		{
			name:          "control-plane node with different internal and external interface names",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"eth0"},
				ExternalAddresses: []string{"eth1"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "eth0", "tls-san": []string{"eth1"}},
		},
		{
			name:          "control-plane node with same internal and external interface names",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"eth0"},
				ExternalAddresses: []string{"eth0"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  false,
			expectedConfig: map[string]interface{}{},
		},
		{
			name:          "control-plane node with mixed different ip and interface name",
			initialConfig: map[string]interface{}{},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"eth0"},
				ExternalAddresses: []string{"192.168.1.1"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet:  true,
			expectedConfig: map[string]interface{}{"advertise-address": "eth0", "tls-san": []string{"192.168.1.1"}},
		},
		{
			name: "control-plane node with pre-existing tls-san",
			initialConfig: map[string]interface{}{
				"tls-san": []string{"existing-san", "192.168.1.1"},
			},
			info: &machineNetworkInfo{
				InternalAddresses: []string{"10.0.0.1"},
				ExternalAddresses: []string{"192.168.1.1", "192.168.1.2"},
				Entry:             controlPlaneEntry,
			},
			expectAddrSet: true,
			expectedConfig: map[string]interface{}{"advertise-address": "10.0.0.1",
				"tls-san": []string{"existing-san", "192.168.1.1", "192.168.1.2"}},
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
