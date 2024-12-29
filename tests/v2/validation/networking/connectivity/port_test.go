//go:build (validation || infra.rke2k3s || cluster.any || sanity) && !stress && !extended

package connectivity

import (
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
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
	defaultPort = 80
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

	if p.cluster.EnableNetworkPolicy == nil || !*p.cluster.EnableNetworkPolicy {
		p.T().Skip("The Host Port test requires project network enabled.")
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

	daemonsetName := namegen.AppendRandomString("test-daemonset")

	p.T().Logf("Creating a daemonset with the test container with name [%v]", daemonsetName)
	daemonsetTemplate := shepworkloads.NewDaemonSetTemplate(daemonsetName, p.namespace.Name, testContainerPodTemplate, true, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), createdDaemonSet.Name, daemonsetName)

	p.T().Logf("Waiting for daemonset [%v] to have expected number of available replicas", daemonsetName)
	err = charts.WatchAndWaitDaemonSets(p.client, p.cluster.ID, p.namespace.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	err = validateHostPortSSH(p.client, p.cluster.ID, p.cluster.Name, steveClient, hostPort, daemonsetName, p.namespace.Name)
	require.NoError(p.T(), err)
}

func (p *PortTestSuite) TestNodePort() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	if p.cluster.EnableNetworkPolicy == nil || !*p.cluster.EnableNetworkPolicy {
		p.T().Skip("The Node Port test requires project network enabled.")
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

	err = validateNodePort(p.client, p.cluster.ID, steveClient, nodePort, daemonsetName)
	require.NoError(p.T(), err)
}

func (p *PortTestSuite) TestClusterIP() {
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

	err = validateClusterIP(p.client, p.cluster.Name, steveClient, serviceResp.ID, port, daemonsetName)
	require.NoError(p.T(), err)
}

func (p *PortTestSuite) TestLoadBalancer() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	isEnabled, err := isCloudManagerEnabled(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	if !isEnabled {
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

	err = validateLoadBalancer(p.client, p.cluster.ID, steveClient, nodePort, daemonsetName)
	require.NoError(p.T(), err)
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

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 3, namespace.Name)
	require.NoError(p.T(), err)

	err = validateClusterIP(p.client, p.cluster.Name, steveClient, serviceResp.ID, port, deploymentName)
	require.NoError(p.T(), err)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateClusterIP(p.client, p.cluster.Name, steveClient, serviceResp.ID, port, deploymentName)
	require.NoError(p.T(), err)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateClusterIP(p.client, p.cluster.Name, steveClient, serviceResp.ID, port, deploymentName)
	require.NoError(p.T(), err)
}

func (p *PortTestSuite) TestHostPortScaleAndUpgrade() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	steveClient, err := p.client.Steve.ProxyDownstream(p.cluster.ID)
	require.NoError(p.T(), err)

	isPool, err := IsNodePoolSizeValid(steveClient)
	require.NoError(p.T(), err)

	if !isPool {
		p.T().Skip("The Host Port scale up/down test requires at least 3 worker nodes.")
	}

	if p.cluster.EnableNetworkPolicy == nil || !*p.cluster.EnableNetworkPolicy {
		p.T().Skip("The Host Port scale up/down test requires project network enabled.")
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

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 3, namespace.Name)
	require.NoError(p.T(), err)

	err = validateHostPortSSH(p.client, p.cluster.ID, p.cluster.Name, steveClient, hostPort, deploymentName, namespace.Name)
	require.NoError(p.T(), err)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateHostPortSSH(p.client, p.cluster.ID, p.cluster.Name, steveClient, hostPort, deploymentName, namespace.Name)
	require.NoError(p.T(), err)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateHostPortSSH(p.client, p.cluster.ID, p.cluster.Name, steveClient, hostPort, deploymentName, namespace.Name)
	require.NoError(p.T(), err)
}

func (p *PortTestSuite) TestNodePortScaleAndUpgrade() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	if p.cluster.EnableNetworkPolicy == nil || !*p.cluster.EnableNetworkPolicy {
		p.T().Skip("The Node Port scale and upgrade test requires project network enabled.")
	}

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

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 3, namespace.Name)
	require.NoError(p.T(), err)

	err = validateNodePort(p.client, p.cluster.ID, steveClient, nodePort, deploymentName)
	require.NoError(p.T(), err)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateNodePort(p.client, p.cluster.ID, steveClient, nodePort, deploymentName)
	require.NoError(p.T(), err)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateNodePort(p.client, p.cluster.ID, steveClient, nodePort, deploymentName)
	require.NoError(p.T(), err)
}

func (p *PortTestSuite) TestLoadBalanceScaleAndUpgrade() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	isEnabled, err := isCloudManagerEnabled(p.client, p.cluster.ID)
	require.NoError(p.T(), err)

	if !isEnabled {
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

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 3, namespace.Name)
	require.NoError(p.T(), err)

	err = validateLoadBalancer(p.client, p.cluster.ID, steveClient, nodePort, deploymentName)
	require.NoError(p.T(), err)

	log.Info("Scaling down deployment")
	replicas = int32(2)
	deploymentTemplate.Spec.Replicas = &replicas
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateLoadBalancer(p.client, p.cluster.ID, steveClient, nodePort, deploymentName)
	require.NoError(p.T(), err)

	log.Info("Upgrading deployment")
	for _, c := range deploymentTemplate.Spec.Template.Spec.Containers {
		c.Name = namegen.AppendRandomString("test-upgrade")
	}

	log.Info("Updating deployment replicas")
	deploymentTemplate, err = deployment.UpdateDeployment(p.client, p.cluster.ID, namespace.Name, deploymentTemplate, true)
	require.NoError(p.T(), err)

	err = validateWorkload(p.client, p.cluster.ID, deploymentTemplate, containerImage, 2, namespace.Name)
	require.NoError(p.T(), err)

	err = validateLoadBalancer(p.client, p.cluster.ID, steveClient, nodePort, deploymentName)
	require.NoError(p.T(), err)
}

func TestPortTestSuite(t *testing.T) {
	suite.Run(t, new(PortTestSuite))
}
