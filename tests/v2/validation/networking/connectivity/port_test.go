//go:build (validation || infra.rke2k3s || cluster.any || sanity) && !stress && !extended

package connectivity

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"strconv"
	"strings"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	kubeapinodes "github.com/rancher/shepherd/extensions/kubeapi/nodes"
	shepworkloads "github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	clusterNameAnnotation  = "cluster.x-k8s.io/cluster-name"
	kubeSystemNamespace    = "kube-system"
	portLimit              = 65534
	cloudControllerManager = "aws-cloud-controller-manager"
)

type PortTestSuite struct {
	suite.Suite
	client      *rancher.Client
	session     *session.Session
	cluster     *management.Cluster
	clusterName string
}

func (p *PortTestSuite) TearDownSuite() {
	p.session.Cleanup()
}

func (p *PortTestSuite) SetupSuite() {
	p.session = session.NewSession()

	client, err := rancher.NewClient("", p.session)
	require.NoError(p.T(), err)

	p.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	p.clusterName = client.RancherConfig.ClusterName
	require.NotEmptyf(p.T(), p.clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(p.client, p.clusterName)
	require.NoError(p.T(), err, "Error getting cluster ID")

	p.cluster, err = p.client.Management.Cluster.ByID(clusterID)
	require.NoError(p.T(), err)
}

func (p *PortTestSuite) TestHostPort() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	//This must be a valid port number, 0 < hostPort < 65536
	//Using a random port to avoid 'port in use' failures and allow the test to be rerun
	hostPort := rand.IntN(65534) + 1

	testContainerPodTemplate := newPodTemplateWithTestContainer()
	testContainerPodTemplate.Spec.Containers[0].Ports = []corev1.ContainerPort{
		corev1.ContainerPort{
			HostPort:      int32(hostPort),
			ContainerPort: 80,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	p.T().Logf("Getting the node using the cluster name [%v]", p.clusterName)
	query, err := url.ParseQuery(fmt.Sprintf("%s=%s", clusterNameAnnotation, p.clusterName))
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), nodeList, err)
	assert.NotEmpty(p.T(), nodeList.Data)

	for _, machine := range nodeList.Data {
		p.T().Log("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		assert.NoError(p.T(), err)
		nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)

		p.T().Log("Executing the SSH shell command on the node")
		log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(hostPort)))
		require.NoError(p.T(), err)
		require.True(p.T(), strings.Contains(log, daemonsetName))
	}
}

func (p *PortTestSuite) TestNodePort() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	//It will allocate a port from a range 30000-32767
	//Using a random port to avoid 'port in use' failures and allow the test to be rerun
	nodePort := rand.IntN(2767) + 30000

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol: corev1.ProtocolTCP,
			Port:     80,
			NodePort: int32(nodePort),
		},
	}
	nodePortservice := services.NewServiceTemplate(serviceName, namespace.Name, corev1.ServiceTypeNodePort, ports, daemonsetTemplate.Spec.Template.Labels)
	serviceResp, err := services.CreateService(steveClient, nodePortservice)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	p.T().Logf("Getting the node using the cluster name [%v]", p.clusterName)
	query, err := url.ParseQuery(fmt.Sprintf("%s=%s", clusterNameAnnotation, p.clusterName))
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), nodeList, err)
	assert.NotEmpty(p.T(), nodeList.Data)

	for _, machine := range nodeList.Data {
		p.T().Log("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		assert.NoError(p.T(), err)

		// Project Network Isolation should be enabled when setting up the cluster for this test
		nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeExternalIP)
		if nodeIP == "" {
			nodeIP = kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)
		}

		p.T().Log("Executing the SSH shell command on the node")
		log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(nodePort)))
		require.NoError(p.T(), err)
		require.True(p.T(), strings.Contains(log, daemonsetName))
	}
}

func (p *PortTestSuite) TestClusterIP() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	//This must be a valid port number, 0 < hostPort < 65536
	//Using a random port to avoid 'port in use' failures and allow the test to be rerun
	hostPort := rand.IntN(65534) + 1

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       int32(hostPort),
			TargetPort: intstr.FromInt(80),
		},
	}
	clusterIPService := services.NewServiceTemplate(serviceName, namespace.Name, corev1.ServiceTypeClusterIP, ports, daemonsetTemplate.Spec.Template.Labels)
	serviceResp, err := services.CreateService(steveClient, clusterIPService)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	serviceResp, err = steveClient.SteveType(services.ServiceSteveType).ByID(serviceResp.ID)
	assert.NoError(p.T(), err)

	p.T().Log("Getting the cluster IP")
	newService := &corev1.Service{}
	err = steveV1.ConvertToK8sType(serviceResp.JSONResp, newService)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), newService.Spec.ClusterIP)

	clusterIP := newService.Spec.ClusterIP

	p.T().Log("Executing the SSH shell command on the node")
	log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", clusterIP, strconv.Itoa(hostPort)))
	require.NoError(p.T(), err)
	require.True(p.T(), strings.Contains(log, daemonsetName))
}

func (p *PortTestSuite) TestLoadBalance() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Checking cluster version and if the cloud-controller-manager is installed")
	catalogClient, err := p.client.GetClusterCatalogClient(p.cluster.ID)
	require.NoError(p.T(), err)

	clusterID, err := clusters.GetV1ProvisioningClusterByName(p.client, p.client.RancherConfig.ClusterName)
	require.NoError(p.T(), err)

	cluster, err := p.client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterID)
	require.NoError(p.T(), err)

	newCluster := &provv1.Cluster{}
	err = steveV1.ConvertToK8sType(cluster, newCluster)
	require.NoError(p.T(), err)

	_, err = catalogClient.Apps(kubeSystemNamespace).Get(context.TODO(), cloudControllerManager, metav1.GetOptions{})
	if !strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") && err != nil && strings.Contains(err.Error(), "not found") {
		p.T().Skip("Load Balance test requires access to cloud provider.")
	}

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	//This must be a valid port number, 0 < hostPort < 65536
	//Using a random port to avoid 'port in use' failures and allow the test to be rerun
	hostPort := rand.IntN(65534) + 1

	//It will allocate a port from a range 30000-32767
	//Using a random port to avoid 'port in use' failures and allow the test to be rerun
	nodePort := rand.IntN(2767) + 30000

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       int32(hostPort),
			TargetPort: intstr.FromInt(80),
			NodePort:   int32(nodePort),
		},
	}
	lbService := services.NewServiceTemplate(serviceName, namespace.Name, corev1.ServiceTypeLoadBalancer, ports, daemonsetTemplate.Spec.Selector.MatchLabels)
	serviceResp, err := services.CreateService(steveClient, lbService)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	p.T().Logf("Getting the node using the cluster name [%v]", p.clusterName)
	query, err := url.ParseQuery(fmt.Sprintf("%s=%s", clusterNameAnnotation, p.clusterName))
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), nodeList, err)
	assert.NotEmpty(p.T(), nodeList.Data)

	for _, machine := range nodeList.Data {
		p.T().Log("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		assert.NoError(p.T(), err)

		nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeExternalIP)
		if nodeIP == "" {
			nodeIP = kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)
		}

		p.T().Log("Executing the SSH shell command on the node")
		log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(nodePort)))
		require.NoError(p.T(), err)
		require.True(p.T(), strings.Contains(log, daemonsetName))
	}
}

func TestPortTestSuite(t *testing.T) {
	suite.Run(t, new(PortTestSuite))
}
