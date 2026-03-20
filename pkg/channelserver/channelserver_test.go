package channelserver

import (
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetURLAndInterval(t *testing.T) {
	original := settings.RkeMetadataConfig.Get()
	defer func() {
		require.NoError(t, settings.RkeMetadataConfig.Set(original))
	}()

	tests := []struct {
		name             string
		config           string
		expectedURL      string
		expectedInterval time.Duration
	}{
		{
			name:             "valid URL and interval",
			config:           `{"url":"https://releases.rancher.com/kontainer-driver-metadata/dev-v2.14/data.json","refresh-interval-minutes":"1440"}`,
			expectedURL:      "https://releases.rancher.com/kontainer-driver-metadata/dev-v2.14/data.json",
			expectedInterval: 1440 * time.Minute,
		},
		{
			name:             "custom URL and interval",
			config:           `{"url":"https://example.com/data.json","refresh-interval-minutes":"60"}`,
			expectedURL:      "https://example.com/data.json",
			expectedInterval: 60 * time.Minute,
		},
		{
			name:             "zero interval defaults to 1440",
			config:           `{"url":"https://example.com/data.json","refresh-interval-minutes":"0"}`,
			expectedURL:      "https://example.com/data.json",
			expectedInterval: 1440 * time.Minute,
		},
		{
			name:             "negative interval defaults to 1440",
			config:           `{"url":"https://example.com/data.json","refresh-interval-minutes":"-5"}`,
			expectedURL:      "https://example.com/data.json",
			expectedInterval: 1440 * time.Minute,
		},
		{
			name:             "invalid JSON returns empty string and zero duration",
			config:           "not-valid-json",
			expectedURL:      "",
			expectedInterval: 0,
		},
		{
			name:             "empty URL field",
			config:           `{"url":"","refresh-interval-minutes":"1440"}`,
			expectedURL:      "",
			expectedInterval: 1440 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, settings.RkeMetadataConfig.Set(tt.config))
			gotURL, gotInterval := GetURLAndInterval()
			assert.Equal(t, tt.expectedURL, gotURL)
			assert.Equal(t, tt.expectedInterval, gotInterval)
		})
	}
}

func TestDynamicSourceURL(t *testing.T) {
	originalUseLocalData := settings.KDMUseLocalData.Get()
	originalRkeMetadataConfig := settings.RkeMetadataConfig.Get()
	defer func() {
		require.NoError(t, settings.KDMUseLocalData.Set(originalUseLocalData))
		require.NoError(t, settings.RkeMetadataConfig.Set(originalRkeMetadataConfig))
	}()

	const (
		remoteURL = "https://releases.rancher.com/kontainer-driver-metadata/dev-v2.14/data.json"
		localFile = "/var/lib/rancher-data/driver-metadata/data.json"
	)

	require.NoError(t, settings.RkeMetadataConfig.Set(
		`{"url":"` + remoteURL + `","refresh-interval-minutes":"1440"}`,
	))

	tests := []struct {
		name         string
		useLocalData string
		expectedURL  string
	}{
		{
			name:         "returns remote URL when kdm-use-local-data is false",
			useLocalData: "false",
			expectedURL:  remoteURL,
		},
		{
			name:         "returns local file path when kdm-use-local-data is true",
			useLocalData: "true",
			expectedURL:  localFile,
		},
		{
			name:         "returns remote URL when kdm-use-local-data is empty string",
			useLocalData: "",
			expectedURL:  remoteURL,
		},
	}

	d := &DynamicSource{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, settings.KDMUseLocalData.Set(tt.useLocalData))
			assert.Equal(t, tt.expectedURL, d.URL())
		})
	}
}
