package rancher

import (
	"testing"

	"github.com/rancher/rancher/pkg/buildconfig"
	"github.com/rancher/rancher/pkg/settings"
)

func TestEffectiveWebhookVersion(t *testing.T) {
	// Resolve the env key once
	envKey := settings.GetEnvKey(settings.RancherWebhookVersion.Name)

	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "runtime env override",
			envValue: "110.0.0+up0.12.0",
			expected: "110.0.0+up0.12.0",
		},
		{
			name:     "buildconfig fallback when env not set",
			envValue: "",
			expected: buildconfig.WebhookVersion,
		},
		{
			name:     "custom version format",
			envValue: "115.0.0+up1.0.0",
			expected: "115.0.0+up1.0.0",
		},
		{
			name:     "empty string env var is treated as not set",
			envValue: "",
			expected: buildconfig.WebhookVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Guard against env var poisoning: unconditionally set the env var
			// t.Setenv automatically restores the original value after the test
			t.Setenv(envKey, tt.envValue)

			// Call the actual implementation
			got := effectiveWebhookVersion()

			// Verify the result
			if got != tt.expected {
				t.Errorf("effectiveWebhookVersion() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestGetEnvKeyFormat(t *testing.T) {
	// Verify that GetEnvKey generates the expected env var name
	envKey := settings.GetEnvKey(settings.RancherWebhookVersion.Name)
	expected := "CATTLE_RANCHER_WEBHOOK_VERSION"

	if envKey != expected {
		t.Errorf("GetEnvKey(%q) = %q, expected %q",
			settings.RancherWebhookVersion.Name, envKey, expected)
	}
}
