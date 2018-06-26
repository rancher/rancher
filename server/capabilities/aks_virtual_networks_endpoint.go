package capabilities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-05-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

var regex = regexp.MustCompile("/resourceGroups/(.+?)/")

func NewAKSVirtualNetworksHandler() *AKSVirtualNetworksHandler {
	return &AKSVirtualNetworksHandler{}
}

type AKSVirtualNetworksHandler struct {
}

type virtualNetworksRequestBody struct {
	// credentials
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	SubscriptionID string `json:"subscriptionId"`
	TenantID       string `json:"tenantId"`
}

type virtualNetworksResponseBody struct {
	Name          string   `json:"name"`
	ResourceGroup string   `json:"resourceGroup"`
	Subnets       []string `json:"subnets"`
}

func (g *AKSVirtualNetworksHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writer.Header().Set("Content-Type", "application/json")

	var body virtualNetworksRequestBody
	if err := extractRequestBody(writer, req, &body); err != nil {
		handleErr(writer, err)
		return
	}

	if err := validateVirtualNetworksRequestBody(&body); err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, err)
		return
	}

	clientID := body.ClientID
	clientSecret := body.ClientSecret
	subscriptionID := body.SubscriptionID
	tenantID := body.TenantID

	oAuthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to configure azure oauth: %v", err))
		return
	}

	spToken, err := adal.NewServicePrincipalToken(*oAuthConfig, clientID, clientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to create token: %v", err))
		return
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	client := network.NewVirtualNetworksClient(subscriptionID)
	client.Authorizer = authorizer

	var networks []virtualNetworksResponseBody

	pointer, err := client.ListAll(context.Background())
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("failed to get networks: %v", err))
		return
	}

	for pointer.NotDone() {
		var batch []virtualNetworksResponseBody

		for _, azureNetwork := range pointer.Values() {
			var subnets []string

			if azureNetwork.Subnets != nil {
				for _, subnet := range *azureNetwork.Subnets {
					if subnet.Name != nil {
						subnets = append(subnets, *subnet.Name)
					}
				}
			}

			if azureNetwork.ID == nil {
				writer.WriteHeader(http.StatusInternalServerError)
				handleErr(writer, errors.New("no ID on virtual network"))
				return
			}

			match := regex.FindStringSubmatch(*azureNetwork.ID)

			if len(match) < 2 || match[1] == "" {
				writer.WriteHeader(http.StatusInternalServerError)
				handleErr(writer, errors.New("could not parse virtual network ID"))
				return
			}

			if azureNetwork.Name == nil {
				writer.WriteHeader(http.StatusInternalServerError)
				handleErr(writer, errors.New("no name on virtual network"))
				return
			}

			batch = append(batch, virtualNetworksResponseBody{
				Name:          *azureNetwork.Name,
				ResourceGroup: match[1],
				Subnets:       subnets,
			})
		}

		networks = append(networks, batch...)

		err = pointer.Next()
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			handleErr(writer, err)
			return
		}
	}

	serialized, err := json.Marshal(networks)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	writer.Write(serialized)
}

func validateVirtualNetworksRequestBody(body *virtualNetworksRequestBody) error {
	if body.ClientID == "" {
		return fmt.Errorf("invalid clientID")
	}

	if body.ClientSecret == "" {
		return fmt.Errorf("invalid clientSecret")
	}

	if body.SubscriptionID == "" {
		return fmt.Errorf("invalid subscriptionID")
	}

	if body.TenantID == "" {
		return fmt.Errorf("invalid tenantID")
	}

	return nil
}
