package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-09-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-07-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/mcuadros/go-version"
)

type virtualNetworksResponseBody struct {
	Name          string   `json:"name"`
	ResourceGroup string   `json:"resourceGroup"`
	Subnets       []subnet `json:"subnets"`
}

type subnet struct {
	Name         string `json:"name"`
	AddressRange string `json:"addressRange"`
}

var matchResourceGroup = regexp.MustCompile("/resource[gG]roups/(.+?)/")

func NewClientAuthorizer(cap *Capabilities) (autorest.Authorizer, error) {
	oauthConfig, err := adal.NewOAuthConfig(cap.AuthBaseURL, cap.TenantID)
	if err != nil {
		return nil, err
	}

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, cap.ClientID, cap.ClientSecret, cap.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("couldn't authenticate to Azure cloud with error: %v", err)
	}

	return autorest.NewBearerAuthorizer(spToken), nil
}

func NewContainerServiceClient(cap *Capabilities) (*containerservice.ContainerServicesClient, error) {
	authorizer, err := NewClientAuthorizer(cap)
	if err != nil {
		return nil, err
	}

	containerService := containerservice.NewContainerServicesClientWithBaseURI(cap.BaseURL, cap.SubscriptionID)
	containerService.Authorizer = authorizer

	return &containerService, nil
}

func NewNetworkServiceClient(cap *Capabilities) (*network.VirtualNetworksClient, error) {
	authorizer, err := NewClientAuthorizer(cap)
	if err != nil {
		return nil, err
	}

	containerService := network.NewVirtualNetworksClientWithBaseURI(cap.BaseURL, cap.SubscriptionID)
	containerService.Authorizer = authorizer

	return &containerService, nil
}

func NewClusterClient(cap *Capabilities) (*containerservice.ManagedClustersClient, error) {
	authorizer, err := NewClientAuthorizer(cap)
	if err != nil {
		return nil, err
	}

	client := containerservice.NewManagedClustersClientWithBaseURI(cap.BaseURL, cap.SubscriptionID)
	client.Authorizer = authorizer

	return &client, nil
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

func listKubernetesVersions(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ResourceLocation == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("region is required")
	}

	clientContainer, err := NewContainerServiceClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	orchestrators, err := clientContainer.ListOrchestrators(ctx, cap.ResourceLocation, "managedClusters")
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get orchestrators: %v", err)
	}

	if orchestrators.Orchestrators == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("no version profiles returned: %v", err)
	}

	var kubernetesVersions []string

	for _, profile := range *orchestrators.Orchestrators {
		if profile.OrchestratorType == nil || profile.OrchestratorVersion == nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("unexpected nil orchestrator type or version")
		}

		if *profile.OrchestratorType == "Kubernetes" {
			kubernetesVersions = append(kubernetesVersions, *profile.OrchestratorVersion)
		}
	}

	sort.Sort(sortableVersion(kubernetesVersions))

	return encodeOutput(kubernetesVersions)
}

func listVirtualNetworks(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	clientNetwork, err := NewNetworkServiceClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	networkList, err := clientNetwork.ListAll(ctx)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get networks: %v", err)
	}

	var networks []virtualNetworksResponseBody

	for networkList.NotDone() {
		var batch []virtualNetworksResponseBody

		for _, azureNetwork := range networkList.Values() {
			var subnets []subnet

			if azureNetwork.Subnets != nil {
				for _, azureSubnet := range *azureNetwork.Subnets {
					if azureSubnet.Name != nil {
						subnets = append(subnets, subnet{
							Name:         *azureSubnet.Name,
							AddressRange: *azureSubnet.AddressPrefix,
						})
					}
				}
			}

			if azureNetwork.ID == nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("no ID on virtual network")
			}

			match := matchResourceGroup.FindStringSubmatch(*azureNetwork.ID)

			if len(match) < 2 || match[1] == "" {
				return nil, http.StatusInternalServerError, fmt.Errorf("could not parse virtual network ID")
			}

			if azureNetwork.Name == nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("no name on virtual network")
			}

			batch = append(batch, virtualNetworksResponseBody{
				Name:          *azureNetwork.Name,
				ResourceGroup: match[1],
				Subnets:       subnets,
			})
		}

		networks = append(networks, batch...)

		err = networkList.NextWithContext(ctx)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
	}

	return encodeOutput(networks)
}

type clustersResponseBody struct {
	ResourceGroup string `json:"resourceGroup"`
	ClusterName   string `json:"clusterName"`
}

func listClusters(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	clientCluster, err := NewClusterClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	clusterList, err := clientCluster.List(ctx)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get cluster list: %v", err)
	}

	var clusters []clustersResponseBody

	for clusterList.NotDone() {
		for _, cluster := range clusterList.Values() {
			var tmpCluster clustersResponseBody
			if cluster.ID != nil {
				tmpCluster.ResourceGroup = matchResourceGroup.FindStringSubmatch(to.String(cluster.ID))[1]
			}
			if cluster.Name != nil {
				tmpCluster.ClusterName = to.String(cluster.Name)
			}
			clusters = append(clusters, tmpCluster)
		}

		err = clusterList.NextWithContext(ctx)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
	}

	return encodeOutput(clusters)
}

func encodeOutput(result interface{}) ([]byte, int, error) {
	data, err := json.Marshal(&result)
	if err != nil {
		return data, http.StatusInternalServerError, err
	}

	return data, http.StatusOK, err
}
