package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2017-09-30/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/mcuadros/go-version"
	"net/http"
	"sort"
)

func NewAKSVersionsHandler() *AKSVersionHandler {
	return &AKSVersionHandler{}
}

type AKSVersionHandler struct {
}

type regionCapabilitiesRequestBody struct {
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

	oAuthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to configure azure oaith: %v", err))
		return
	}

	spToken, err := adal.NewServicePrincipalToken(*oAuthConfig, clientID, clientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to create token: %v", err))
		return
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	client := containerservice.NewContainerServicesClient(subscriptionID)
	client.Authorizer = authorizer

	orchestrators, err := client.ListOrchestrators(context.Background(), region, "")
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
	region := body.Region

	clientID := body.ClientID
	clientSecret := body.ClientSecret
	subscriptionID := body.SubscriptionID
	tenantID := body.TenantID

	if region == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid region")
	}

	if clientID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid clientID")
	}

	if clientSecret == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid clientSecret")
	}

	if subscriptionID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid subscriptionID")
	}

	if tenantID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid tenantID")
	}

	return nil
}
