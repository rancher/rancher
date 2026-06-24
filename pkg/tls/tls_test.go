package tls

import (
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
)

// TestFilterCN covers all branches of filterCN, which gates dynamic CN
// additions to the serving-cert (:443) listener.
//
// filterCN is the func(...string)[]string passed to dynamiclistener as
// FilterCN.  allowDefaultSANs (inside dynamiclistener) already short-circuits
// CNs that are in Config.SANs, so filterCN only ever receives the *unknown*
// (dynamically presented) ones.
func TestFilterCN(t *testing.T) {
	tests := []struct {
		name      string
		mcmAgent  bool
		serverURL string
		input     []string
		expected  []string
	}{
		{
			name:     "MCMAgent enabled — reject all dynamic CNs regardless of serverURL",
			mcmAgent: true,
			input:    []string{"attacker.evil.com", "10.0.0.5"},
			expected: nil,
		},
		{
			name:      "MCMAgent enabled, empty serverURL — still returns nil",
			mcmAgent:  true,
			serverURL: "",
			input:     []string{"anything.com"},
			expected:  nil,
		},
		{
			name:      "MCMAgent disabled, empty serverURL — pass-through (pre-bootstrap)",
			mcmAgent:  false,
			serverURL: "",
			input:     []string{"anything.com", "10.0.0.5"},
			expected:  []string{"anything.com", "10.0.0.5"},
		},
		{
			name:      "MCMAgent disabled, serverURL set — only server hostname allowed",
			mcmAgent:  false,
			serverURL: "https://rancher.example.com",
			input:     []string{"attacker.evil.com"},
			expected:  []string{"rancher.example.com"},
		},
		{
			name:      "MCMAgent disabled, serverURL with port — hostname without port returned",
			mcmAgent:  false,
			serverURL: "https://rancher.example.com:8443",
			input:     []string{"attacker.evil.com"},
			expected:  []string{"rancher.example.com"},
		},
		{
			name:      "MCMAgent disabled, unparseable serverURL — pass-through",
			mcmAgent:  false,
			serverURL: "://bad-url",
			input:     []string{"attacker.evil.com"},
			expected:  []string{"attacker.evil.com"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// filterCN reads global state; do not run subtests in parallel.
			features.MCMAgent.Set(tt.mcmAgent)
			t.Cleanup(func() { features.MCMAgent.Set(false) })

			_ = settings.ServerURL.Set(tt.serverURL)
			t.Cleanup(func() { _ = settings.ServerURL.Set("") })

			got := filterCN(tt.input...)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("filterCN(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
