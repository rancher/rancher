package azure

import (
	"fmt"
	"strings"

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

// GraphEndpointMigratedAnnotation is the main piece of data based on which Rancher decides to use either the
// deprecated authentication flow via Azure AD Graph or the new one via Microsoft Graph.
// If the annotation is missing on the Auth Config object, or is present with a value of anything other than "true",
// then Rancher uses the old, deprecated flow. If the annotation is present and set to "true", Rancher uses the new flow.
const GraphEndpointMigratedAnnotation = "auth.cattle.io/azuread-endpoint-migrated"

func authProviderEnabled(config *v32.AzureADConfig) bool {
	return config.Enabled && config.GraphEndpoint != ""
}

// IsConfigDeprecated returns true if a given Azure AD auth config specifies the old,
// deprecated authentication flow via the Azure AD Graph.
func IsConfigDeprecated(cfg *v32.AzureADConfig) bool {
	return authProviderEnabled(cfg) && !configHasNewFlowAnnotation(cfg)
}

func configHasNewFlowAnnotation(cfg *v32.AzureADConfig) bool {
	if cfg.ObjectMeta.Annotations != nil {
		return cfg.ObjectMeta.Annotations[GraphEndpointMigratedAnnotation] == "true"
	}
	return false
}

func updateAzureADEndpoints(c *v32.AzureADConfig) {
	if isConfigForChina(c) {
		updateEndpointsForChina(c)
	} else {
		updateEndpointsForGlobal(c)
	}
}

func isConfigForChina(c *v32.AzureADConfig) bool {
	return strings.HasSuffix(c.GraphEndpoint, ".cn") || strings.HasSuffix(c.GraphEndpoint, ".cn/")
}

func updateEndpointsForGlobal(c *v32.AzureADConfig) {
	if c.GraphEndpoint != globalAzureADGraphEndpoint {
		logrus.Infof("Refusing to upgrade because the Graph Endpoint %s is not deprecated.", c.GraphEndpoint)
		return
	}
	// Update the Graph Endpoint.
	c.GraphEndpoint = globalMSGraphEndpoint
	// Update the Auth Endpoint and Token Endpoint.
	c.AuthEndpoint = fmt.Sprintf("%s%s/oauth2/v2.0/authorize", c.Endpoint, c.TenantID)
	c.TokenEndpoint = fmt.Sprintf("%s%s/oauth2/v2.0/token", c.Endpoint, c.TenantID)
}

func updateEndpointsForChina(c *v32.AzureADConfig) {
	if c.GraphEndpoint != chinaAzureADGraphEndpoint {
		logrus.Infof("Refusing to upgrade because the Graph Endpoint %s is not deprecated.", c.GraphEndpoint)
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
