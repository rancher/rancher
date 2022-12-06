package importing

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/aks"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/aks/resources"
	"github.com/sirupsen/logrus"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

var (
	ctx                     = context.Background()
	defaultRandStringLength = 5
	enableNetworkPolicy     = false
)

// ImportAKSCluster is a helper function that imports an AKS cluster into Rancher. It will first create a resoure group
// and then create a cluster in that resource group. It will then import the cluster into Rancher.
func ImportAKSCluster(client *rancher.Client, cloudCredential *cloudcredentials.CloudCredential, clusterName string) (*management.Cluster, error) {
	logrus.Infof("======================================")
	logrus.Infof("TEST CASE: Importing AKS Cluster")
	logrus.Infof("======================================")

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		logrus.Fatalf("FAILURE: Unable to authenticate. %+v", err)
	}

	aksHostCluster := aks.AKSHostClusterConfig(clusterName, cloudCredential.ID)

	logrus.Infof("Creating cluster %s...", clusterName)
	azClient, err := resources.CreateAKSCluster(clusterName, cloudCredential.AzureCredentialConfig.SubscriptionID, cloudCredential.AzureCredentialConfig.ClientID, aksHostCluster.ResourceGroup, cred)
	if err != nil {
		logrus.Fatalf("FAILURE: Unable to create cluster: %+v", err)
	} else {
		_, err = azClient.PollUntilDone(ctx, nil)
		if err != nil {
			logrus.Fatalf("FAILURE: Unable to create cluster: %+v", err)
		} else {
			logrus.Infof("Cluster %s is created successfully!", clusterName)
		}
	}

	aksHostCluster.Imported = true

	importedCluster := &management.Cluster{
		AKSConfig:               aksHostCluster,
		DockerRootDir:           "/var/lib/docker",
		EnableClusterAlerting:   false,
		EnableClusterMonitoring: false,
		EnableNetworkPolicy:     &enableNetworkPolicy,
		Labels:                  map[string]string{},
		Name:                    clusterName,
		WindowsPreferedCluster:  false,
	}

	logrus.Infof("Importing cluster %s into Rancher...", clusterName)
	cluster, err := client.Management.Cluster.Create(importedCluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		clusterResp, err := client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.State == "active" {
			logrus.Infof("Cluster %s has been successfully imported!", clusterName)
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	logrus.Infof("Deleting cluster %s from Azure...", clusterName)
	delClient, err := resources.DeleteAKSCluster(clusterName, cloudCredential.AzureCredentialConfig.SubscriptionID, aksHostCluster.ResourceGroup, cred)
	if err != nil {
		logrus.Fatalf("FAILURE: Unable to delete cluster: %+v", err)
	} else {
		_, err = delClient.PollUntilDone(ctx, nil)
		if err != nil {
			logrus.Fatalf("FAILURE: Unable to delete cluster: %+v", err)
		} else {
			logrus.Infof("Cluster %s is deleted successfully!", clusterName)
		}
	}

	return cluster, err
}
