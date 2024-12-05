//go:build (validation || infra.rke2k3s || cluster.any || sanity) && !stress && !extended

package connectivity

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	kubeapinodes "github.com/rancher/shepherd/extensions/kubeapi/nodes"
	"github.com/rancher/shepherd/extensions/sshkeys"
	shepworkloads "github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	labelWorker            = "labelSelector=node-role.kubernetes.io/worker=true"
	kubeSystemNamespace    = "kube-system"
	cloudControllerManager = "aws-cloud-controller-manager"
	defaultPort            = 80
)

type PortTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	cluster   *management.Cluster
	namespace *corev1.Namespace
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
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(p.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(p.client, clusterName)
	require.NoError(p.T(), err, "Error getting cluster ID")

	p.cluster, err = p.client.Management.Cluster.ByID(clusterID)
	require.NoError(p.T(), err)

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)
	p.namespace = namespace
}

func (p *PortTestSuite) TestHostPort() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	hostPort := getHostPort()

	testContainerPodTemplate := newPodTemplateWithTestContainer()
	testContainerPodTemplate.Spec.Containers[0].Ports = []corev1.ContainerPort{
		corev1.ContainerPort{
			HostPort:      int32(hostPort),
			ContainerPort: defaultPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, p.namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, p.namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	p.validateHostPort(steveClient, hostPort, daemonsetName)
}

func (p *PortTestSuite) TestNodePort() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, p.namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, p.namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	nodePort := getNodePort()

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol: corev1.ProtocolTCP,
			Port:     defaultPort,
			NodePort: int32(nodePort),
		},
	}
	nodePortservice := services.NewServiceTemplate(serviceName, p.namespace.Name, corev1.ServiceTypeNodePort, ports, daemonsetTemplate.Spec.Template.Labels)
	serviceResp, err := services.CreateService(steveClient, nodePortservice)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	p.validateNodePort(steveClient, nodePort, daemonsetName)
}

func (p *PortTestSuite) TestClusterIP() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	//p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, p.namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, p.namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	port := getHostPort()

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       int32(port),
			TargetPort: intstr.FromInt(defaultPort),
		},
	}
	clusterIPService := services.NewServiceTemplate(serviceName, p.namespace.Name, corev1.ServiceTypeClusterIP, ports, daemonsetTemplate.Spec.Template.Labels)
	serviceResp, err := services.CreateService(steveClient, clusterIPService)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	p.validateClusterIP(steveClient, serviceResp.ID, port, daemonsetName)
}

func (p *PortTestSuite) TestLoadBalancer() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	if !p.isCloudManagerEnabled() {
		p.T().Skip("Load Balance test requires access to cloud provider.")
	}

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, p.namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, p.namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	port := getHostPort()
	nodePort := getNodePort()

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       int32(port),
			TargetPort: intstr.FromInt(defaultPort),
			NodePort:   int32(nodePort),
		},
	}
	lbService := services.NewServiceTemplate(serviceName, p.namespace.Name, corev1.ServiceTypeLoadBalancer, ports, daemonsetTemplate.Spec.Selector.MatchLabels)
	serviceResp, err := services.CreateService(steveClient, lbService)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	p.validateLoadBalancer(steveClient, nodePort, daemonsetName)
}

func (p *PortTestSuite) TestClusterIPScaleAndUpgrade() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	deploymentName := namegen.AppendRandomString("test-scale-up")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", deploymentName)
	deploymentTemplate := shepworkloads.NewDeploymentTemplate(deploymentName, namespace.Name, testContainerPodTemplate, true, nil)
	replicas := int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	createdDeployment, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDeployment.Name, deploymentName)

	p.T().Logf("Waiting for deployment [%v] to have expected number of available replicas", deploymentName)
	err = charts.WatchAndWaitDeployments(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentTemplate.Name,
	})
	require.NoError(p.T(), err)

	port := getHostPort()

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       int32(port),
			TargetPort: intstr.FromInt(defaultPort),
		},
	}
	clusterIPService := services.NewServiceTemplate(serviceName, namespace.Name, corev1.ServiceTypeClusterIP, ports, deploymentTemplate.Spec.Template.Labels)
	serviceResp, err := services.CreateService(steveClient, clusterIPService)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	log.Info("Scaling up deployment")
	replicas = int32(3)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 3, namespace.Name)
	p.validateClusterIP(steveClient, serviceResp.ID, port, deploymentName)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateClusterIP(steveClient, serviceResp.ID, port, deploymentName)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateClusterIP(steveClient, serviceResp.ID, port, deploymentName)
}

func (p *PortTestSuite) TestHostPortScaleAndUpgrade() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	if !p.isNodePool(steveClient) {
		p.T().Skip("The Host Port scale up/down test requires at least 3 worker nodes.")
	}

	hostPort := getHostPort()

	testContainerPodTemplate := newPodTemplateWithTestContainer()
	testContainerPodTemplate.Spec.Containers[0].Ports = []corev1.ContainerPort{
		corev1.ContainerPort{
			HostPort:      int32(hostPort),
			ContainerPort: defaultPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	deploymentName := namegen.AppendRandomString("test-scale-up")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", deploymentName)
	deploymentTemplate := shepworkloads.NewDeploymentTemplate(deploymentName, namespace.Name, testContainerPodTemplate, true, nil)
	replicas := int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	createdDeployment, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDeployment.Name, deploymentName)

	p.T().Logf("Waiting for deployment [%v] to have expected number of available replicas", deploymentName)
	err = charts.WatchAndWaitDeployments(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentTemplate.Name,
	})
	require.NoError(p.T(), err)

	log.Info("Scaling up deployment")
	replicas = int32(3)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 3, namespace.Name)
	p.validateHostPortSSH(steveClient, hostPort, deploymentName, namespace.Name)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateHostPortSSH(steveClient, hostPort, deploymentName, namespace.Name)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateHostPortSSH(steveClient, hostPort, deploymentName, namespace.Name)
}

func (p *PortTestSuite) TestNodePortScaleAndUpgrade() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	deploymentName := namegen.AppendRandomString("test-scale-up")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", deploymentName)
	deploymentTemplate := shepworkloads.NewDeploymentTemplate(deploymentName, namespace.Name, testContainerPodTemplate, true, nil)
	replicas := int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	createdDeployment, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDeployment.Name, deploymentName)

	p.T().Logf("Waiting for deployment [%v] to have expected number of available replicas", deploymentName)
	err = charts.WatchAndWaitDeployments(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentTemplate.Name,
	})
	require.NoError(p.T(), err)

	nodePort := getNodePort()

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol: corev1.ProtocolTCP,
			Port:     defaultPort,
			NodePort: int32(nodePort),
		},
	}

	clusterIPService := services.NewServiceTemplate(serviceName, namespace.Name, corev1.ServiceTypeNodePort, ports, deploymentTemplate.Spec.Template.Labels)
	serviceResp, err := services.CreateService(steveClient, clusterIPService)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	log.Info("Scaling up deployment")
	replicas = int32(3)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 3, namespace.Name)
	p.validateNodePort(steveClient, nodePort, deploymentName)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateNodePort(steveClient, nodePort, deploymentName)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateNodePort(steveClient, nodePort, deploymentName)
}

func (p *PortTestSuite) TestLoadBalanceScaleAndUpgrade() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	if !p.isCloudManagerEnabled() {
		p.T().Skip("Load Balance test requires access to cloud provider.")
	}

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	deploymentName := namegen.AppendRandomString("test-scale-up")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", deploymentName)
	deploymentTemplate := shepworkloads.NewDeploymentTemplate(deploymentName, namespace.Name, testContainerPodTemplate, true, nil)
	replicas := int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	createdDeployment, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDeployment.Name, deploymentName)

	p.T().Logf("Waiting for deployment [%v] to have expected number of available replicas", deploymentName)
	err = charts.WatchAndWaitDeployments(p.client, p.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentTemplate.Name,
	})
	require.NoError(p.T(), err)

	port := getHostPort()
	nodePort := getNodePort()

	serviceName := namegen.AppendRandomString("test-service")
	p.T().Logf("Creating service with name [%v]", serviceName)
	ports := []corev1.ServicePort{
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       int32(port),
			TargetPort: intstr.FromInt(defaultPort),
			NodePort:   int32(nodePort),
		},
	}

	clusterIPService := services.NewServiceTemplate(serviceName, namespace.Name, corev1.ServiceTypeLoadBalancer, ports, deploymentTemplate.Spec.Template.Labels)
	serviceResp, err := services.CreateService(steveClient, clusterIPService)
	require.NoError(p.T(), err)

	err = services.VerifyService(steveClient, serviceResp)
	require.NoError(p.T(), err)

	log.Info("Scaling up deployment")
	replicas = int32(3)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 3, namespace.Name)
	p.validateLoadBalancer(steveClient, nodePort, deploymentName)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateLoadBalancer(steveClient, nodePort, deploymentName)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)
	p.validateWorkload(deploymentTemplate, containerImage, 2, namespace.Name)
	p.validateLoadBalancer(steveClient, nodePort, deploymentName)
}

// This must be a valid port number, 10250 < hostPort < 65536
// The range 1-10250 are reserved to Rancher
// Using a random port to avoid 'port in use' failures and allow the test to be rerun
func getHostPort() int {
	return rand.IntN(55283) + 10251
}

// It will allocate a port from a range 30000-32767
// Using a random port to avoid 'port in use' failures and allow the test to be rerun
func getNodePort() int {
	return rand.IntN(2767) + 30000
}

func (p *PortTestSuite) validateWorkload(deployment *appv1.Deployment, image string, expectedReplicas int, namespaceName string) {
	p.T().Log("Waiting deployment comes up active")
	err := charts.WatchAndWaitDeployments(p.client, p.cluster.ID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deployment.Name,
	})
	require.NoError(p.T(), err)

	p.T().Log("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(p.client, p.cluster.ID, namespaceName, deployment)
	require.NoError(p.T(), err)

	p.T().Logf("Counting all pods running by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(p.client, p.cluster.ID, namespaceName, image)
	require.NoError(p.T(), err)
	require.Equal(p.T(), expectedReplicas, countPods)
}

func (p *PortTestSuite) validateClusterIP(steveClient *steveV1.Client, serviceID string, hostPort int, workloadName string) {
	serviceResp, err := steveClient.SteveType(services.ServiceSteveType).ByID(serviceID)
	assert.NoError(p.T(), err)

	p.T().Log("Getting the cluster IP")
	newService := &corev1.Service{}
	err = steveV1.ConvertToK8sType(serviceResp.JSONResp, newService)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), newService.Spec.ClusterIP)

	clusterIP := newService.Spec.ClusterIP

	log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", clusterIP, strconv.Itoa(hostPort)))
	require.NoError(p.T(), err)
	require.True(p.T(), strings.Contains(log, workloadName))
}

func (p *PortTestSuite) validateNodePort(steveClient *steveV1.Client, nodePort int, workloadName string) {
	p.T().Logf("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
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

		log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(nodePort)))
		require.NoError(p.T(), err)
		require.True(p.T(), strings.Contains(log, workloadName))
	}
}

func (p *PortTestSuite) validateHostPort(steveClient *steveV1.Client, hostPort int, workloadName string) {
	p.T().Logf("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), nodeList.Data)

	for _, machine := range nodeList.Data {
		p.T().Log("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		assert.NoError(p.T(), err)

		// Project Network Isolation should be enabled when setting up the cluster for this test
		nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)

		log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(hostPort)))
		require.NoError(p.T(), err)
		require.True(p.T(), strings.Contains(log, workloadName))
	}
}

func (p *PortTestSuite) validateHostPortSSH(steveClient *steveV1.Client, hostPort int, workloadName string, namespaceName string) {
	p.T().Logf("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), nodeList.Data)

	_, stevecluster, err := clusters.GetProvisioningClusterByName(p.client, p.cluster.Name, provisioninginput.Namespace)

	wc, err := p.client.WranglerContext.DownStreamClusterWranglerContext(p.cluster.ID)
	require.NoError(p.T(), err)

	pods, err := wc.Core.Pod().List(namespaceName, metav1.ListOptions{})
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), pods)

	var nodes []string
	nodes = make([]string, 0)
	for i := 0; i < len(pods.Items); i++ {
		nodeName := pods.Items[i].Spec.NodeName
		nodes = append(nodes, nodeName)
	}
	assert.NotEmpty(p.T(), nodes)

	for _, machine := range nodeList.Data {
		p.T().Log("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		assert.NoError(p.T(), err)
		assert.NotEmpty(p.T(), newNode.Name)

		_, found := slices.BinarySearch(nodes, newNode.Name)
		if found {
			// Project Network Isolation should be enabled when setting up the cluster for this test
			nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)

			sshUser, err := sshkeys.GetSSHUser(p.client, stevecluster)
			assert.NoError(p.T(), err)
			assert.NotEmpty(p.T(), sshUser, errors.New("sshUser does not exist"))

			sshNode, err := sshkeys.GetSSHNodeFromMachine(p.client, sshUser, &machine)
			assert.NoError(p.T(), err)

			log, err := sshNode.ExecuteCommand(fmt.Sprintf("curl %s:%s/name.html", nodeIP, strconv.Itoa(hostPort)))
			if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
				assert.NoError(p.T(), err)
			}
			p.T().Logf("Log of the curl command {%v}", log)
			require.True(p.T(), strings.Contains(log, workloadName))
		}
	}
}

func (p *PortTestSuite) validateLoadBalancer(steveClient *steveV1.Client, nodePort int, workloadName string) {
	p.T().Logf("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
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

		log, err := curlCommand(p.client, p.cluster.ID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(nodePort)))
		require.NoError(p.T(), err)
		require.True(p.T(), strings.Contains(log, workloadName))
	}
}

func (p *PortTestSuite) isCloudManagerEnabled() bool {
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
		return false
	}

	return true
}

func (p *PortTestSuite) isNodePool(steveClient *steveV1.Client) bool {
	log.Info("Checking node pool")

	p.T().Logf("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	assert.NoError(p.T(), err)

	nodeList, err := steveClient.SteveType("node").List(query)
	assert.NoError(p.T(), err)
	assert.NotEmpty(p.T(), nodeList.Data)

	return len(nodeList.Data) >= 3
}

func TestPortTestSuite(t *testing.T) {
	suite.Run(t, new(PortTestSuite))
}
