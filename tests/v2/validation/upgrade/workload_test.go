package upgrade

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/ingresses"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/workloads/daemonsets"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/services"
	"github.com/rancher/rancher/tests/framework/extensions/steve"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/deployments"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpgradeWorkloadTestSuite struct {
	suite.Suite
	session   *session.Session
	client    *rancher.Client
	project   *management.Project
	namespace *v1.SteveAPIObject
	names     *resourceNames
}

func (u *UpgradeWorkloadTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeWorkloadTestSuite) SetupSuite() {
	testSession := session.NewSession(u.T())
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client

	names := newNames()
	u.names = names

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(u.T(), clusterName, "Cluster name to install resources is not set")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(u.T(), err)

	projectName := names.core["projectName"]
	project, err := projects.GetProjectByName(client, clusterID, projectName)
	require.NoError(u.T(), err)

	u.project = project

	if project == nil {
		projectConfig := &management.Project{
			ClusterID: clusterID,
			Name:      projectName,
		}
		createdProject, err := client.Management.Project.Create(projectConfig)
		require.NoError(u.T(), err)
		require.Equal(u.T(), createdProject.Name, projectName)
		u.project = createdProject
	}
}

func (u *UpgradeWorkloadTestSuite) TestWorkloadPreUpgrade() {
	subSession := u.session.NewSession()
	defer subSession.Cleanup()

	client, err := u.client.WithSession(subSession)
	require.NoError(u.T(), err)

	steveClient, err := u.client.Steve.ProxyDownstream(u.project.ClusterID)
	require.NoError(u.T(), err)

	u.T().Logf("Creating namespace with name [%v]", u.names.random["namespaceName"])
	createdNamespace, err := namespaces.CreateNamespace(client, u.names.random["namespaceName"], "{}", map[string]string{}, map[string]string{}, u.project)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdNamespace.Name, u.names.random["namespaceName"])
	u.namespace = createdNamespace

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	u.T().Logf("Creating a deployment with the test container with name [%v]", u.names.random["deploymentName"])
	createdDeployment, err := deployments.CreateDeployment(client, u.project.ClusterID, u.names.random["deploymentName"], u.namespace.Name, testContainerPodTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDeployment.Name, u.names.random["deploymentName"])

	u.T().Logf("Waiting deployment [%v] to have expected number of available replicas", u.names.random["deploymentName"])
	err = charts.WatchAndWaitDeployments(client, u.project.ClusterID, u.namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Creating a daemonset with the test container with name [%v]", u.names.random["daemonsetName"])
	createdDeamontSet, err := daemonsets.CreateDaemonSet(client, u.project.ClusterID, u.names.random["daemonsetName"], u.namespace.Name, testContainerPodTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDeamontSet.Name, u.names.random["daemonsetName"])

	u.T().Logf("Waiting daemonset [%v] to have expected number of available replicas", u.names.random["daemonsetName"])
	err = charts.WatchAndWaitDaemonSets(client, u.project.ClusterID, u.namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Validating daemonset[%v] available replicas number is equal to worker nodes number in the cluster [%v]", u.names.random["daemonsetName"], u.project.ClusterID)
	validateDaemonset(u.T(), client, u.project.ClusterID, u.namespace.Name, u.names.random["daemonsetName"])

	secretTemplate := secrets.NewSecretTemplate(u.names.random["secretName"], u.namespace.Name, map[string][]byte{"test": []byte("test")})

	u.T().Logf("Creating a secret with name [%v]", u.names.random["secretName"])
	createdSecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdSecret.Name, u.names.random["secretName"])

	podTemplateWithSecretVolume := newPodTemplateWithSecretVolume(u.names.random["secretName"])

	u.T().Logf("Creating a deployment with the test container and secret as volume with name [%v]", u.names.random["deploymentNameForVolumeSecret"])
	createdDeploymentWithSecretVolume, err := deployments.CreateDeployment(client, u.project.ClusterID, u.names.random["deploymentNameForVolumeSecret"], u.namespace.Name, podTemplateWithSecretVolume)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDeploymentWithSecretVolume.Name, u.names.random["deploymentNameForVolumeSecret"])

	u.T().Logf("Creating a daemonset with the test container and secret as volume with name [%v]", u.names.random["daemonsetNameForVolumeSecret"])
	createdDaemonsetWithSecretVolume, err := daemonsets.CreateDaemonSet(client, u.project.ClusterID, u.names.random["daemonsetNameForVolumeSecret"], u.namespace.Name, podTemplateWithSecretVolume)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDaemonsetWithSecretVolume.Name, u.names.random["daemonsetNameForVolumeSecret"])

	u.T().Logf("Waiting daemonset [%v] to have expected number of available replicas", u.names.random["daemonsetNameForVolumeSecret"])
	err = charts.WatchAndWaitDaemonSets(client, u.project.ClusterID, u.namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Validating daemonset [%v] available replicas number is equal to worker nodes number in the cluster [%v]", u.names.random["daemonsetNameForVolumeSecret"], u.project.ClusterID)
	validateDaemonset(u.T(), client, u.project.ClusterID, u.namespace.Name, u.names.random["daemonsetNameForVolumeSecret"])

	podTemplateWithSecretEnvironmentVariable := newPodTemplateWithSecretEnvironmentVariable(u.names.random["secretName"])

	u.T().Logf("Creating a deployment with the test container and secret as environment variable with name [%v]", u.names.random["deploymentNameForEnvironmentVariableSecret"])
	createdDeploymentEnvironmentVariableSecret, err := deployments.CreateDeployment(client, u.project.ClusterID, u.names.random["deploymentNameForEnvironmentVariableSecret"], u.namespace.Name, podTemplateWithSecretEnvironmentVariable)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDeploymentEnvironmentVariableSecret.Name, u.names.random["deploymentNameForEnvironmentVariableSecret"])

	u.T().Logf("Creating a daemonset with the test container and secret as environment variable with name [%v]", u.names.random["daemonsetNameForEnvironmentVariableSecret"])
	createdDaemonsetEnvironmentVariableSecret, err := daemonsets.CreateDaemonSet(client, u.project.ClusterID, u.names.random["daemonsetNameForEnvironmentVariableSecret"], u.namespace.Name, podTemplateWithSecretEnvironmentVariable)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDaemonsetEnvironmentVariableSecret.Name, u.names.random["daemonsetNameForEnvironmentVariableSecret"])

	u.T().Logf("Waiting daemonset [%v] to have expected number of available replicas", u.names.random["daemonsetNameForEnvironmentVariableSecret"])
	err = charts.WatchAndWaitDaemonSets(client, u.project.ClusterID, u.namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Validating daemonset [%v] available replicas number is equal to worker nodes number in the cluster [%v]", u.names.random["daemonsetNameForEnvironmentVariableSecret"], u.project.ClusterID)
	validateDaemonset(u.T(), client, u.project.ClusterID, u.namespace.Name, u.names.random["daemonsetNameForEnvironmentVariableSecret"])

	if client.Flags.GetValue(environmentflag.Ingress) {
		u.T().Log("Ingress tests are enabled")

		u.T().Logf("Creating a deployment with the test container for ingress with name [%v]", u.names.random["deploymentNameForIngress"])
		createdDeploymentForIngress, err := deployments.CreateDeployment(client, u.project.ClusterID, u.names.random["deploymentNameForIngress"], u.namespace.Name, testContainerPodTemplate)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdDeploymentForIngress.Name, u.names.random["deploymentNameForIngress"])

		deploymentForIngressSpec := &appv1.DeploymentSpec{}
		err = v1.ConvertToK8sType(createdDeploymentForIngress.Spec, deploymentForIngressSpec)
		require.NoError(u.T(), err)
		serviceTemplateForDeployment := newServiceTemplate(u.names.random["serviceNameForDeployment"], u.namespace.Name, deploymentForIngressSpec.Template.Labels)

		u.T().Logf("Creating a service linked to the deployment with name [%v]", u.names.random["serviceNameForDeployment"])
		createdServiceForDeployment, err := steveClient.SteveType(services.ServiceSteveType).Create(serviceTemplateForDeployment)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdServiceForDeployment.Name, u.names.random["serviceNameForDeployment"])

		ingressTemplateForDeployment := newIngressTemplate(u.names.random["ingressNameForDeployment"], u.namespace.Name, u.names.random["serviceNameForDeployment"])

		u.T().Logf("Creating an ingress linked to the service [%v] with name [%v]", u.names.random["serviceNameForDeployment"], u.names.random["ingressNameForDeployment"])
		createdIngressForDeployment, err := steveClient.SteveType(ingresses.IngressSteveType).Create(ingressTemplateForDeployment)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdIngressForDeployment.Name, u.names.random["ingressNameForDeployment"])

		u.T().Logf("Waiting ingress [%v] hostname to be ready", u.names.random["ingressNameForDeployment"])
		err = waitUntilIngressHostnameUpdates(client, u.project.ClusterID, u.namespace.Name, u.names.random["ingressNameForDeployment"])
		require.NoError(u.T(), err)

		u.T().Logf("Checking if ingress for deployment with name [%v] is accessible", u.names.random["ingressNameForDeployment"])
		ingressForDeploymentID := getSteveID(u.namespace.Name, u.names.random["ingressNameForDeployment"])
		ingressForDeploymentResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressForDeploymentID)
		require.NoError(u.T(), err)
		ingressForDeploymentSpec := &networkingv1.IngressSpec{}
		err = v1.ConvertToK8sType(ingressForDeploymentResp.Spec, ingressForDeploymentSpec)
		require.NoError(u.T(), err)

		isIngressForDeploymentAccessible, err := waitUntilIngressIsAccessible(client, ingressForDeploymentSpec.Rules[0].Host)
		require.NoError(u.T(), err)
		assert.True(u.T(), isIngressForDeploymentAccessible)

		u.T().Logf("Creating a daemonset with the test container for ingress with name [%v]", u.names.random["daemonsetNameForIngress"])
		createdDeamontSetForIngress, err := daemonsets.CreateDaemonSet(client, u.project.ClusterID, u.names.random["daemonsetNameForIngress"], u.namespace.Name, testContainerPodTemplate)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdDeamontSetForIngress.Name, u.names.random["daemonsetNameForIngress"])

		serviceTemplateForDaemonset := newServiceTemplate(u.names.random["serviceNameForDaemonset"], u.namespace.Name, createdDeamontSetForIngress.Spec.Template.Labels)

		u.T().Logf("Creating a service linked to the daemonset with name [%v]", u.names.random["serviceNameForDaemonset"])
		createdServiceForDaemonset, err := steveClient.SteveType(services.ServiceSteveType).Create(serviceTemplateForDaemonset)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdServiceForDaemonset.Name, u.names.random["serviceNameForDaemonset"])

		ingressTemplateForDaemonset := newIngressTemplate(u.names.random["ingressNameForDaemonset"], u.namespace.Name, u.names.random["serviceNameForDaemonset"])

		u.T().Logf("Creating an ingress linked to the service [%v] with name [%v]", u.names.random["serviceNameForDaemonset"], u.names.random["ingressNameForDaemonset"])
		createdIngressForDaemonset, err := steveClient.SteveType(ingresses.IngressSteveType).Create(ingressTemplateForDaemonset)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdIngressForDaemonset.Name, u.names.random["ingressNameForDaemonset"])

		u.T().Logf("Waiting ingress [%v] hostname to be ready", u.names.random["ingressNameForDaemonset"])
		err = waitUntilIngressHostnameUpdates(client, u.project.ClusterID, u.namespace.Name, u.names.random["ingressNameForDaemonset"])
		require.NoError(u.T(), err)

		u.T().Logf("Checking if ingress for daemonset with name [%v] is accessible", u.names.random["ingressNameForDaemonset"])
		ingressForDaemonsetID := getSteveID(u.namespace.Name, u.names.random["ingressNameForDaemonset"])
		ingressForDaemonsetResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressForDaemonsetID)
		require.NoError(u.T(), err)
		ingressForDaemonsetSpec := &networkingv1.IngressSpec{}
		err = v1.ConvertToK8sType(ingressForDaemonsetResp.Spec, ingressForDaemonsetSpec)
		require.NoError(u.T(), err)

		isIngressForDaemonsetAccessible, err := waitUntilIngressIsAccessible(client, ingressForDaemonsetSpec.Rules[0].Host)
		require.NoError(u.T(), err)
		assert.True(u.T(), isIngressForDaemonsetAccessible)
	}

	if client.Flags.GetValue(environmentflag.Chart) {
		u.T().Log("Charts tests are enabled")

		u.T().Logf("Checking if the logging chart is installed in cluster [%v]", u.project.ClusterID)
		loggingChart, err := charts.GetChartStatus(client, u.project.ClusterID, charts.RancherLoggingNamespace, charts.RancherLoggingName)
		require.NoError(u.T(), err)

		if !loggingChart.IsAlreadyInstalled {
			clusterName, err := clusters.GetClusterNameByID(client, u.project.ClusterID)
			require.NoError(u.T(), err)
			latestLoggingVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherLoggingName)
			require.NoError(u.T(), err)

			loggingChartInstallOption := &charts.InstallOptions{
				ClusterName: clusterName,
				ClusterID:   u.project.ClusterID,
				Version:     latestLoggingVersion,
				ProjectID:   u.project.ID,
			}

			loggingChartFeatureOption := &charts.RancherLoggingOpts{
				AdditionalLoggingSources: true,
			}

			u.T().Logf("Installing logging chart with the latest version in cluster [%v] with version [%v]", u.project.ClusterID, latestLoggingVersion)
			err = charts.InstallRancherLoggingChart(client, loggingChartInstallOption, loggingChartFeatureOption)
			require.NoError(u.T(), err)
		}
	}
}

func (u *UpgradeWorkloadTestSuite) TestWorkloadPostUpgrade() {
	subSession := u.session.NewSession()
	defer subSession.Cleanup()

	client, err := u.client.WithSession(subSession)
	require.NoError(u.T(), err)

	steveClient, err := u.client.Steve.ProxyDownstream(u.project.ClusterID)
	require.NoError(u.T(), err)

	namespaceList, err := steveClient.SteveType(namespaces.NamespaceSteveType).List(nil)
	require.NoError(u.T(), err)
	doesNamespaceExist := containsItemWithPrefix(steve.Names(namespaceList), u.names.core["namespaceName"])
	assert.True(u.T(), doesNamespaceExist)

	if !doesNamespaceExist {
		u.T().Skipf("Namespace with prefix %s doesn't exist", u.names.core["namespaceName"])
	}

	u.T().Logf("Checking if the namespace %s does exist", u.names.core["namespaceName"])
	namespaceID := getItemWithPrefix(steve.Names(namespaceList), u.names.core["namespaceName"])
	namespace, err := steveClient.SteveType(namespaces.NamespaceSteveType).ByID(namespaceID)
	require.NoError(u.T(), err)
	u.namespace = namespace

	u.T().Logf("Checking deployments in namespace %s", u.namespace.Name)
	deploymentList, err := steveClient.SteveType(workloads.DeploymentSteveType).List(nil)
	require.NoError(u.T(), err)
	deploymentNames := []string{
		u.names.coreWithSuffix["deploymentNameForVolumeSecret"],
		u.names.coreWithSuffix["deploymentNameForEnvironmentVariableSecret"],
	}
	for _, expectedDeploymentName := range deploymentNames {
		doesContainDeployment := containsItemWithPrefix(steve.Names(deploymentList), expectedDeploymentName)
		assert.Truef(u.T(), doesContainDeployment, "Deployment with prefix %s doesn't exist", expectedDeploymentName)
	}

	u.T().Logf("Checking daemonsets in namespace %s", u.namespace.Name)
	daemonsetList, err := steveClient.SteveType(workloads.DaemonsetSteveType).List(nil)
	require.NoError(u.T(), err)
	daemonsetNames := []string{
		u.names.coreWithSuffix["daemonsetName"],
	}
	for _, expectedDaemonsetName := range daemonsetNames {
		doesContainDaemonset := containsItemWithPrefix(steve.Names(daemonsetList), expectedDaemonsetName)
		assert.Truef(u.T(), doesContainDaemonset, "Daemonset with prefix %s doesn't exist", expectedDaemonsetName)
	}

	if client.Flags.GetValue(environmentflag.Ingress) {
		u.T().Logf("Ingress tests are enabled")

		u.T().Logf("Checking deployment for ingress in namespace %s", u.namespace.Name)
		doesContainDeploymentForIngress := containsItemWithPrefix(steve.Names(deploymentList), u.names.coreWithSuffix["deploymentNameForIngress"])
		assert.Truef(u.T(), doesContainDeploymentForIngress, "Deployment with prefix %s doesn't exist", u.names.coreWithSuffix["deploymentNameForIngress"])

		u.T().Logf("Checking daemonset for ingress in namespace %s", u.namespace.Name)
		doesContainDaemonsetForIngress := containsItemWithPrefix(steve.Names(daemonsetList), u.names.coreWithSuffix["daemonsetNameForIngress"])
		assert.Truef(u.T(), doesContainDaemonsetForIngress, "Daemonset with prefix %s doesn't exist", u.names.coreWithSuffix["daemonsetNameForIngress"])

		u.T().Logf("Checking ingresses in namespace %s", u.namespace.Name)
		ingressList, err := steveClient.SteveType(ingresses.IngressSteveType).List(nil)
		require.NoError(u.T(), err)
		ingressNames := []string{
			u.names.coreWithSuffix["ingressNameForDeployment"],
			u.names.coreWithSuffix["ingressNameForDaemonset"],
		}
		for _, expectedIngressName := range ingressNames {
			doesContainIngress := containsItemWithPrefix(steve.Names(ingressList), expectedIngressName)
			assert.Truef(u.T(), doesContainIngress, "Ingress with prefix %s doesn't exist", expectedIngressName)

			if doesContainIngress {
				ingressName := getItemWithPrefix(steve.Names(ingressList), expectedIngressName)
				ingressID := getSteveID(u.namespace.Name, ingressName)
				ingressResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressID)
				require.NoError(u.T(), err)
				ingressSpec := &networkingv1.IngressSpec{}
				err = v1.ConvertToK8sType(ingressResp.Spec, ingressSpec)
				require.NoError(u.T(), err)

				u.T().Logf("Checking if the ingress %s is accessible", ingressResp.Name)
				isIngressAcessible, err := waitUntilIngressIsAccessible(client, ingressSpec.Rules[0].Host)
				require.NoError(u.T(), err)
				assert.True(u.T(), isIngressAcessible)
			}
		}
	}

	u.T().Logf("Checking the secret in namespace %s", u.namespace.Name)
	secretList, err := steveClient.SteveType(secrets.SecretSteveType).List(nil)
	require.NoError(u.T(), err)
	doesContainSecret := containsItemWithPrefix(steve.Names(secretList), u.names.core["secretName"])
	assert.Truef(u.T(), doesContainSecret, "Secret with prefix %s doesn't exist", u.names.core["secretName"])

	if client.Flags.GetValue(environmentflag.Chart) {
		u.T().Logf("Chart tests are enabled")

		u.T().Logf("Checking if the logging chart is installed")
		loggingChart, err := charts.GetChartStatus(client, u.project.ClusterID, charts.RancherLoggingNamespace, charts.RancherLoggingName)
		require.NoError(u.T(), err)
		assert.True(u.T(), loggingChart.IsAlreadyInstalled)
	}

	u.T().Logf("Running the pre-upgrade checks")
	u.TestWorkloadPreUpgrade()
}

func TestWorkloadUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeWorkloadTestSuite))
}
