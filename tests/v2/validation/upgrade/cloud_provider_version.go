package upgrade

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func upgradeVsphereCloudProviderCharts(t *testing.T, client *rancher.Client, clusterName string) {
	logrus.Info("Starting upgrade test...")
	err := charts.UpgradeVsphereOutOfTreeCharts(client, catalog.RancherChartRepo, clusterName)
	require.NoError(t, err)

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)

	podErrors := pods.StatusPods(client, clusterID)
	require.Empty(t, podErrors)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	clusterObject, err := adminClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(provisioninginput.Namespace + "/" + clusterID)
	require.NoError(t, err)

	permutations.CreatePVCWorkload(t, client, clusterObject)
}
