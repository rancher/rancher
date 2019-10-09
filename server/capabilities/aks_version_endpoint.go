package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2017-09-30/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/mcuadros/go-version"
)

func NewAKSVersionsHandler() *AKSVersionHandler {
	return &AKSVersionHandler{}
}

type AKSVersionHandler struct {
}

type regionCapabilitiesRequestBody struct {
	// BaseURL specifies the Azure Resource management endpoint, it defaults "https://management.azure.com/".
	BaseURL string `json:"baseUrl"`
	// AuthBaseURL specifies the Azure OAuth 2.0 authentication endpoint, it defaults "https://login.microsoftonline.com/".
	AuthBaseURL string `json:"authBaseUrl"`
	// credentials
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	SubscriptionID string `json:"subscriptionId"`
	TenantID       string `json:"tenantId"`

	Region string `json:"region"`
}

func (g *AKSVersionHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writer.Header().Set("Content-Type", "application/json")

	var body regionCapabilitiesRequestBody
	if err := extractRequestBody(writer, req, &body); err != nil {
		handleErr(writer, err)
		return
	}

	if err := validateRegionRequestBody(writer, &body); err != nil {
		handleErr(writer, err)
		return
	}

	region := body.Region

	clientID := body.ClientID
	clientSecret := body.ClientSecret
	subscriptionID := body.SubscriptionID
	tenantID := body.TenantID

	baseURL := body.BaseURL
	authBaseURL := body.AuthBaseURL
	if baseURL == "" {
		baseURL = azure.PublicCloud.ResourceManagerEndpoint
	}
	if authBaseURL == "" {
		authBaseURL = azure.PublicCloud.ActiveDirectoryEndpoint
	}

	oAuthConfig, err := adal.NewOAuthConfig(authBaseURL, tenantID)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to configure azure oaith: %v", err))
		return
	}

	spToken, err := adal.NewServicePrincipalToken(*oAuthConfig, clientID, clientSecret, baseURL)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to create token: %v", err))
		return
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	client := containerservice.NewContainerServicesClientWithBaseURI(baseURL, subscriptionID)
	client.Authorizer = authorizer

	orchestrators, err := client.ListOrchestrators(context.Background(), region, "managedClusters")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to get orchestrators: %v", err))
		return
	}

	if orchestrators.Orchestrators == nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("no version profiles returned: %v", err))
		return
	}

	var kubernetesVersions []string

	for _, profile := range *orchestrators.Orchestrators {
		if profile.OrchestratorType == nil || profile.OrchestratorVersion == nil {
			writer.WriteHeader(http.StatusInternalServerError)
			handleErr(writer, fmt.Errorf("unexpected nil orchestrator type or version"))
			return
		}

		if *profile.OrchestratorType == "Kubernetes" {
			kubernetesVersions = append(kubernetesVersions, *profile.OrchestratorVersion)
		}
	}

	sort.Sort(sortableVersion(kubernetesVersions))

	serialized, err := json.Marshal(kubernetesVersions)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	writer.Write(serialized)
}

type sortableVersion []string

func (s sortableVersion) Len() int {
	return len(s)
}

func (s sortableVersion) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

func (s sortableVersion) Less(a, b int) bool {
	return version.Compare(s[a], s[b], "<")
}

func validateRegionRequestBody(writer http.ResponseWriter, body *regionCapabilitiesRequestBody) error {
	toCheck := [][]string{
		{"region", body.Region},
		{"clientID", body.ClientID},
		{"clientSecret", body.ClientSecret},
		{"subscriptionID", body.SubscriptionID},
		{"tenantID", body.TenantID},
	}
	for _, v := range toCheck {
		if v[1] == "" {
			writer.WriteHeader(http.StatusBadRequest)
			return fmt.Errorf("invalid %s", v[0])
		}
	}

	return nil
}
