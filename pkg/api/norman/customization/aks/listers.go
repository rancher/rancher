package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	azcoreto "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
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

func NewClientSecretCredential(cap *Capabilities) (*azidentity.ClientSecretCredential, error) {
	cloud, _ := GetEnvironment(cap.Environment)

	return azidentity.NewClientSecretCredential(cap.TenantID, cap.ClientID, cap.ClientSecret, &azidentity.ClientSecretCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cloud,
		},
	})
}

func NewVirtualMachineSKUClient(cap *Capabilities) (*armcompute.ResourceSKUsClient, error) {
	cloud, _ := GetEnvironment(cap.Environment)

	cred, err := NewClientSecretCredential(cap)
	if err != nil {
		return nil, err
	}

	options := &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Cloud: cloud,
		},
	}

	clientFactory, err := armcompute.NewClientFactory(cap.SubscriptionID, cred, options)
	if err != nil {
		return nil, err
	}

	return clientFactory.NewResourceSKUsClient(), nil
}

func NewVirtualMachineSizesClient(cap *Capabilities) (*armcompute.VirtualMachineSizesClient, error) {
	cloud, _ := GetEnvironment(cap.Environment)

	cred, err := NewClientSecretCredential(cap)
	if err != nil {
		return nil, err
	}

	clientFactory, err := armcompute.NewClientFactory(cap.SubscriptionID, cred, &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cloud,
		},
	})
	if err != nil {
		return nil, err
	}

	return clientFactory.NewVirtualMachineSizesClient(), nil
}

func NewNetworkServiceClient(cap *Capabilities) (*armnetwork.VirtualNetworksClient, error) {
	cloud, _ := GetEnvironment(cap.Environment)

	cred, err := NewClientSecretCredential(cap)
	if err != nil {
		return nil, err
	}

	options := &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Cloud: cloud,
		},
	}

	clientFactory, err := armnetwork.NewClientFactory(cap.SubscriptionID, cred, options)
	if err != nil {
		return nil, err
	}

	return clientFactory.NewVirtualNetworksClient(), nil
}

func NewManagedClustersClient(cap *Capabilities) (*armcontainerservice.ManagedClustersClient, error) {
	cloud, _ := GetEnvironment(cap.Environment)

	cred, err := NewClientSecretCredential(cap)
	if err != nil {
		return nil, err
	}

	options := &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Cloud: cloud,
		},
	}

	clientFactory, err := armcontainerservice.NewClientFactory(cap.SubscriptionID, cred, options)
	if err != nil {
		return nil, err
	}

	return clientFactory.NewManagedClustersClient(), nil
}

func NewSubscriptionServiceClient(cap *Capabilities) (*armsubscription.SubscriptionsClient, error) {
	cloud, _ := GetEnvironment(cap.Environment)

	cred, err := NewClientSecretCredential(cap)
	if err != nil {
		return nil, err
	}

	options := &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Cloud: cloud,
		},
	}

	clientFactory, err := armsubscription.NewClientFactory(cred, options)
	if err != nil {
		return nil, err
	}

	return clientFactory.NewSubscriptionsClient(), nil
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

	if cluster.Spec.AKSConfig.KubernetesVersion != nil {
		resp.CurrentVersion = *cluster.Spec.AKSConfig.KubernetesVersion
	} else {
		if cluster.Status.AKSStatus.UpstreamSpec == nil || cluster.Status.AKSStatus.UpstreamSpec.KubernetesVersion == nil {
			return nil, http.StatusBadRequest, fmt.Errorf("kubernetes version of the cluster cannot be determined")
		}
		resp.CurrentVersion = *cluster.Status.AKSStatus.UpstreamSpec.KubernetesVersion
	}

	// get the client for aks container service
	client, err := NewManagedClustersClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	res, err := client.ListKubernetesVersions(ctx, cap.ResourceLocation, nil)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get Kubernetes versions: %w", err)
	}

	if len(res.Values) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("no versions were returned: %w", err)
	}

	var upgradeVersions map[string]bool
	for _, v := range res.Values {
		if v == nil {
			logrus.Warning("unexpected nil version")
			continue
		}

		for patchVersion, upgrades := range v.PatchVersions {
			// exclude any versions older than the current version
			if version.Compare(patchVersion, resp.CurrentVersion, "<") {
				continue
			}

			// generate the upgrade map when the current version is found
			if patchVersion == resp.CurrentVersion {
				upgradeVersions = upgradeableVersionsMap(upgrades)
				continue
			}

			// store this kubernetes version
			resp.Upgrades = append(resp.Upgrades, &KubernetesUpgradeVersion{Version: patchVersion})
		}
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

	client, err := NewManagedClustersClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	res, err := client.ListKubernetesVersions(ctx, cap.ResourceLocation, nil)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get Kubernetes versions: %w", err)
	}

	if len(res.Values) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("no versions were returned: %w", err)
	}

	var kubernetesVersions []string
	for _, v := range res.Values {
		if v.Capabilities.SupportPlan != nil {
			for _, supportPlan := range v.Capabilities.SupportPlan {
				if *supportPlan == armcontainerservice.KubernetesSupportPlanKubernetesOfficial {
					for version := range v.PatchVersions {
						kubernetesVersions = append(kubernetesVersions, version)
					}
				}
			}
		}
	}

	sort.Sort(sortableVersion(kubernetesVersions))

	return encodeOutput(kubernetesVersions)
}

func listVirtualNetworks(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	client, err := NewNetworkServiceClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	var networks []virtualNetworksResponseBody
	pager := client.NewListAllPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get networks: %w", err)
		}

		var batch []virtualNetworksResponseBody
		for _, azureNetwork := range page.Value {
			if cap.ResourceLocation != "" && to.String(azureNetwork.Location) != cap.ResourceLocation {
				continue
			}
			var subnets []subnet

			if azureNetwork.Properties.Subnets != nil {
				for _, azureSubnet := range azureNetwork.Properties.Subnets {
					if azureSubnet.Name != nil {
						subnets = append(subnets, subnet{
							Name:         to.String(azureSubnet.Name),
							AddressRange: to.String(azureSubnet.Properties.AddressPrefix),
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
	client, err := NewManagedClustersClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	var clusters []clustersResponseBody
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get cluster list: %w", err)
		}

		for _, v := range page.Value {
			cluster := clustersResponseBody{
				ClusterName: to.String(v.Name),
				RBACEnabled: to.Bool(v.Properties.EnableRBAC),
				Location:    to.String(v.Location),
			}
			if v.ID != nil {
				match := matchResourceGroup.FindStringSubmatch(to.String(v.ID))
				if len(match) < 2 || match[1] == "" {
					return nil, http.StatusInternalServerError, fmt.Errorf("could not parse virtual network ID")
				}
				cluster.ResourceGroup = match[1]
			}
			clusters = append(clusters, cluster)
		}
	}

	return encodeOutput(clusters)
}

func listVMSizesV1(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ResourceLocation == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("region is required")
	}

	client, err := NewVirtualMachineSizesClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	vmSizes := make([]string, 0)
	pager := client.NewListPager(cap.ResourceLocation, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get VM sizes: %v", err)
		}
		for _, rg := range nextResult.Value {
			vmSizes = append(vmSizes, to.String(rg.Name))
		}
	}

	return encodeOutput(vmSizes)
}

const (
	AzureSkuResourceTypeVM            = "virtualMachines"
	AzureAcceleratedNetworkingFeature = "AcceleratedNetworkingEnabled"
)

func listVMSizesV2(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ResourceLocation == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("region is required")
	}

	type VMSizeInfo struct {
		//Name is synonymous with the size shown in Rancher UI
		Name                           string
		AcceleratedNetworkingSupported bool
		AvailabilityZones              []string
	}

	skuClient, err := NewVirtualMachineSKUClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	var vmSkuInfo []VMSizeInfo
	// only get resources for the given location.
	// https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5#example-ResourceSKUsClient.NewListPager-ListsAllAvailableResourceSkUsForTheSpecifiedRegion
	pager := skuClient.NewListPager(&armcompute.ResourceSKUsClientListOptions{
		Filter: azcoreto.Ptr(fmt.Sprintf("location eq '%s'", cap.ResourceLocation)),
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		for _, v := range page.Value {
			// we currently can't specify a particular SKU type in the API request,
			// we have to filter them out here
			if to.String(v.ResourceType) != AzureSkuResourceTypeVM {
				continue
			}

			vm := VMSizeInfo{
				Name:              to.String(v.Name),
				AvailabilityZones: []string{},
			}

			if v.Capabilities != nil {
				for _, skuCapabilities := range v.Capabilities {
					if to.String(skuCapabilities.Name) == AzureAcceleratedNetworkingFeature && to.String(skuCapabilities.Value) == "True" {
						vm.AcceleratedNetworkingSupported = true
						break
					}
				}
			}

			if v.LocationInfo != nil && len(v.LocationInfo) > 0 {
				locInfo := v.LocationInfo
				// We specified a location in the Azure API request so there is at most one element
				for _, z := range locInfo[0].Zones {
					vm.AvailabilityZones = append(vm.AvailabilityZones, *z)
				}
			}

			vmSkuInfo = append(vmSkuInfo, vm)
		}
	}

	return encodeOutput(vmSkuInfo)
}

type locationsResponseBody struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

func listLocations(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	client, err := NewSubscriptionServiceClient(cap)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	var locations []locationsResponseBody
	pager := client.NewListLocationsPager(cap.SubscriptionID, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get locations: %w", err)
		}

		for _, v := range page.Value {
			locations = append(locations, locationsResponseBody{
				Name:        to.String(v.Name),
				DisplayName: to.String(v.DisplayName),
			})
		}
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

func upgradeableVersionsMap(patchVersion *armcontainerservice.KubernetesPatchVersion) map[string]bool {
	rval := make(map[string]bool, 0)

	if patchVersion.Upgrades == nil {
		// already on latest version, no upgrades available
		return rval
	}
	for _, upgradeVersion := range patchVersion.Upgrades {
		rval[*upgradeVersion] = true
	}

	return rval
}
