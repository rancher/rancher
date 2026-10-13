package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyConfigName(t *testing.T) {
	tests := []struct {
		name            string
		inputConfigName string
		configName      string
		want            string
	}{
		{name: "configName provided", inputConfigName: "azure-eu", configName: "other", want: "azure-eu"},
		{name: "falls back to the config's name", inputConfigName: "", configName: "azure-eu", want: "azure-eu"},
		{name: "falls back to the default config", inputConfigName: "", configName: "", want: ProviderName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, applyConfigName(tt.inputConfigName, tt.configName))
		})
	}
}
