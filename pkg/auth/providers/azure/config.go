package azure

import (
	"strings"

	"github.com/rancher/norman/types/slice"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

const (
	globalAzureADGraphEndpoint = "https://graph.windows.net/"
	globalMSGraphEndpoint      = "https://graph.microsoft.com"
	chinaAzureADGraphEndpoint  = "https://graph.chinacloudapi.cn/"
	chinaMSGraphEndpoint       = "https://microsoftgraph.chinacloudapi.cn"
)

func authProviderEnabled(config *v32.AzureADConfig) bool {
	return config.Enabled && config.GraphEndpoint != ""
}

func graphEndpointDeprecated(endpoint string) bool {
	deprecatedEndpoints := []string{globalAzureADGraphEndpoint, chinaAzureADGraphEndpoint}
	return slice.ContainsString(deprecatedEndpoints, endpoint)
}

func updateAzureADConfig(c *v32.AzureADConfig) {
	// Update the Graph Endpoint.
	graphEndpointMigration := map[string]string{
		globalAzureADGraphEndpoint: globalMSGraphEndpoint,
		chinaAzureADGraphEndpoint:  chinaMSGraphEndpoint,
	}
	// This will be a no-op if the config already uses the new endpoint.
	if newEndpoint, oldUsed := graphEndpointMigration[c.GraphEndpoint]; oldUsed {
		c.GraphEndpoint = newEndpoint
	}

	// Update the Auth Endpoint and Token Endpoint.
	c.AuthEndpoint = strings.Replace(c.AuthEndpoint, "/oauth2/authorize", "/oauth2/v2.0/authorize", 1)
	c.TokenEndpoint = strings.Replace(c.TokenEndpoint, "/oauth2/token", "/oauth2/v2.0/token", 1)
}
