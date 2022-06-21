package azure

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types/slice"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	globalAzureADGraphEndpoint = "https://graph.windows.net/"
	globalMSGraphEndpoint      = "https://graph.microsoft.com"
	chinaAzureADGraphEndpoint  = "https://graph.chinacloudapi.cn/"
	chinaMSGraphEndpoint       = "https://microsoftgraph.chinacloudapi.cn"

	chinaAzureADLoginEndpoint = "https://login.chinacloudapi.cn/"
	chinaAzureMSLoginEndpoint = "https://login.partner.microsoftonline.cn/"
)

func authProviderEnabled(config *v32.AzureADConfig) bool {
	return config.Enabled && config.GraphEndpoint != ""
}

func graphEndpointDeprecated(endpoint string) bool {
	deprecatedEndpoints := []string{globalAzureADGraphEndpoint, chinaAzureADGraphEndpoint}
	return slice.ContainsString(deprecatedEndpoints, endpoint)
}

func updateAzureADConfig(c *v32.AzureADConfig) {
	if isConfigForChina(c) {
		updateConfigForChina(c)
	} else {
		updateConfigForGlobal(c)
	}
}

func isConfigForChina(c *v32.AzureADConfig) bool {
	return strings.HasSuffix(c.GraphEndpoint, ".cn") || strings.HasSuffix(c.GraphEndpoint, ".cn/")
}

func updateConfigForGlobal(c *v32.AzureADConfig) {
	if c.GraphEndpoint != globalAzureADGraphEndpoint {
		logrus.Infoln("Refusing to upgrade because the Graph Endpoint is not deprecated.")
		return
	}
	// Update the Graph Endpoint.
	c.GraphEndpoint = globalMSGraphEndpoint
	// Update the Auth Endpoint and Token Endpoint.
	c.AuthEndpoint = fmt.Sprintf("%s%s/oauth2/v2.0/authorize", c.Endpoint, c.TenantID)
	c.TokenEndpoint = fmt.Sprintf("%s%s/oauth2/v2.0/token", c.Endpoint, c.TenantID)
}

func updateConfigForChina(c *v32.AzureADConfig) {
	if c.GraphEndpoint != chinaAzureADGraphEndpoint {
		logrus.Infoln("Refusing to upgrade because the Graph Endpoint is not deprecated.")
		return
	}
	// Update the Graph Endpoint.
	c.GraphEndpoint = chinaMSGraphEndpoint
	// Update the login endpoint.
	if c.Endpoint == chinaAzureADLoginEndpoint {
		c.Endpoint = chinaAzureMSLoginEndpoint
	}
	// Update the Auth Endpoint and Token Endpoint.
	c.AuthEndpoint = fmt.Sprintf("%s%s/oauth2/v2.0/authorize", c.Endpoint, c.TenantID)
	c.TokenEndpoint = fmt.Sprintf("%s%s/oauth2/v2.0/token", c.Endpoint, c.TenantID)
}
