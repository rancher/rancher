//go:build validation

package upgrade

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/ingresses"
	"github.com/rancher/shepherd/extensions/namespaces"
	"github.com/rancher/shepherd/extensions/secrets"
	"github.com/rancher/shepherd/extensions/services"
	"github.com/rancher/shepherd/extensions/upgradeinput"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpgradeWorkloadTestSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	clusters []upgradeinput.Cluster
}

func (u *UpgradeWorkloadTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeWorkloadTestSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client

	u.clusters, err = upgradeinput.LoadUpgradeWorkloadConfig(client)
	require.NoError(u.T(), err)
}

func (u *UpgradeWorkloadTestSuite) TestWorkloadPreUpgrade() {
	for _, cluster := range u.clusters {
		if cluster.Name == "local" {
			u.T().Skip()
		}

		cluster := cluster
		names := newNames()
		u.Run(cluster.Name, func() {
			u.testPreUpgradeSingleCluster(cluster.Name, cluster.FeaturesToTest, names)
		})
	}
}

func (u *UpgradeWorkloadTestSuite) TestWorkloadPostUpgrade() {
	for _, cluster := range u.clusters {
		if cluster.Name == "local" {
			u.T().Skip()
		}

		cluster := cluster
		names := newNames()
		u.Run(cluster.Name, func() {
			u.testPostUpgradeSingleCluster(cluster.Name, cluster.FeaturesToTest, names)
		})
	}
}

func TestWorkloadUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeWorkloadTestSuite))
}

func (u *UpgradeWorkloadTestSuite) testPreUpgradeSingleCluster(clusterName string, featuresToTest upgradeinput.Features, names *resourceNames) {
	isCattleLabeled := true

	subSession := u.session.NewSession()
	defer subSession.Cleanup()

	client, err := u.client.WithSession(subSession)
	require.NoError(u.T(), err)

	project, err := getProject(client, clusterName, names.core["projectName"])
	require.NoError(u.T(), err)

	steveClient, err := u.client.Steve.ProxyDownstream(project.ClusterID)
	require.NoError(u.T(), err)

	u.T().Logf("Creating namespace with name [%v]", names.random["namespaceName"])
	namespace, err := namespaces.CreateNamespace(client, names.random["namespaceName"], "{}", map[string]string{}, map[string]string{}, project)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), namespace.Name, names.random["namespaceName"])

	testContainerPodTemplate := newPodTemplateWithTestContainer()

	u.T().Logf("Creating a deployment with the test container with name [%v]", names.random["deploymentName"])
	deploymentTemplate := workloads.NewDeploymentTemplate(names.random["deploymentName"], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
	createdDeployment, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDeployment.Name, names.random["deploymentName"])

	u.T().Logf("Waiting deployment [%v] to have expected number of available replicas", names.random["deploymentName"])
	err = charts.WatchAndWaitDeployments(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Creating a daemonset with the test container with name [%v]", names.random["daemonsetName"])
	daemonsetTemplate := workloads.NewDaemonSetTemplate(names.random["daemonsetName"], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDaemonSet.Name, names.random["daemonsetName"])

	u.T().Logf("Waiting daemonset [%v] to have expected number of available replicas", names.random["daemonsetName"])
	err = charts.WatchAndWaitDaemonSets(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Validating daemonset[%v] available replicas number is equal to worker nodes number in the cluster [%v]", names.random["daemonsetName"], project.ClusterID)
	validateDaemonset(u.T(), client, project.ClusterID, namespace.Name, names.random["daemonsetName"])

	secretTemplate := secrets.NewSecretTemplate(names.random["secretName"], namespace.Name, map[string][]byte{"test": []byte("test")}, corev1.SecretTypeOpaque)

	u.T().Logf("Creating a secret with name [%v]", names.random["secretName"])
	createdSecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdSecret.Name, names.random["secretName"])

	podTemplateWithSecretVolume := newPodTemplateWithSecretVolume(names.random["secretName"])

	u.T().Logf("Creating a deployment with the test container and secret as volume with name [%v]", names.random["deploymentNameForVolumeSecret"])
	deploymentWithSecretTemplate := workloads.NewDeploymentTemplate(names.random["deploymentNameForVolumeSecret"], namespace.Name, podTemplateWithSecretVolume, isCattleLabeled, nil)
	createdDeploymentWithSecretVolume, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentWithSecretTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDeploymentWithSecretVolume.Name, names.random["deploymentNameForVolumeSecret"])

	u.T().Logf("Creating a daemonset with the test container and secret as volume with name [%v]", names.random["daemonsetNameForVolumeSecret"])
	daemonsetWithSecretTemplate := workloads.NewDaemonSetTemplate(names.random["daemonsetNameForVolumeSecret"], namespace.Name, podTemplateWithSecretVolume, isCattleLabeled, nil)
	createdDaemonSetWithSecretVolume, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetWithSecretTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDaemonSetWithSecretVolume.Name, names.random["daemonsetNameForVolumeSecret"])

	u.T().Logf("Waiting daemonset [%v] to have expected number of available replicas", names.random["daemonsetNameForVolumeSecret"])
	err = charts.WatchAndWaitDaemonSets(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Validating daemonset [%v] available replicas number is equal to worker nodes number in the cluster [%v]", names.random["daemonsetNameForVolumeSecret"], project.ClusterID)
	validateDaemonset(u.T(), client, project.ClusterID, namespace.Name, names.random["daemonsetNameForVolumeSecret"])

	podTemplateWithSecretEnvironmentVariable := newPodTemplateWithSecretEnvironmentVariable(names.random["secretName"])

	u.T().Logf("Creating a deployment with the test container and secret as environment variable with name [%v]", names.random["deploymentNameForEnvironmentVariableSecret"])
	deploymentEnvironmentWithSecretTemplate := workloads.NewDeploymentTemplate(names.random["deploymentNameForEnvironmentVariableSecret"], namespace.Name, podTemplateWithSecretEnvironmentVariable, isCattleLabeled, nil)
	createdDeploymentEnvironmentVariableSecret, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentEnvironmentWithSecretTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDeploymentEnvironmentVariableSecret.Name, names.random["deploymentNameForEnvironmentVariableSecret"])

	u.T().Logf("Creating a daemonset with the test container and secret as environment variable with name [%v]", names.random["daemonsetNameForEnvironmentVariableSecret"])
	daemonSetEnvironmentWithSecretTemplate := workloads.NewDaemonSetTemplate(names.random["daemonsetNameForEnvironmentVariableSecret"], namespace.Name, podTemplateWithSecretEnvironmentVariable, isCattleLabeled, nil)
	createdDaemonSetEnvironmentVariableSecret, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonSetEnvironmentWithSecretTemplate)
	require.NoError(u.T(), err)
	assert.Equal(u.T(), createdDaemonSetEnvironmentVariableSecret.Name, names.random["daemonsetNameForEnvironmentVariableSecret"])

	u.T().Logf("Waiting daemonset [%v] to have expected number of available replicas", names.random["daemonsetNameForEnvironmentVariableSecret"])
	err = charts.WatchAndWaitDaemonSets(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(u.T(), err)

	u.T().Logf("Validating daemonset [%v] available replicas number is equal to worker nodes number in the cluster [%v]", names.random["daemonsetNameForEnvironmentVariableSecret"], project.ClusterID)
	validateDaemonset(u.T(), client, project.ClusterID, namespace.Name, names.random["daemonsetNameForEnvironmentVariableSecret"])

	if *featuresToTest.Ingress {
		u.T().Log("Ingress tests are enabled")

		u.T().Logf("Creating a deployment with the test container for ingress with name [%v]", names.random["deploymentNameForIngress"])
		deploymentForIngressTemplate := workloads.NewDeploymentTemplate(names.random["deploymentNameForIngress"], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
		createdDeploymentForIngress, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentForIngressTemplate)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdDeploymentForIngress.Name, names.random["deploymentNameForIngress"])

		deploymentForIngressSpec := &appv1.DeploymentSpec{}
		err = v1.ConvertToK8sType(createdDeploymentForIngress.Spec, deploymentForIngressSpec)
		require.NoError(u.T(), err)

		serviceTemplateForDeployment := newServiceTemplate(names.random["serviceNameForDeployment"], namespace.Name, deploymentForIngressSpec.Template.Labels)
		u.T().Logf("Creating a service linked to the deployment with name [%v]", names.random["serviceNameForDeployment"])
		createdServiceForDeployment, err := steveClient.SteveType(services.ServiceSteveType).Create(serviceTemplateForDeployment)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdServiceForDeployment.Name, names.random["serviceNameForDeployment"])

		ingressTemplateForDeployment := newIngressTemplate(names.random["ingressNameForDeployment"], namespace.Name, names.random["serviceNameForDeployment"])

		u.T().Logf("Creating an ingress linked to the service [%v] with name [%v]", names.random["serviceNameForDeployment"], names.random["ingressNameForDeployment"])
		createdIngressForDeployment, err := steveClient.SteveType(ingresses.IngressSteveType).Create(ingressTemplateForDeployment)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdIngressForDeployment.Name, names.random["ingressNameForDeployment"])

		u.T().Logf("Waiting ingress [%v] hostname to be ready", names.random["ingressNameForDeployment"])
		err = waitUntilIngressHostnameUpdates(client, project.ClusterID, namespace.Name, names.random["ingressNameForDeployment"])
		require.NoError(u.T(), err)

		u.T().Logf("Checking if ingress for deployment with name [%v] is accessible", names.random["ingressNameForDeployment"])
		ingressForDeploymentID := getSteveID(namespace.Name, names.random["ingressNameForDeployment"])
		ingressForDeploymentResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressForDeploymentID)
		require.NoError(u.T(), err)
		ingressForDeploymentSpec := &networkingv1.IngressSpec{}
		err = v1.ConvertToK8sType(ingressForDeploymentResp.Spec, ingressForDeploymentSpec)
		require.NoError(u.T(), err)

		isIngressForDeploymentAccessible, err := waitUntilIngressIsAccessible(client, ingressForDeploymentSpec.Rules[0].Host)
		require.NoError(u.T(), err)
		assert.True(u.T(), isIngressForDeploymentAccessible)

		u.T().Logf("Creating a daemonset with the test container for ingress with name [%v]", names.random["daemonsetNameForIngress"])
		daemonSetForIngressTemplate := workloads.NewDaemonSetTemplate(names.random["daemonsetNameForIngress"], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
		createdDaemonSetForIngress, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonSetForIngressTemplate)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdDaemonSetForIngress.Name, names.random["daemonsetNameForIngress"])

		daemonSetForIngressSpec := &appv1.DaemonSetSpec{}
		err = v1.ConvertToK8sType(createdDaemonSetForIngress.Spec, daemonSetForIngressSpec)
		require.NoError(u.T(), err)

		serviceTemplateForDaemonset := newServiceTemplate(names.random["serviceNameForDaemonset"], namespace.Name, daemonSetForIngressSpec.Template.Labels)

		u.T().Logf("Creating a service linked to the daemonset with name [%v]", names.random["serviceNameForDaemonset"])
		createdServiceForDaemonset, err := steveClient.SteveType(services.ServiceSteveType).Create(serviceTemplateForDaemonset)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdServiceForDaemonset.Name, names.random["serviceNameForDaemonset"])

		ingressTemplateForDaemonset := newIngressTemplate(names.random["ingressNameForDaemonset"], namespace.Name, names.random["serviceNameForDaemonset"])

		u.T().Logf("Creating an ingress linked to the service [%v] with name [%v]", names.random["serviceNameForDaemonset"], names.random["ingressNameForDaemonset"])
		createdIngressForDaemonset, err := steveClient.SteveType(ingresses.IngressSteveType).Create(ingressTemplateForDaemonset)
		require.NoError(u.T(), err)
		assert.Equal(u.T(), createdIngressForDaemonset.Name, names.random["ingressNameForDaemonset"])

		u.T().Logf("Waiting ingress [%v] hostname to be ready", names.random["ingressNameForDaemonset"])
		err = waitUntilIngressHostnameUpdates(client, project.ClusterID, namespace.Name, names.random["ingressNameForDaemonset"])
		require.NoError(u.T(), err)

		u.T().Logf("Checking if ingress for daemonset with name [%v] is accessible", names.random["ingressNameForDaemonset"])
		ingressForDaemonsetID := getSteveID(namespace.Name, names.random["ingressNameForDaemonset"])
		ingressForDaemonsetResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressForDaemonsetID)
		require.NoError(u.T(), err)
		ingressForDaemonsetSpec := &networkingv1.IngressSpec{}
		err = v1.ConvertToK8sType(ingressForDaemonsetResp.Spec, ingressForDaemonsetSpec)
		require.NoError(u.T(), err)

		isIngressForDaemonsetAccessible, err := waitUntilIngressIsAccessible(client, ingressForDaemonsetSpec.Rules[0].Host)
		require.NoError(u.T(), err)
		assert.True(u.T(), isIngressForDaemonsetAccessible)
	}

	if *featuresToTest.Chart {
		u.T().Log("Charts tests are enabled")

		u.T().Logf("Checking if the logging chart is installed in cluster [%v]", project.ClusterID)
		loggingChart, err := charts.GetChartStatus(client, project.ClusterID, charts.RancherLoggingNamespace, charts.RancherLoggingName)
		require.NoError(u.T(), err)

		if !loggingChart.IsAlreadyInstalled {
			clusterName, err := clusters.GetClusterNameByID(client, project.ClusterID)
			require.NoError(u.T(), err)
			latestLoggingVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherLoggingName, catalog.RancherChartRepo)
			require.NoError(u.T(), err)

			loggingChartInstallOption := &charts.InstallOptions{
				ClusterName: clusterName,
				ClusterID:   project.ClusterID,
				Version:     latestLoggingVersion,
				ProjectID:   project.ID,
			}

			loggingChartFeatureOption := &charts.RancherLoggingOpts{
				AdditionalLoggingSources: true,
			}

			u.T().Logf("Installing logging chart with the latest version in cluster [%v] with version [%v]", project.ClusterID, latestLoggingVersion)
			err = charts.InstallRancherLoggingChart(client, loggingChartInstallOption, loggingChartFeatureOption)
			require.NoError(u.T(), err)
		}
	}
}

func (u *UpgradeWorkloadTestSuite) testPostUpgradeSingleCluster(clusterName string, featuresToTest upgradeinput.Features, names *resourceNames) {
	subSession := u.session.NewSession()
	defer subSession.Cleanup()

	client, err := u.client.WithSession(subSession)
	require.NoError(u.T(), err)

	project, err := getProject(client, clusterName, names.core["projectName"])
	require.NoError(u.T(), err)

	steveClient, err := u.client.Steve.ProxyDownstream(project.ClusterID)
	require.NoError(u.T(), err)

	namespaceList, err := steveClient.SteveType(namespaces.NamespaceSteveType).List(nil)
	require.NoError(u.T(), err)
	doesNamespaceExist := containsItemWithPrefix(namespaceList.Names(), names.core["namespaceName"])
	assert.True(u.T(), doesNamespaceExist)

	if !doesNamespaceExist {
		u.T().Skipf("Namespace with prefix %s doesn't exist", names.core["namespaceName"])
	}

	u.T().Logf("Checking if the namespace %s does exist", names.core["namespaceName"])
	namespaceID := getItemWithPrefix(namespaceList.Names(), names.core["namespaceName"])
	namespace, err := steveClient.SteveType(namespaces.NamespaceSteveType).ByID(namespaceID)
	require.NoError(u.T(), err)

	u.T().Logf("Checking deployments in namespace %s", namespace.Name)
	deploymentList, err := steveClient.SteveType(workloads.DeploymentSteveType).List(nil)
	require.NoError(u.T(), err)
	deploymentNames := []string{
		names.coreWithSuffix["deploymentNameForVolumeSecret"],
		names.coreWithSuffix["deploymentNameForEnvironmentVariableSecret"],
	}
	for _, expectedDeploymentName := range deploymentNames {
		doesContainDeployment := containsItemWithPrefix(deploymentList.Names(), expectedDeploymentName)
		assert.Truef(u.T(), doesContainDeployment, "Deployment with prefix %s doesn't exist", expectedDeploymentName)
	}

	u.T().Logf("Checking daemonsets in namespace %s", namespace.Name)
	daemonsetList, err := steveClient.SteveType(workloads.DaemonsetSteveType).List(nil)
	require.NoError(u.T(), err)
	daemonsetNames := []string{
		names.coreWithSuffix["daemonsetName"],
	}
	for _, expectedDaemonsetName := range daemonsetNames {
		doesContainDaemonset := containsItemWithPrefix(daemonsetList.Names(), expectedDaemonsetName)
		assert.Truef(u.T(), doesContainDaemonset, "Daemonset with prefix %s doesn't exist", expectedDaemonsetName)
	}

	if *featuresToTest.Ingress {
		u.T().Logf("Ingress tests are enabled")

		u.T().Logf("Checking deployment for ingress in namespace %s", namespace.Name)
		doesContainDeploymentForIngress := containsItemWithPrefix(deploymentList.Names(), names.coreWithSuffix["deploymentNameForIngress"])
		assert.Truef(u.T(), doesContainDeploymentForIngress, "Deployment with prefix %s doesn't exist", names.coreWithSuffix["deploymentNameForIngress"])

		u.T().Logf("Checking daemonset for ingress in namespace %s", namespace.Name)
		doesContainDaemonsetForIngress := containsItemWithPrefix(daemonsetList.Names(), names.coreWithSuffix["daemonsetNameForIngress"])
		assert.Truef(u.T(), doesContainDaemonsetForIngress, "Daemonset with prefix %s doesn't exist", names.coreWithSuffix["daemonsetNameForIngress"])

		u.T().Logf("Checking ingresses in namespace %s", namespace.Name)
		ingressList, err := steveClient.SteveType(ingresses.IngressSteveType).List(nil)
		require.NoError(u.T(), err)
		ingressNames := []string{
			names.coreWithSuffix["ingressNameForDeployment"],
			names.coreWithSuffix["ingressNameForDaemonset"],
		}
		for _, expectedIngressName := range ingressNames {
			doesContainIngress := containsItemWithPrefix(ingressList.Names(), expectedIngressName)
			assert.Truef(u.T(), doesContainIngress, "Ingress with prefix %s doesn't exist", expectedIngressName)

			if doesContainIngress {
				ingressName := getItemWithPrefix(ingressList.Names(), expectedIngressName)
				ingressID := getSteveID(namespace.Name, ingressName)
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

	u.T().Logf("Checking the secret in namespace %s", namespace.Name)
	secretList, err := steveClient.SteveType(secrets.SecretSteveType).List(nil)
	require.NoError(u.T(), err)
	doesContainSecret := containsItemWithPrefix(secretList.Names(), names.core["secretName"])
	assert.Truef(u.T(), doesContainSecret, "Secret with prefix %s doesn't exist", names.core["secretName"])

	if *featuresToTest.Chart {
		u.T().Logf("Chart tests are enabled")

		u.T().Logf("Checking if the logging chart is installed")
		loggingChart, err := charts.GetChartStatus(client, project.ClusterID, charts.RancherLoggingNamespace, charts.RancherLoggingName)
		require.NoError(u.T(), err)
		assert.True(u.T(), loggingChart.IsAlreadyInstalled)
	}

	u.T().Logf("Running the pre-upgrade checks")

	u.testPreUpgradeSingleCluster(clusterName, featuresToTest, names)
}
