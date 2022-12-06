package resources

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

var (
	balanceSimilarNodeGroups = "false"
	ctx                      = context.Background()
	defaultRandStringLength  = 5
	disableLocalAccounts     = false
	dnsPrefix                = "auto-dns"
	enableAutoScaling        = true
	enablePublicIP           = false
	enableRBAC               = true
	k8sVersion               = "1.24.6"
	loadProfileCount         = (int32)(2)
	location                 = "eastus2"
	maxCount                 = (int32)(3)
	maxNodeProvisionTime     = "15m"
	maxPods                  = (int32)(110)
	minCount                 = (int32)(1)
	nodeCount                = (int32)(3)
	nodePoolName             = "nodepool"
	osDiskSize               = (int32)(120)
	osDiskType               = "Managed"
	scanInterval             = "20s"
	skipNodesWithSystemPods  = "false"
	vmSize                   = "Standard_DS2_v2"
)

// CreateAKSCluster creates an AKS cluster in the specified resource group.
func CreateAKSCluster(clusterName, subscriptionID, clientID, resourceGroupName string, credential azcore.TokenCredential) (*runtime.Poller[armcontainerservice.ManagedClustersClientCreateOrUpdateResponse], error) {
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, err
	}

	agentPool := []*armcontainerservice.ManagedClusterAgentPoolProfile{
		{
			Type: to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
			AvailabilityZones: []*string{
				to.Ptr("1"),
				to.Ptr("2"),
				to.Ptr("3")},
			Count:              &nodeCount,
			EnableAutoScaling:  &enableAutoScaling,
			EnableNodePublicIP: &enablePublicIP,
			MaxCount:           &maxCount,
			MaxPods:            &maxPods,
			MinCount:           &minCount,
			Mode:               to.Ptr(armcontainerservice.AgentPoolModeSystem),
			Name:               &nodePoolName,
			OSDiskSizeGB:       &osDiskSize,
			OSDiskType:         to.Ptr(armcontainerservice.OSDiskType(osDiskType)),
			OSType:             to.Ptr(armcontainerservice.OSTypeLinux),
			ScaleDownMode:      to.Ptr(armcontainerservice.ScaleDownModeDeallocate),
			VMSize:             &vmSize,
		},
	}

	autoScaler := &armcontainerservice.ManagedClusterPropertiesAutoScalerProfile{
		BalanceSimilarNodeGroups: &balanceSimilarNodeGroups,
		Expander:                 to.Ptr(armcontainerservice.ExpanderPriority),
		MaxNodeProvisionTime:     &maxNodeProvisionTime,
		ScanInterval:             &scanInterval,
		SkipNodesWithSystemPods:  &skipNodesWithSystemPods,
	}

	clusterIdentity := &armcontainerservice.ManagedClusterIdentity{
		Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
	}

	networkProfile := &armcontainerservice.NetworkProfile{
		LoadBalancerProfile: &armcontainerservice.ManagedClusterLoadBalancerProfile{
			ManagedOutboundIPs: &armcontainerservice.ManagedClusterLoadBalancerProfileManagedOutboundIPs{
				Count: &loadProfileCount,
			},
		},
	}

	servicePrincipalProfile := &armcontainerservice.ManagedClusterServicePrincipalProfile{
		ClientID: &clientID,
	}

	skuProfile := &armcontainerservice.ManagedClusterSKU{
		Name: to.Ptr(armcontainerservice.ManagedClusterSKUNameBasic),
		Tier: to.Ptr(armcontainerservice.ManagedClusterSKUTierPaid),
	}

	properties := &armcontainerservice.ManagedClusterProperties{
		AgentPoolProfiles:       agentPool,
		AutoScalerProfile:       autoScaler,
		DisableLocalAccounts:    &disableLocalAccounts,
		DNSPrefix:               &dnsPrefix,
		EnableRBAC:              &enableRBAC,
		KubernetesVersion:       &k8sVersion,
		NetworkProfile:          networkProfile,
		ServicePrincipalProfile: servicePrincipalProfile,
	}

	params := armcontainerservice.ManagedCluster{
		Location:   &location,
		Identity:   clusterIdentity,
		Tags:       map[string]*string{},
		Properties: properties,
		SKU:        skuProfile,
	}

	return client.BeginCreateOrUpdate(ctx, resourceGroupName, clusterName, params, nil)
}

// DeleteAKSCluster deletes an AKS cluster in the specified resource group.
func DeleteAKSCluster(clusterName, subscriptionID, resourceGroupName string, credential azcore.TokenCredential) (*runtime.Poller[armcontainerservice.ManagedClustersClientDeleteResponse], error) {
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, err
	}

	return client.BeginDelete(ctx, resourceGroupName, clusterName, nil)
}
