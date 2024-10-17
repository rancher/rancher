//go:build (validation || infra.rke2k3s || cluster.any || sanity) && !stress && !extended

package connectivity

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	shepworkloads "github.com/rancher/shepherd/extensions/workloads"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type NetworkPolicyTestSuite struct {
	suite.Suite
	session     *session.Session
	client      *rancher.Client
	project     *management.Project
	clusterName string
	namespace   *v1.SteveAPIObject
	steveClient *v1.Client
}

const (
	nodeRole     = "control-plane"
	podSteveType = "pod"
)

func (n *NetworkPolicyTestSuite) TearDownSuite() {
	n.session.Cleanup()
}

func (n *NetworkPolicyTestSuite) SetupSuite() {
	testSession := session.NewSession()
	n.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(n.T(), err)

	n.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmpty(n.T(), clusterName, "Cluster name to install is not set")
	n.clusterName = clusterName

	cluster, err := clusters.NewClusterMeta(client, clusterName)
	require.NoError(n.T(), err)

	projectConfig := &management.Project{
		ClusterID: cluster.ID,
		Name:      pingPodProjectName,
	}

	createdProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(n.T(), err)
	require.Equal(n.T(), createdProject.Name, pingPodProjectName)
	n.project = createdProject

	names := newNames()

	namespaceName := names.random["namespaceName"]
	daemonsetName := names.random["daemonsetName"]

	n.T().Logf("Creating namespace with name [%v]", namespaceName)
	n.namespace, err = namespaces.CreateNamespace(n.client, namespaceName, "{}", map[string]string{}, map[string]string{}, n.project)
	require.NoError(n.T(), err)
	assert.Equal(n.T(), n.namespace.Name, namespaceName)

	n.steveClient, err = n.client.Steve.ProxyDownstream(n.project.ClusterID)
	require.NoError(n.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	n.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, n.namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := n.steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(n.T(), err)
	assert.Equal(n.T(), createdDaemonSet.Name, daemonsetName)

	n.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(n.client, n.project.ClusterID, n.namespace.Name, metav1.ListOptions{})
	require.NoError(n.T(), err)
}

func (n *NetworkPolicyTestSuite) TestPingPodsFromCPNode() {
	steveclient, err := n.client.Steve.ProxyDownstream(n.project.ClusterID)
	assert.NoError(n.T(), err)

	pods, err := steveclient.SteveType(podSteveType).NamespacedSteveClient(n.namespace.Name).List(nil)
	assert.NoError(n.T(), err)
	assert.NotEmpty(n.T(), pods.Data)

	query, err := url.ParseQuery("labelSelector=node-role.kubernetes.io/" + nodeRole + "=true")
	assert.NoError(n.T(), err)

	nodeList, err := n.steveClient.SteveType("node").List(query)
	assert.NoError(n.T(), err)
	assert.NotEmpty(n.T(), nodeList, err)

	for _, machine := range nodeList.Data {
		n.T().Logf("Running ping on [%v]", machine.Name)

		for _, podResp := range pods.Data {
			newPod := &corev1.Pod{}
			err = v1.ConvertToK8sType(podResp, newPod)
			assert.NoError(n.T(), err)

			podIP := newPod.Status.PodIP
			log, err := curlCommand(n.client, n.project.ClusterID, fmt.Sprintf("%s:%s/name.html", podIP, strconv.Itoa(80)))
			require.NoError(n.T(), err)
			require.True(n.T(), strings.Contains(log, newPod.Name))
		}
	}
}

func TestNetworkPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(NetworkPolicyTestSuite))
}
