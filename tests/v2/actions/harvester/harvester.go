package harvester

import (
	"time"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/rancher/norman/types"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"

	"github.com/rancher/shepherd/clients/harvester"
	"github.com/rancher/shepherd/clients/rancher"

	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/namegenerator"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	Harvester            = "harvester"
	HarvesterSettingType = "harvesterhci.io.setting"

	clusterRegistrationURL = "cluster-registration-url"
	clusterId              = "clusterId"
	providerLabel          = "provider.cattle.io"
)

// RegisterHarvesterCluster is a function that creates a Virtualization Management object in rancher, and registers
// the external harvester cluster with said object in rancher. This is required to use harvester as a rancher provider.
func RegisterHarvesterWithRancher(rancherClient *rancher.Client, harvesterClient *harvester.Client) (string, error) {
	importCluster := provv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      namegenerator.AppendRandomString("hvst-import"),
			Namespace: "fleet-default",
			Labels: map[string]string{
				providerLabel: Harvester,
			},
		},
	}

	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    40,
	}

	_, err := clusters.CreateK3SRKE2Cluster(rancherClient, &importCluster)
	if err != nil {
		return "", err
	}

	updatedCluster := new(provv1.Cluster)

	// wait for rancher's import cluster to be in pending state. No registration occurs yet
	err = wait.ExponentialBackoff(backoff, func() (hinished bool, err error) {
		updatedCluster, _, err = clusters.GetProvisioningClusterByName(rancherClient, importCluster.Name, importCluster.Namespace)
		if err != nil {
			return false, err
		}

		if updatedCluster.Status.ClusterName == "" {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return "", err
	}

	var token management.ClusterRegistrationToken

	// get the token from rancher's import cluster object once its ready
	err = kwait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		res, err := rancherClient.Management.ClusterRegistrationToken.List(&types.ListOpts{Filters: map[string]interface{}{
			clusterId: updatedCluster.Status.ClusterName,
		}})
		if err != nil {
			return false, err
		}

		if len(res.Data) > 0 && res.Data[0].ManifestURL != "" {
			token = res.Data[0]
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return "", err
	}

	clusterSteveUrl, err := harvesterClient.Steve.SteveType(HarvesterSettingType).ByID(clusterRegistrationURL)
	if err != nil {
		return "", err
	}

	setting := &harvesterv1.Setting{}
	err = steveV1.ConvertToK8sType(clusterSteveUrl, setting)
	if err != nil {
		return "", err
	}

	setting.Value = token.ManifestURL

	_, err = harvesterClient.Steve.SteveType(HarvesterSettingType).Update(clusterSteveUrl, setting)
	if err != nil {
		return "", err
	}

	status := &provv1.ClusterStatus{}
	err = steveV1.ConvertToK8sType(updatedCluster.Status, status)
	if err != nil {
		return "", err
	}

	return status.ClusterName, wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		updatedCluster, _, err = clusters.GetProvisioningClusterByName(rancherClient, importCluster.Name, importCluster.Namespace)
		if err != nil {
			return false, nil
		}

		if updatedCluster.Status.Ready {
			return true, nil
		}

		return false, nil
	})
}
