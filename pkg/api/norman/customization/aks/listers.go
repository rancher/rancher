package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-09-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-07-01/network"
	"github.com/Azure/azure-sdk-for-go/services/subscription/mgmt/2020-09-01/subscription"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/mcuadros/go-version"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type virtualNetworksResponseBody struct {
	Name          string   `json:"name"`
	ResourceGroup string   `json:"resourceGroup"`
	Subnets       []subnet `json:"subnets"`
	Location      string   `json:"location"`
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

func NewVirtualMachineClient(cap *Capabilities) (*compute.VirtualMachineSizesClient, error) {
	authorizer, err := NewClientAuthorizer(cap)
	if err != nil {
		return nil, err
	}

	virtualMachine := compute.NewVirtualMachineSizesClient(cap.SubscriptionID)
	virtualMachine.Authorizer = authorizer

	return &virtualMachine, nil
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

func NewSubscriptionServiceClient(cap *Capabilities) (*subscription.SubscriptionsClient, error) {
	authorizer, err := NewClientAuthorizer(cap)
	if err != nil {
		return nil, err
	}

	subscriptionService := subscription.NewSubscriptionsClient()
	subscriptionService.Authorizer = authorizer

	return &subscriptionService, nil
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

type KubernetesUpgradeVersion struct {
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

type KubernetesUpgradeVersions []*KubernetesUpgradeVersion

func (s KubernetesUpgradeVersions) Len() int {
	return len(s)
}

func (s KubernetesUpgradeVersions) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

func (s KubernetesUpgradeVersions) Less(a, b int) bool {
	return version.Compare(s[a].Version, s[b].Version, "<")
}

type UpgradeVersionsResponse struct {
	CurrentVersion string                    `json:"currentVersion"`
	Upgrades       KubernetesUpgradeVersions `json:"upgrades"`
}

// listKubernetesUpgradeVersions lists all kubernetes versions listed by AKS Container Service and marks which ones the
// given cluster can be upgraded to.  A version's `Enabled` flag is true if the cluster can be upgraded to the version
// in its current state.
func listKubernetesUpgradeVersions(ctx context.Context, clusterLister mgmtv3.ClusterCache, cap *Capabilities) ([]byte, int, error) {
	var resp UpgradeVersionsResponse

	// load the target cluster, if the cluster is not found we cannot proceed
	cluster, err := clusterLister.Get(cap.ClusterID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid cluster id")
	}

	resp.CurrentVersion = *cluster.Spec.AKSConfig.KubernetesVersion

	// get the client for aks container service
	clientContainer, err := NewContainerServiceClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// request a list of orchestrators
	orchestrators, err := clientContainer.ListOrchestrators(ctx, cap.ResourceLocation, "managedClusters")
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get orchestrators: %v", err)
	}

	// ensure the orchestrators are returned
	if orchestrators.Orchestrators == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("no version profiles returned: %v", err)
	}

	var upgradeVersions map[string]bool
	for _, profile := range *orchestrators.Orchestrators {
		// check for nil pointers to avoid a panic
		if profile.OrchestratorType == nil || profile.OrchestratorVersion == nil {
			logrus.Warning("unexpected nil orchestrator type or version")
			continue
		}

		// exclude any non kubernetes types
		if containerservice.OrchestratorTypes(*profile.OrchestratorType) != containerservice.Kubernetes {
			continue
		}

		// exclude any versions older than the current version
		if version.Compare(*profile.OrchestratorVersion, resp.CurrentVersion, "<") {
			continue
		}

		// generate the upgrade map when the current version is found
		if *profile.OrchestratorVersion == resp.CurrentVersion {
			upgradeVersions = upgradeableVersionsMap(profile)
			continue
		}

		// store this kubernetes version
		resp.Upgrades = append(resp.Upgrades, &KubernetesUpgradeVersion{Version: *profile.OrchestratorVersion})
	}

	// enable any version listed in the upgrade versions
	for _, kubernetesVersion := range resp.Upgrades {
		if upgradeVersions[kubernetesVersion.Version] {
			kubernetesVersion.Enabled = true
		}
	}

	// sort versions
	sort.Sort(resp.Upgrades)

	return encodeOutput(resp)
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
			if cap.ResourceLocation != "" && to.String(azureNetwork.Location) != cap.ResourceLocation {
				continue
			}
			var subnets []subnet

			if azureNetwork.Subnets != nil {
				for _, azureSubnet := range *azureNetwork.Subnets {
					if azureSubnet.Name != nil {
						subnets = append(subnets, subnet{
							Name:         to.String(azureSubnet.Name),
							AddressRange: to.String(azureSubnet.AddressPrefix),
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
				Name:          to.String(azureNetwork.Name),
				ResourceGroup: match[1],
				Subnets:       subnets,
				Location:      to.String(azureNetwork.Location),
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
	RBACEnabled   bool   `json:"rbacEnabled"`
	Location      string `json:"location"`
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
			tmpCluster := clustersResponseBody{
				ClusterName: to.String(cluster.Name),
				RBACEnabled: to.Bool(cluster.EnableRBAC),
				Location:    to.String(cluster.Location),
			}
			if cluster.ID != nil {
				match := matchResourceGroup.FindStringSubmatch(to.String(cluster.ID))
				if len(match) < 2 || match[1] == "" {
					return nil, http.StatusInternalServerError, fmt.Errorf("could not parse virtual network ID")
				}
				tmpCluster.ResourceGroup = match[1]
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

func listVMSizes(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ResourceLocation == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("region is required")
	}

	virtualMachine, err := NewVirtualMachineClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	vmMachineSizeList, err := virtualMachine.List(ctx, cap.ResourceLocation)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get VM sizes: %v", err)
	}

	vmSizes := make([]string, 0, len(*vmMachineSizeList.Value))

	for _, virtualMachineSize := range *vmMachineSizeList.Value {
		vmSizes = append(vmSizes, to.String(virtualMachineSize.Name))
	}

	return encodeOutput(vmSizes)
}

type locationsResponseBody struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

func listLocations(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	clientSubscription, err := NewSubscriptionServiceClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	locationList, err := clientSubscription.ListLocations(ctx, cap.SubscriptionID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get locations: %v", err)
	}

	var locations []locationsResponseBody

	for _, location := range *locationList.Value {
		locations = append(locations, locationsResponseBody{
			Name:        to.String(location.Name),
			DisplayName: to.String(location.DisplayName),
		})
	}

	return encodeOutput(locations)
}

func encodeOutput(result interface{}) ([]byte, int, error) {
	data, err := json.Marshal(&result)
	if err != nil {
		return data, http.StatusInternalServerError, err
	}

	return data, http.StatusOK, err
}

func upgradeableVersionsMap(upgradeProfile containerservice.OrchestratorVersionProfile) map[string]bool {
	rval := make(map[string]bool, 0)

	if upgradeProfile.Upgrades == nil {
		// already on latest version, no upgrades available
		return rval
	}
	for _, profile := range *upgradeProfile.Upgrades {
		rval[*profile.OrchestratorVersion] = true
	}

	return rval
}
