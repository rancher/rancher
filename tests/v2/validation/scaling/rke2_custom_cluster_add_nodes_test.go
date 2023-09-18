package scaling

import (
	"fmt"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/provisioning"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RKE2CustomClusterAddNodeTestSuit struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *provisioninginput.Config
}

func (c *RKE2CustomClusterAddNodeTestSuit) TearDownSuite() {
	c.session.Cleanup()
}

func (c *RKE2CustomClusterAddNodeTestSuit) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	c.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, c.clustersConfig)
	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client
}

func (c *RKE2CustomClusterAddNodeTestSuit) TestRKE2CustomClusterAddWorkerNode() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(c.T(), clusterName, "Cluster name is not provided")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(c.T(), err)

	daemonsetName := namegen.AppendRandomString("test-ds")

	nodeProviderNames := c.clustersConfig.NodeProviders
	for _, nodeProviderName := range nodeProviderNames {
		externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
		steveClient, err := client.Steve.ProxyDownstream(clusterID)
		require.NoError(c.T(), err)

		initialWorkerNodeCount, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": clusterID,
				"worker":    true,
			},
		})
		require.NoError(c.T(), err)

		containerTemplate := workloads.NewContainer("rancher-test", "ranchertest/mytestcontainer", corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{})
		podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil)
		daemonsetTemplate := workloads.NewDaemonSetTemplate(daemonsetName, workloadNS, podTemplate, true, nil)
		daemonsetObj, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
		require.NoError(c.T(), err)
		assert.Equal(c.T(), daemonsetObj.Name, daemonsetName)

		err = charts.WatchAndWaitDaemonSets(client, clusterID, workloadNS, metav1.ListOptions{
			FieldSelector: "metadata.name=" + daemonsetObj.Name,
		})
		require.NoError(c.T(), err)

		c.validateDaemonset(client, clusterID, workloadNS, daemonsetObj)
		logrus.Infof("daemonset created successfully")

		newWorkerNodes, err := externalNodeProvider.NodeCreationFunc(client, []string{"--worker"}, []int32{1})
		require.NoError(c.T(), err)

		token, err := tokenregistration.GetRegistrationToken(client, clusterID)
		require.NoError(c.T(), err)

		for _, newWorkerNode := range newWorkerNodes {
			command := fmt.Sprintf("%s %s --address %s", token.InsecureNodeCommand, "--worker", newWorkerNode.PublicIPAddress)
			output, err := newWorkerNode.ExecuteCommand(command)
			require.NoError(c.T(), err)
			logrus.Info(output)
		}

		_, clusterObj, err := clusters.GetProvisioningClusterByName(client, clusterName, clusterNS)
		require.NoError(c.T(), err)

		provisioning.VerifyCluster(c.T(), client, clusterObj)

		result, err := validateNodeCount(client, clusterID, "worker", len(initialWorkerNodeCount.Data))
		require.NoError(c.T(), err)
		require.True(c.T(), result)

		daemonsetObj, err = steveClient.SteveType(workloads.DaemonsetSteveType).ByID(daemonsetObj.Namespace + "/" + daemonsetObj.Name)
		require.NoError(c.T(), err)

		err = charts.WatchAndWaitDaemonSets(client, clusterID, workloadNS, metav1.ListOptions{
			FieldSelector: "metadata.name=" + daemonsetObj.Name,
		})
		require.NoError(c.T(), err)

		c.validateDaemonset(client, clusterID, workloadNS, daemonsetObj)
		logrus.Infof("daemonset scaled successfully")

		err = steveClient.SteveType(workloads.DaemonsetSteveType).Delete(daemonsetObj)
		require.NoError(c.T(), err)

		err = deleteRKE2CustomClusterNode(client, newWorkerNodes, clusterID)
		require.NoError(c.T(), err)
		_, clusterObj, err = clusters.GetProvisioningClusterByName(client, clusterName, clusterNS)
		require.NoError(c.T(), err)

		provisioning.VerifyCluster(c.T(), client, clusterObj)
	}
}

func (c *RKE2CustomClusterAddNodeTestSuit) TestRKE2CustomClusterAddControlPlaneNode() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(c.T(), clusterName, "Cluster name is not provided")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(c.T(), err)

	nodeProviderNames := c.clustersConfig.NodeProviders
	for _, nodeProviderName := range nodeProviderNames {
		externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
		initialControlPlaneNodeCount, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId":    clusterID,
				"controlplane": true,
			},
		})
		require.NoError(c.T(), err)

		newControlPlaneNodes, err := externalNodeProvider.NodeCreationFunc(client, []string{"--controlplane"}, []int32{1})
		require.NoError(c.T(), err)

		token, err := tokenregistration.GetRegistrationToken(client, clusterID)
		require.NoError(c.T(), err)

		for _, newControlPlaneNode := range newControlPlaneNodes {
			command := fmt.Sprintf("%s %s --address %s", token.InsecureNodeCommand, "--controlplane", newControlPlaneNode.PublicIPAddress)
			output, err := newControlPlaneNode.ExecuteCommand(command)
			require.NoError(c.T(), err)
			logrus.Info(output)
		}

		_, clusterObj, err := clusters.GetProvisioningClusterByName(client, clusterName, clusterNS)
		require.NoError(c.T(), err)

		provisioning.VerifyCluster(c.T(), client, clusterObj)

		result, err := validateNodeCount(client, clusterID, "controlPlane", len(initialControlPlaneNodeCount.Data))
		require.NoError(c.T(), err)
		require.True(c.T(), result)

		err = deleteRKE2CustomClusterNode(client, newControlPlaneNodes, clusterID)
		require.NoError(c.T(), err)
		_, clusterObj, err = clusters.GetProvisioningClusterByName(client, clusterName, clusterNS)
		require.NoError(c.T(), err)

		provisioning.VerifyCluster(c.T(), client, clusterObj)
	}
}

func (c *RKE2CustomClusterAddNodeTestSuit) TestRKE2CustomClusterAddEtcdNode() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(c.T(), clusterName, "Cluster name is not provided")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(c.T(), err)

	nodeProviderNames := c.clustersConfig.NodeProviders
	for _, nodeProviderName := range nodeProviderNames {
		externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
		initialEtcdNodeCount, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": clusterID,
				"etcd":      true,
			},
		})
		require.NoError(c.T(), err)

		newEtcdNodes, err := externalNodeProvider.NodeCreationFunc(client, []string{"--etcd"}, []int32{2})
		require.NoError(c.T(), err)

		token, err := tokenregistration.GetRegistrationToken(client, clusterID)
		require.NoError(c.T(), err)

		for _, newEtcdNode := range newEtcdNodes {
			command := fmt.Sprintf("%s %s --address %s", token.InsecureNodeCommand, "--etcd", newEtcdNode.PublicIPAddress)
			output, err := newEtcdNode.ExecuteCommand(command)
			require.NoError(c.T(), err)
			logrus.Info(output)
		}

		_, clusterObj, err := clusters.GetProvisioningClusterByName(client, clusterName, clusterNS)
		require.NoError(c.T(), err)

		provisioning.VerifyCluster(c.T(), client, clusterObj)

		result, err := validateNodeCount(client, clusterID, "etcd", len(initialEtcdNodeCount.Data))
		require.NoError(c.T(), err)
		require.True(c.T(), result)

		err = deleteRKE2CustomClusterNode(client, newEtcdNodes, clusterID)
		require.NoError(c.T(), err)
		_, clusterObj, err = clusters.GetProvisioningClusterByName(client, clusterName, clusterNS)
		require.NoError(c.T(), err)

		provisioning.VerifyCluster(c.T(), client, clusterObj)
	}
}

func (c *RKE2CustomClusterAddNodeTestSuit) validateDaemonset(client *rancher.Client, clusterID string, namespaceName string, daemonsetObj *v1.SteveAPIObject) {
	workerNodesCollection, err := client.Management.Node.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
			"worker":    true,
		},
	})
	require.NoError(c.T(), err)

	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(c.T(), err)

	daemonSetID := daemonsetObj.Namespace + "/" + daemonsetObj.Name
	daemonsetResp, err := steveClient.SteveType(workloads.DaemonsetSteveType).ByID(daemonSetID)
	require.NoError(c.T(), err)

	daemonsetStatus := &appv1.DaemonSetStatus{}
	err = v1.ConvertToK8sType(daemonsetResp.Status, daemonsetStatus)
	require.NoError(c.T(), err)

	require.Equalf(c.T(), int(daemonsetStatus.NumberAvailable), len(workerNodesCollection.Data), "Daemonset %v doesn't have the required ready", daemonsetObj.Name)
}

func TestRKE2CustomClusterAddNodes(t *testing.T) {
	suite.Run(t, new(RKE2CustomClusterAddNodeTestSuit))
}
