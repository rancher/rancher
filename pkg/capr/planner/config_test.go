package planner

import (
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
)

func TestUpdateConfigWithAddresses(t *testing.T) {
	tests := []struct {
		name                      string
		initialConfig             map[string]interface{}
		info                      *machineNetworkInfo
		onlyWorker                bool
		expectedNodeIPs           []string
		expectedNodeExternalIPs   []string
		expectedAdvertiseAddress  string
		expectedTLSSANs           []string
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"10.0.0.5", "2001:db8::1"},
			expectedNodeExternalIPs:  []string{"1.2.3.4"},
			expectedAdvertiseAddress: "10.0.0.5",
			expectedTLSSANs:          []string{"1.2.3.4"},
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"10.0.0.5"},
			expectedNodeExternalIPs:  []string{"1.2.3.4"},
			expectedAdvertiseAddress: "10.0.0.5",
			expectedTLSSANs:          []string{"1.2.3.4"},
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
			onlyWorker:              false,
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"10.1.2.3"},
			expectedNodeExternalIPs:  []string{"203.0.113.1"},
			expectedAdvertiseAddress: "10.1.2.3",
			expectedTLSSANs:          []string{"203.0.113.1"},
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
			onlyWorker:              false,
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"10.1.2.3", "2001:db8::1"},
			expectedNodeExternalIPs:  []string{"203.0.113.1"},
			expectedAdvertiseAddress: "10.1.2.3",
			expectedTLSSANs:          []string{"203.0.113.1"},
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
			onlyWorker:              false,
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"10.0.0.7", "2001:db8::7"},
			expectedNodeExternalIPs:  []string{},
			expectedAdvertiseAddress: "10.0.0.7",
			expectedTLSSANs:          []string{"203.0.113.5"},
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"10.0.0.5", "10.0.0.6", "2001:db8::1"},
			expectedNodeExternalIPs:  []string{"1.2.3.4", "1.2.3.5"},
			expectedAdvertiseAddress: "10.0.0.5",
			expectedTLSSANs:          []string{"1.2.3.4", "1.2.3.5"},
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"2001:db8::2", "10.0.0.6"},
			expectedNodeExternalIPs:  []string{"1.2.3.4"},
			expectedAdvertiseAddress: "2001:db8::2",
			expectedTLSSANs:          []string{"1.2.3.4"},
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"2001:db8::2", "2001:db8::3"},
			expectedNodeExternalIPs:  []string{"1.2.3.4"},
			expectedAdvertiseAddress: "2001:db8::2",
			expectedTLSSANs:          []string{"1.2.3.4"},
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
			onlyWorker:              false,
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
			onlyWorker:               false,
			expectedNodeIPs:          []string{"1.2.3.4", "1.2.3.5", "1.2.3.7", "2001:db8::1"},
			expectedNodeExternalIPs:  []string{"1.2.3.6"},
			expectedAdvertiseAddress: "1.2.3.4",
			expectedTLSSANs:          []string{"1.2.3.6"},
		},
		{
			name: "Worker-only node does not set advertise-address",
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
			onlyWorker:              true,
			expectedNodeIPs:         []string{"10.0.0.5"},
			expectedNodeExternalIPs: []string{"1.2.3.4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := make(map[string]interface{}, len(tt.initialConfig))
			for k, v := range tt.initialConfig {
				config[k] = v
			}
			updateConfigWithAddresses(config, tt.info, tt.onlyWorker)

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

			gotAdvertiseAddr := convert.ToString(config["advertise-address"])
			if tt.expectedAdvertiseAddress != "" {
				if gotAdvertiseAddr != tt.expectedAdvertiseAddress {
					t.Errorf("advertise-address mismatch:\n  got  %v\n  want %v", gotAdvertiseAddr, tt.expectedAdvertiseAddress)
				}
			} else {
				if gotAdvertiseAddr != "" {
					t.Errorf("unexpected advertise-address: %v", gotAdvertiseAddr)
				}
			}

			gotTLSSANs := convert.ToStringSlice(config["tls-san"])
			if len(tt.expectedTLSSANs) > 0 {
				if !reflect.DeepEqual(gotTLSSANs, tt.expectedTLSSANs) {
					t.Errorf("tls-san mismatch:\n  got  %v\n  want %v", gotTLSSANs, tt.expectedTLSSANs)
				}
			} else {
				if len(gotTLSSANs) > 0 {
					t.Errorf("unexpected tls-san: %v", gotTLSSANs)
				}
			}
		})
	}
}
