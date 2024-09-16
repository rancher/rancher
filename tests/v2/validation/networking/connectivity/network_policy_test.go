//go:build (validation || infra.rke2k3s || cluster.any || sanity) && !stress && !extended

package connectivity

import (
	"errors"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/sshkeys"
	shepworkloads "github.com/rancher/shepherd/extensions/workloads"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/url"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NetworkPolicyTestSuite struct {
	suite.Suite
	session     *session.Session
	client      *rancher.Client
	project     *management.Project
	clusterName string
}

const (
	nodeRole = "control-plane"
	// Ping once
	pingCmd       = "ping -c 1"
	succesfulPing = "0% packet loss"
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
}

func (n *NetworkPolicyTestSuite) TestPingPods() {
	names := newNames()

	namespaceName := names.random["namespaceName"]
	daemonsetName := names.random["daemonsetName"]

	n.T().Logf("Creating namespace with name [%v]", namespaceName)
	namespace, err := namespaces.CreateNamespace(n.client, namespaceName, "{}", map[string]string{}, map[string]string{}, n.project)
	require.NoError(n.T(), err)
	assert.Equal(n.T(), namespace.Name, namespaceName)

	steveClient, err := n.client.Steve.ProxyDownstream(n.project.ClusterID)
	require.NoError(n.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	n.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(n.T(), err)
	assert.Equal(n.T(), createdDaemonSet.Name, daemonsetName)

	n.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(n.client, n.project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(n.T(), err)

	wc, err := n.client.WranglerContext.DownStreamClusterWranglerContext(n.project.ClusterID)
	require.NoError(n.T(), err)

	pods, err := wc.Core.Pod().List(namespace.Name, metav1.ListOptions{})
	assert.NoError(n.T(), err)
	assert.NotEmpty(n.T(), pods)

	pod2Ip := pods.Items[1].Status.PodIP
	pingExecCmd := pingCmd + " " + pod2Ip
	nodeRole := nodeRole
	_, stevecluster, err := clusters.GetProvisioningClusterByName(n.client, n.clusterName, provisioninginput.Namespace)

	query, err := url.ParseQuery("labelSelector=node-role.kubernetes.io/" + nodeRole + "=true")
	assert.NoError(n.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(n.T(), err)

	firstMachine := nodeList.Data[0]

	sshUser, err := sshkeys.GetSSHUser(n.client, stevecluster)
	assert.NoError(n.T(), err)

	if sshUser == "" {
		assert.NoError(n.T(), errors.New("sshUser does not exist"))
	}

	sshNode, err := sshkeys.GetSSHNodeFromMachine(n.client, sshUser, &firstMachine)
	assert.NoError(n.T(), err)

	n.T().Logf("Running ping on [%v]", firstMachine.Name)

	excmdLog, err := sshNode.ExecuteCommand(pingExecCmd)
	if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
		assert.NoError(n.T(), err)
	}
	n.T().Logf("Log of the ping command {%v}", excmdLog)

	assert.Contains(n.T(), excmdLog, succesfulPing, "Unable to ping the pod")

}

func TestNetworkPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(NetworkPolicyTestSuite))
}
