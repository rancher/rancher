package upgrade

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	kubeingress "github.com/rancher/rancher/tests/v2/actions/kubeapi/ingresses"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/secrets"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/rancher/tests/v2/actions/upgradeinput"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/ingresses"
	extensionsworkloads "github.com/rancher/shepherd/extensions/workloads"
	wloads "github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubewait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

type resourceNames struct {
	core           map[string]string
	coreWithSuffix map[string]string
	random         map[string]string
}

const (
	containerImage                             = "ranchertest/mytestcontainer"
	containerName                              = "test1"
	daemonsetName                              = "daemonsetName"
	daemonsetNameForEnvironmentVariableSecret  = "daemonsetNameForEnvironmentVariableSecret"
	daemonsetNameForIngress                    = "daemonsetNameForIngress"
	daemonsetNameForVolumeSecret               = "daemonsetNameForVolumeSecret"
	deploymentName                             = "deploymentName"
	deploymentNameForIngress                   = "deploymentNameForIngress"
	deploymentNameForEnvironmentVariableSecret = "deploymentNameForEnvironmentVariableSecret"
	deploymentNameForVolumeSecret              = "deploymentNameForVolumeSecret"
	ingressHostName                            = "sslip.io"
	ingressNameForDaemonset                    = "ingressNameForDaemonset"
	ingressNameForDeployment                   = "ingressNameForDeployment"
	namespaceName                              = "namespaceName"
	projectName                                = "projectName"
	secretAsVolumeName                         = "secret-as-volume"
	secretName                                 = "secretName"
	serviceNameForDaemonset                    = "serviceNameForDaemonset"
	serviceNameForDeployment                   = "serviceNameForDeployment"
	servicePortName                            = "port"
	servicePortNumber                          = 80
	volumeMountPath                            = "/root/usr/"
	windowsContainerImage                      = "mcr.microsoft.com/windows/servercore/iis"
)

// createPreUpgradeWorkloads creates workloads in the downstream cluster before the upgrade.
func createPreUpgradeWorkloads(t *testing.T, client *rancher.Client, clusterName string, featuresToTest upgradeinput.Features, nodeSelector map[string]string, containerImage string) {
	isCattleLabeled := true
	names := newNames()

	project, err := getProject(client, clusterName, names.core[projectName])
	require.NoError(t, err)

	steveClient, err := client.Steve.ProxyDownstream(project.ClusterID)
	require.NoError(t, err)

	logrus.Infof("Creating namespace: %v", names.random[namespaceName])
	namespace, err := namespaces.CreateNamespace(client, names.random[namespaceName], "{}", map[string]string{}, map[string]string{}, project)
	require.NoError(t, err)
	assert.Equal(t, namespace.Name, names.random[namespaceName])

	testContainerPodTemplate := newPodTemplateWithTestContainer(containerImage, nodeSelector)

	logrus.Infof("Creating deployment: %v", names.random[deploymentName])

	deploymentTemplate := wloads.NewDeploymentTemplate(names.random[deploymentName], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
	createdDeployment, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	require.NoError(t, err)
	assert.Equal(t, createdDeployment.Name, names.random[deploymentName])

	logrus.Infof("Waiting for deployment %v to have expected number of available replicas...", names.random[deploymentName])
	err = extensionscharts.WatchAndWaitDeployments(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(t, err)

	logrus.Infof("Creating daemonset: %v", names.random[daemonsetName])

	daemonsetTemplate := wloads.NewDaemonSetTemplate(names.random[daemonsetName], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
	createdDaemonSet, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(t, err)
	assert.Equal(t, createdDaemonSet.Name, names.random[daemonsetName])

	logrus.Infof("Waiting for daemonset %v to have the expected number of available replicas...", names.random[daemonsetName])
	err = extensionscharts.WatchAndWaitDaemonSets(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(t, err)

	secretTemplate := secrets.NewSecretTemplate(names.random[secretName], namespace.Name, map[string][]byte{"test": []byte("test")}, corev1.SecretTypeOpaque)

	logrus.Infof("Creating secret: %v", names.random[secretName])
	createdSecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)
	require.NoError(t, err)
	assert.Equal(t, createdSecret.Name, names.random[secretName])

	podTemplateWithSecretVolume := newPodTemplateWithSecretVolume(names.random[secretName], containerImage, nodeSelector)

	logrus.Infof("Creating deployment %v with the test container and secret as volume...", names.random[deploymentNameForVolumeSecret])

	deploymentWithSecretTemplate := wloads.NewDeploymentTemplate(names.random[deploymentNameForVolumeSecret], namespace.Name, podTemplateWithSecretVolume, isCattleLabeled, nil)
	createdDeploymentWithSecretVolume, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentWithSecretTemplate)
	require.NoError(t, err)
	assert.Equal(t, createdDeploymentWithSecretVolume.Name, names.random[deploymentNameForVolumeSecret])

	logrus.Infof("Creating daemonset %v with the test container and secret as volume...", names.random[daemonsetNameForVolumeSecret])

	daemonsetWithSecretTemplate := wloads.NewDaemonSetTemplate(names.random[daemonsetNameForVolumeSecret], namespace.Name, podTemplateWithSecretVolume, isCattleLabeled, nil)
	createdDaemonSetWithSecretVolume, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetWithSecretTemplate)
	require.NoError(t, err)
	assert.Equal(t, createdDaemonSetWithSecretVolume.Name, names.random[daemonsetNameForVolumeSecret])

	logrus.Infof("Waiting for daemonset %v to have the expected number of available replicas", names.random[daemonsetNameForVolumeSecret])
	err = extensionscharts.WatchAndWaitDaemonSets(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(t, err)

	podTemplateWithSecretEnvironmentVariable := newPodTemplateWithSecretEnvironmentVariable(names.random[secretName], containerImage, nodeSelector)

	logrus.Infof("Creating deployment %v with the test container and secret as environment variable...", names.random[deploymentNameForEnvironmentVariableSecret])

	deploymentEnvironmentWithSecretTemplate := wloads.NewDeploymentTemplate(names.random[deploymentNameForEnvironmentVariableSecret], namespace.Name, podTemplateWithSecretEnvironmentVariable, isCattleLabeled, nil)
	createdDeploymentEnvironmentVariableSecret, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentEnvironmentWithSecretTemplate)
	require.NoError(t, err)
	assert.Equal(t, createdDeploymentEnvironmentVariableSecret.Name, names.random[deploymentNameForEnvironmentVariableSecret])

	logrus.Infof("Creating daemonset %v with the test container and secret as environment variable...", names.random[daemonsetNameForEnvironmentVariableSecret])

	daemonSetEnvironmentWithSecretTemplate := wloads.NewDaemonSetTemplate(names.random[daemonsetNameForEnvironmentVariableSecret], namespace.Name, podTemplateWithSecretEnvironmentVariable, isCattleLabeled, nil)
	createdDaemonSetEnvironmentVariableSecret, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonSetEnvironmentWithSecretTemplate)
	require.NoError(t, err)
	assert.Equal(t, createdDaemonSetEnvironmentVariableSecret.Name, names.random[daemonsetNameForEnvironmentVariableSecret])

	logrus.Infof("Waiting daemonset %v to have expected number of available replicas", names.random[daemonsetNameForEnvironmentVariableSecret])
	err = extensionscharts.WatchAndWaitDaemonSets(client, project.ClusterID, namespace.Name, metav1.ListOptions{})
	require.NoError(t, err)

	if *featuresToTest.Ingress {
		logrus.Infof("Creating deployment %v with the test container for ingress...", names.random[deploymentNameForIngress])

		deploymentForIngressTemplate := wloads.NewDeploymentTemplate(names.random[deploymentNameForIngress], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
		createdDeploymentForIngress, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentForIngressTemplate)
		require.NoError(t, err)
		assert.Equal(t, createdDeploymentForIngress.Name, names.random[deploymentNameForIngress])

		deploymentForIngressSpec := &appv1.DeploymentSpec{}
		err = v1.ConvertToK8sType(createdDeploymentForIngress.Spec, deploymentForIngressSpec)
		require.NoError(t, err)

		logrus.Infof("Creating service %v linked to the deployment...", names.random[serviceNameForDeployment])
		serviceTemplateForDeployment := newServiceTemplate(names.random[serviceNameForDeployment], namespace.Name, deploymentForIngressSpec.Template.Labels)
		createdServiceForDeployment, err := steveClient.SteveType(services.ServiceSteveType).Create(serviceTemplateForDeployment)
		require.NoError(t, err)
		assert.Equal(t, createdServiceForDeployment.Name, names.random[serviceNameForDeployment])

		ingressTemplateForDeployment := newIngressTemplate(names.random[ingressNameForDeployment], namespace.Name, names.random[serviceNameForDeployment])

		logrus.Infof("Creating ingress %v linked to the service %v", names.random[ingressNameForDeployment], names.random[serviceNameForDeployment])
		createdIngressForDeployment, err := steveClient.SteveType(ingresses.IngressSteveType).Create(ingressTemplateForDeployment)
		require.NoError(t, err)
		assert.Equal(t, createdIngressForDeployment.Name, names.random[ingressNameForDeployment])

		logrus.Infof("Waiting for ingress %v hostname to be ready...", names.random[ingressNameForDeployment])
		err = waitUntilIngressHostnameUpdates(client, project.ClusterID, namespace.Name, names.random[ingressNameForDeployment])
		require.NoError(t, err)

		logrus.Infof("Checking if ingress %v is accessible...", names.random[ingressNameForDeployment])
		ingressForDeploymentID := getSteveID(namespace.Name, names.random[ingressNameForDeployment])
		ingressForDeploymentResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressForDeploymentID)
		require.NoError(t, err)

		ingressForDeploymentSpec := &networkingv1.IngressSpec{}
		err = v1.ConvertToK8sType(ingressForDeploymentResp.Spec, ingressForDeploymentSpec)
		require.NoError(t, err)

		isIngressForDeploymentAccessible, err := waitUntilIngressIsAccessible(client, ingressForDeploymentSpec.Rules[0].Host)
		require.NoError(t, err)
		assert.True(t, isIngressForDeploymentAccessible)

		logrus.Infof("Creating daemonset %v with the test container for ingress...", names.random[daemonsetNameForIngress])

		daemonSetForIngressTemplate := wloads.NewDaemonSetTemplate(names.random[daemonsetNameForIngress], namespace.Name, testContainerPodTemplate, isCattleLabeled, nil)
		createdDaemonSetForIngress, err := steveClient.SteveType(workloads.DaemonsetSteveType).Create(daemonSetForIngressTemplate)
		require.NoError(t, err)
		assert.Equal(t, createdDaemonSetForIngress.Name, names.random[daemonsetNameForIngress])

		daemonSetForIngressSpec := &appv1.DaemonSetSpec{}
		err = v1.ConvertToK8sType(createdDaemonSetForIngress.Spec, daemonSetForIngressSpec)
		require.NoError(t, err)

		serviceTemplateForDaemonset := newServiceTemplate(names.random[serviceNameForDaemonset], namespace.Name, daemonSetForIngressSpec.Template.Labels)

		logrus.Infof("Creating service %v linked to the daemonset...", names.random[serviceNameForDaemonset])
		createdServiceForDaemonset, err := steveClient.SteveType(services.ServiceSteveType).Create(serviceTemplateForDaemonset)
		require.NoError(t, err)
		assert.Equal(t, createdServiceForDaemonset.Name, names.random[serviceNameForDaemonset])

		ingressTemplateForDaemonset := newIngressTemplate(names.random[ingressNameForDaemonset], namespace.Name, names.random[serviceNameForDaemonset])

		logrus.Infof("Creating ingress %v linked to the service...", names.random[ingressNameForDaemonset])
		createdIngressForDaemonset, err := steveClient.SteveType(ingresses.IngressSteveType).Create(ingressTemplateForDaemonset)
		require.NoError(t, err)
		assert.Equal(t, createdIngressForDaemonset.Name, names.random[ingressNameForDaemonset])

		logrus.Infof("Waiting for ingress %v hostname to be ready...", names.random[ingressNameForDaemonset])
		err = waitUntilIngressHostnameUpdates(client, project.ClusterID, namespace.Name, names.random[ingressNameForDaemonset])
		require.NoError(t, err)

		logrus.Infof("Checking if ingress %v is accessible", names.random[ingressNameForDaemonset])
		ingressForDaemonsetID := getSteveID(namespace.Name, names.random[ingressNameForDaemonset])
		ingressForDaemonsetResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressForDaemonsetID)
		require.NoError(t, err)
		ingressForDaemonsetSpec := &networkingv1.IngressSpec{}
		err = v1.ConvertToK8sType(ingressForDaemonsetResp.Spec, ingressForDaemonsetSpec)
		require.NoError(t, err)

		isIngressForDaemonsetAccessible, err := waitUntilIngressIsAccessible(client, ingressForDaemonsetSpec.Rules[0].Host)
		require.NoError(t, err)
		assert.True(t, isIngressForDaemonsetAccessible)
	}

	if *featuresToTest.Chart {
		logrus.Infof("Checking if the logging chart is installed in cluster: %v", project.ClusterID)
		loggingChart, err := extensionscharts.GetChartStatus(client, project.ClusterID, charts.RancherLoggingNamespace, charts.RancherLoggingName)
		require.NoError(t, err)

		if !loggingChart.IsAlreadyInstalled {
			cluster, err := clusters.NewClusterMeta(client, clusterName)
			require.NoError(t, err)
			latestLoggingVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherLoggingName, catalog.RancherChartRepo)
			require.NoError(t, err)

			loggingChartInstallOption := &charts.InstallOptions{
				Cluster:   cluster,
				Version:   latestLoggingVersion,
				ProjectID: project.ID,
			}

			loggingChartFeatureOption := &charts.RancherLoggingOpts{
				AdditionalLoggingSources: true,
			}

			logrus.Infof("Installing logging chart's latest version: %v", latestLoggingVersion)
			err = charts.InstallRancherLoggingChart(client, loggingChartInstallOption, loggingChartFeatureOption)
			require.NoError(t, err)

			logrus.Infof("Successfully installed logging chart in cluster: %v", project.ClusterID)
		} else {
			logrus.Infof("Logging chart is already installed in cluster: %v", project.ClusterID)
		}
	}
}

// createPostUpgradeWorkloads creates workloads in the downstream cluster after the upgrade.
func createPostUpgradeWorkloads(t *testing.T, client *rancher.Client, clusterName string, featuresToTest upgradeinput.Features) {
	names := newNames()

	project, err := getProject(client, clusterName, names.core[projectName])
	require.NoError(t, err)

	steveClient, err := client.Steve.ProxyDownstream(project.ClusterID)
	require.NoError(t, err)

	namespaceList, err := steveClient.SteveType(namespaces.NamespaceSteveType).List(nil)
	require.NoError(t, err)
	doesNamespaceExist := containsItemWithPrefix(namespaceList.Names(), names.core[namespaceName])
	assert.True(t, doesNamespaceExist)

	if !doesNamespaceExist {
		t.Skipf("Namespace with prefix %s doesn't exist", names.core[namespaceName])
	}

	logrus.Infof("Checking if the namespace %s does exist...", names.core[namespaceName])
	namespaceID := getItemWithPrefix(namespaceList.Names(), names.core[namespaceName])
	namespace, err := steveClient.SteveType(namespaces.NamespaceSteveType).ByID(namespaceID)
	require.NoError(t, err)

	logrus.Infof("Checking deployments in namespace: %s", namespace.Name)
	deploymentList, err := steveClient.SteveType(workloads.DeploymentSteveType).List(nil)
	require.NoError(t, err)
	deploymentNames := []string{
		names.coreWithSuffix[deploymentNameForVolumeSecret],
		names.coreWithSuffix[deploymentNameForEnvironmentVariableSecret],
	}

	for _, expectedDeploymentName := range deploymentNames {
		doesContainDeployment := containsItemWithPrefix(deploymentList.Names(), expectedDeploymentName)
		assert.Truef(t, doesContainDeployment, "Deployment with prefix %s doesn't exist", expectedDeploymentName)
	}

	logrus.Infof("Checking daemonsets in namespace %s", namespace.Name)
	daemonsetList, err := steveClient.SteveType(workloads.DaemonsetSteveType).List(nil)
	require.NoError(t, err)
	daemonsetNames := []string{
		names.coreWithSuffix[daemonsetName],
	}

	for _, expectedDaemonsetName := range daemonsetNames {
		doesContainDaemonset := containsItemWithPrefix(daemonsetList.Names(), expectedDaemonsetName)
		assert.Truef(t, doesContainDaemonset, "Daemonset with prefix %s doesn't exist", expectedDaemonsetName)
	}

	if *featuresToTest.Ingress {
		logrus.Infof("Checking deployment for ingress in namespace %s", namespace.Name)
		doesContainDeploymentForIngress := containsItemWithPrefix(deploymentList.Names(), names.coreWithSuffix[deploymentNameForIngress])
		assert.Truef(t, doesContainDeploymentForIngress, "Deployment with prefix %s doesn't exist", names.coreWithSuffix[deploymentNameForIngress])

		logrus.Infof("Checking daemonset for ingress in namespace %s", namespace.Name)
		doesContainDaemonsetForIngress := containsItemWithPrefix(daemonsetList.Names(), names.coreWithSuffix[daemonsetNameForIngress])
		assert.Truef(t, doesContainDaemonsetForIngress, "Daemonset with prefix %s doesn't exist", names.coreWithSuffix[daemonsetNameForIngress])

		logrus.Infof("Checking ingresses in namespace %s", namespace.Name)
		ingressList, err := steveClient.SteveType(ingresses.IngressSteveType).List(nil)
		require.NoError(t, err)
		ingressNames := []string{
			names.coreWithSuffix[ingressNameForDeployment],
			names.coreWithSuffix[ingressNameForDaemonset],
		}

		for _, expectedIngressName := range ingressNames {
			doesContainIngress := containsItemWithPrefix(ingressList.Names(), expectedIngressName)
			assert.Truef(t, doesContainIngress, "Ingress with prefix %s doesn't exist", expectedIngressName)

			if doesContainIngress {
				ingressName := getItemWithPrefix(ingressList.Names(), expectedIngressName)
				ingressID := getSteveID(namespace.Name, ingressName)
				ingressResp, err := steveClient.SteveType(ingresses.IngressSteveType).ByID(ingressID)
				require.NoError(t, err)

				ingressSpec := &networkingv1.IngressSpec{}
				err = v1.ConvertToK8sType(ingressResp.Spec, ingressSpec)
				require.NoError(t, err)

				logrus.Infof("Checking if the ingress %s is accessible", ingressResp.Name)
				isIngressAcessible, err := waitUntilIngressIsAccessible(client, ingressSpec.Rules[0].Host)
				require.NoError(t, err)
				assert.True(t, isIngressAcessible)
			}
		}
	}

	logrus.Infof("Checking the secret in namespace %s", namespace.Name)
	secretList, err := steveClient.SteveType(secrets.SecretSteveType).List(nil)
	require.NoError(t, err)

	doesContainSecret := containsItemWithPrefix(secretList.Names(), names.core[secretName])
	assert.Truef(t, doesContainSecret, "Secret with prefix %s doesn't exist", names.core[secretName])

	if *featuresToTest.Chart {
		logrus.Infof("Checking if the logging chart is installed...")
		loggingChart, err := extensionscharts.GetChartStatus(client, project.ClusterID, charts.RancherLoggingNamespace, charts.RancherLoggingName)
		require.NoError(t, err)
		assert.True(t, loggingChart.IsAlreadyInstalled)
	}
}

func getSteveID(namespaceName, resourceName string) string {
	return fmt.Sprintf(namespaceName + "/" + resourceName)
}

func getProject(client *rancher.Client, clusterName, projectName string) (project *management.Project, err error) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return
	}

	project, err = projects.GetProjectByName(client, clusterID, projectName)
	if err != nil {
		return
	}

	if project == nil {
		projectConfig := &management.Project{
			ClusterID: clusterID,
			Name:      projectName,
		}

		project, err = client.Management.Project.Create(projectConfig)
		if err != nil {
			return nil, err
		}
	}

	return
}

// newIngressTemplate is a private constructor that returns ingress spec for specific services
func newIngressTemplate(ingressName, namespaceName, serviceNameForBackend string) networkingv1.Ingress {
	pathTypePrefix := networkingv1.PathTypeImplementationSpecific
	paths := []networkingv1.HTTPIngressPath{
		{
			Path:     "/",
			PathType: &pathTypePrefix,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: serviceNameForBackend,
					Port: networkingv1.ServiceBackendPort{
						Number: servicePortNumber,
					},
				},
			},
		},
	}

	return ingresses.NewIngressTemplate(ingressName, namespaceName, ingressHostName, paths)
}

// newServiceTemplate is a private constructor that returns service spec for specific workloads
func newServiceTemplate(serviceName, namespaceName string, selector map[string]string) corev1.Service {
	serviceType := corev1.ServiceTypeNodePort
	ports := []corev1.ServicePort{
		{
			Name: servicePortName,
			Port: servicePortNumber,
		},
	}

	return services.NewServiceTemplate(serviceName, namespaceName, serviceType, ports, selector)
}

// newTestContainerMinimal is a private constructor that returns container for minimal workload creations
func newTestContainerMinimal(containerImage string) corev1.Container {
	pullPolicy := corev1.PullAlways

	return wloads.NewContainer(containerName, containerImage, pullPolicy, nil, nil, nil, nil, nil)
}

// newPodTemplateWithTestContainer is a private constructor that returns pod template spec for workload creations
func newPodTemplateWithTestContainer(containerImage string, nodeSelector map[string]string) corev1.PodTemplateSpec {
	testContainer := newTestContainerMinimal(containerImage)
	containers := []corev1.Container{testContainer}
	return extensionsworkloads.NewPodTemplate(containers, nil, nil, nil, nodeSelector)
}

// newPodTemplateWithSecretVolume is a private constructor that returns pod template spec with volume option for workload creations
func newPodTemplateWithSecretVolume(secretName, containerImage string, nodeSelector map[string]string) corev1.PodTemplateSpec {
	testContainer := newTestContainerMinimal(containerImage)
	testContainer.VolumeMounts = []corev1.VolumeMount{{Name: secretAsVolumeName, MountPath: volumeMountPath}}
	containers := []corev1.Container{testContainer}
	volumes := []corev1.Volume{
		{
			Name: secretAsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		},
	}

	return extensionsworkloads.NewPodTemplate(containers, volumes, nil, nil, nodeSelector)
}

// newPodTemplateWithSecretEnvironmentVariable is a private constructor that returns pod template spec with envFrom option for workload creations
func newPodTemplateWithSecretEnvironmentVariable(secretName, containerImage string, nodeSelector map[string]string) corev1.PodTemplateSpec {
	pullPolicy := corev1.PullAlways
	envFrom := []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			},
		},
	}

	var container corev1.Container

	container = wloads.NewContainer(containerName, containerImage, pullPolicy, nil, envFrom, nil, nil, nil)

	containers := []corev1.Container{container}

	return extensionsworkloads.NewPodTemplate(containers, nil, nil, nil, nodeSelector)
}

// waitUntilIngressIsAccessible waits until the ingress is accessible
func waitUntilIngressIsAccessible(client *rancher.Client, hostname string) (bool, error) {
	err := kubewait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		isIngressAccessible, err := ingresses.IsIngressExternallyAccessible(client, hostname, "", false)
		if err != nil {
			return false, err
		}

		return isIngressAccessible, nil
	})

	if err != nil && strings.Contains(err.Error(), kubewait.ErrWaitTimeout.Error()) {
		return false, nil
	}

	return true, nil
}

// waitUntilIngressHostnameUpdates is a private function to wait until the ingress hostname updates
func waitUntilIngressHostnameUpdates(client *rancher.Client, clusterID, namespace, ingressName string) error {
	timeout := int64(60 * 5)
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}
	adminIngressResource := adminDynamicClient.Resource(kubeingress.IngressesGroupVersionResource).Namespace(namespace)

	watchAppInterface, err := adminIngressResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + ingressName,
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		return err
	}

	return wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		ingressUnstructured := event.Object.(*unstructured.Unstructured)
		ingress := &networkingv1.Ingress{}

		err = scheme.Scheme.Convert(ingressUnstructured, ingress, ingressUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}

		if ingress.Spec.Rules[0].Host != ingressHostName {
			return true, nil
		}
		return false, nil
	})
}

// containsItemWithPrefix returns true if the given slice contains an item with the given prefix
func containsItemWithPrefix(slice []string, expected string) bool {
	for _, s := range slice {
		if checkPrefix(s, expected) {
			return true
		}
	}
	return false
}

// getItemWithPrefix returns the item with the given prefix
func getItemWithPrefix(slice []string, expected string) string {
	for _, s := range slice {
		if checkPrefix(s, expected) {
			return s
		}
	}
	return ""
}

// checkPrefix checks if the given string starts with the given prefix
func checkPrefix(name string, prefix string) bool {
	return strings.HasPrefix(name, prefix)
}

// newNames returns a new resourceNames struct
// it creates a random names with random suffix for each resource by using core and coreWithSuffix names
func newNames() *resourceNames {
	const (
		projectName             = "upgrade-wl-project"
		namespaceName           = "namespace"
		deploymentName          = "deployment"
		daemonsetName           = "daemonset"
		secretName              = "secret"
		serviceName             = "service"
		ingressName             = "ingress"
		defaultRandStringLength = 3
	)

	names := &resourceNames{
		core: map[string]string{
			"projectName":    projectName,
			"namespaceName":  namespaceName,
			"deploymentName": deploymentName,
			"daemonsetName":  daemonsetName,
			"secretName":     secretName,
			"serviceName":    serviceName,
			"ingressName":    ingressName,
		},
		coreWithSuffix: map[string]string{
			"deploymentNameForVolumeSecret":              deploymentName + "-volume-secret",
			"deploymentNameForEnvironmentVariableSecret": deploymentName + "-envar-secret",
			"deploymentNameForIngress":                   deploymentName + "-ingress",
			"daemonsetNameForIngress":                    daemonsetName + "-ingress",
			"daemonsetNameForVolumeSecret":               daemonsetName + "-volume-secret",
			"daemonsetNameForEnvironmentVariableSecret":  daemonsetName + "-envar-secret",
			"serviceNameForDeployment":                   serviceName + "-deployment",
			"serviceNameForDaemonset":                    serviceName + "-daemonset",
			"ingressNameForDeployment":                   ingressName + "-deployment",
			"ingressNameForDaemonset":                    ingressName + "-daemonset",
		},
	}

	names.random = map[string]string{}
	for k, v := range names.coreWithSuffix {
		names.random[k] = v + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	}
	for k, v := range names.core {
		names.random[k] = v + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	}

	return names
}
