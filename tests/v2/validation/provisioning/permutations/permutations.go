package permutations

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/storageclasses"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/volumes/persistentvolumeclaims"
	"github.com/rancher/rancher/tests/v2/actions/machinepools"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/rancher/tests/v2/actions/rke1/componentchecks"
	"github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscharts "github.com/rancher/shepherd/extensions/charts"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	wloads "github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	RKE2CustomCluster    = "rke2Custom"
	RKE2ProvisionCluster = "rke2"
	RKE2AirgapCluster    = "rke2Airgap"
	K3SCustomCluster     = "k3sCustom"
	K3SProvisionCluster  = "k3s"
	K3SAirgapCluster     = "k3sAirgap"
	RKE1CustomCluster    = "rke1Custom"
	RKE1ProvisionCluster = "rke1"
	RKE1AirgapCluster    = "rke1Airgap"
	CorralProvider       = "corral"

	outOfTreeAWSFilePath = "../resources/out-of-tree/aws.yml"
	clusterIPPrefix      = "cip"
	loadBalancerPrefix   = "lb"
	portName             = "port"
	nginxName            = "nginx"
	defaultNamespace     = "default"

	pollInterval = time.Duration(1 * time.Second)
	pollTimeout  = time.Duration(1 * time.Minute)

	repoType                     = "catalog.cattle.io.clusterrepo"
	appsType                     = "catalog.cattle.io.apps"
	awsUpstreamCloudProviderRepo = "https://github.com/kubernetes/cloud-provider-aws.git"
	masterBranch                 = "master"
	awsUpstreamChartName         = "aws-cloud-controller-manager"
	kubeSystemNamespace          = "kube-system"
	systemProject                = "System"
	externalProviderString       = "external"
	vsphereCPIchartName          = "rancher-vsphere-cpi"
	vsphereCSIchartName          = "rancher-vsphere-csi"
)

var (
	group int64
	user  int64
)

// RunTestPermutations runs through all relevant perumutations in a given config file, including node providers, k8s versions, and CNIs
func RunTestPermutations(s *suite.Suite, testNamePrefix string, client *rancher.Client, provisioningConfig *provisioninginput.Config, clusterType string, hostnameTruncation []machinepools.HostnameTruncation, corralPackages *corral.Packages) {
	var name string
	var providers []string
	var testClusterConfig *clusters.ClusterConfig
	var err error

	testSession := session.NewSession()
	defer testSession.Cleanup()
	client, err = client.WithSession(testSession)
	require.NoError(s.T(), err)

	if strings.Contains(clusterType, "Custom") {
		providers = provisioningConfig.NodeProviders
	} else if strings.Contains(clusterType, "Airgap") {
		providers = []string{"Corral"}
	} else {
		providers = provisioningConfig.Providers
	}

	for _, nodeProviderName := range providers {

		nodeProvider, rke1Provider, customProvider, kubeVersions := GetClusterProvider(clusterType, nodeProviderName, provisioningConfig)

		for _, kubeVersion := range kubeVersions {
			for _, cni := range provisioningConfig.CNIs {

				testClusterConfig = clusters.ConvertConfigToClusterConfig(provisioningConfig)
				testClusterConfig.CNI = cni
				name = testNamePrefix + " Node Provider: " + nodeProviderName + " Kubernetes version: " + kubeVersion + " cni: " + cni

				clusterObject := &steveV1.SteveAPIObject{}
				rke1ClusterObject := &management.Cluster{}
				nodeTemplate := &nodetemplates.NodeTemplate{}

				s.Run(name, func() {
					if testClusterConfig.CloudProvider == provisioninginput.AWSProviderName.String() {
						byteYaml, err := os.ReadFile(outOfTreeAWSFilePath)
						require.NoError(s.T(), err)
						testClusterConfig.AddOnConfig = &provisioninginput.AddOnConfig{
							AdditionalManifest: string(byteYaml),
						}
					}

					switch clusterType {
					case RKE2ProvisionCluster, K3SProvisionCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						clusterObject, err = provisioning.CreateProvisioningCluster(client, *nodeProvider, testClusterConfig, hostnameTruncation)
						reports.TimeoutClusterReport(clusterObject, err)
						require.NoError(s.T(), err)

						provisioning.VerifyCluster(s.T(), client, testClusterConfig, clusterObject)

					case RKE1ProvisionCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						nodeTemplate, err = rke1Provider.NodeTemplateFunc(client)
						require.NoError(s.T(), err)
						// workaround to simplify config for rke1 clusters with cloud provider set. This will allow external charts to be installed
						// while using the rke2 CloudProvider.
						if testClusterConfig.CloudProvider == provisioninginput.VsphereCloudProviderName.String() {
							testClusterConfig.CloudProvider = "external"
						}

						rke1ClusterObject, err = provisioning.CreateProvisioningRKE1Cluster(client, *rke1Provider, testClusterConfig, nodeTemplate)
						reports.TimeoutRKEReport(rke1ClusterObject, err)
						require.NoError(s.T(), err)

						provisioning.VerifyRKE1Cluster(s.T(), client, testClusterConfig, rke1ClusterObject)

					case RKE2CustomCluster, K3SCustomCluster:
						testClusterConfig.KubernetesVersion = kubeVersion

						clusterObject, err = provisioning.CreateProvisioningCustomCluster(client, customProvider, testClusterConfig)
						reports.TimeoutClusterReport(clusterObject, err)
						require.NoError(s.T(), err)

						provisioning.VerifyCluster(s.T(), client, testClusterConfig, clusterObject)

					case RKE1CustomCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						// workaround to simplify config for rke1 clusters with cloud provider set. This will allow external charts to be installed
						// while using the rke2 CloudProvider name in the
						if testClusterConfig.CloudProvider == provisioninginput.VsphereCloudProviderName.String() {
							testClusterConfig.CloudProvider = "external"
						}

						rke1ClusterObject, nodes, err := provisioning.CreateProvisioningRKE1CustomCluster(client, customProvider, testClusterConfig)
						reports.TimeoutRKEReport(rke1ClusterObject, err)
						require.NoError(s.T(), err)

						provisioning.VerifyRKE1Cluster(s.T(), client, testClusterConfig, rke1ClusterObject)
						etcdVersion, err := componentchecks.CheckETCDVersion(client, nodes, rke1ClusterObject.ID)
						require.NoError(s.T(), err)
						require.NotEmpty(s.T(), etcdVersion)

					// airgap currently uses corral to create nodes and register with rancher
					case RKE2AirgapCluster, K3SAirgapCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						clusterObject, err = provisioning.CreateProvisioningAirgapCustomCluster(client, testClusterConfig, corralPackages)
						reports.TimeoutClusterReport(clusterObject, err)
						require.NoError(s.T(), err)

						provisioning.VerifyCluster(s.T(), client, testClusterConfig, clusterObject)

					case RKE1AirgapCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						// workaround to simplify config for rke1 clusters with cloud provider set. This will allow external charts to be installed
						// while using the rke2 CloudProvider name in the
						if testClusterConfig.CloudProvider == provisioninginput.VsphereCloudProviderName.String() {
							testClusterConfig.CloudProvider = "external"
						}

						clusterObject, err := provisioning.CreateProvisioningRKE1AirgapCustomCluster(client, testClusterConfig, corralPackages)
						reports.TimeoutRKEReport(clusterObject, err)
						require.NoError(s.T(), err)

						provisioning.VerifyRKE1Cluster(s.T(), client, testClusterConfig, clusterObject)

					default:
						s.T().Fatalf("Invalid cluster type: %s", clusterType)
					}

					RunPostClusterCloudProviderChecks(s.T(), client, clusterType, nodeTemplate, testClusterConfig, clusterObject, rke1ClusterObject)
				})
			}
		}
	}
}

// RunPostClusterCloudProviderChecks does additinal checks on the cluster if there's a cloud provider set
// on an active cluster.
func RunPostClusterCloudProviderChecks(t *testing.T, client *rancher.Client, clusterType string, nodeTemplate *nodetemplates.NodeTemplate, testClusterConfig *clusters.ClusterConfig, clusterObject *steveV1.SteveAPIObject, rke1ClusterObject *management.Cluster) {
	if strings.Contains(clusterType, extensionscluster.RKE1ClusterType.String()) {
		adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
		require.NoError(t, err)

		if strings.Contains(testClusterConfig.CloudProvider, provisioninginput.AWSProviderName.String()) {
			if strings.Contains(testClusterConfig.CloudProvider, externalProviderString) {
				clusterMeta, err := extensionscluster.NewClusterMeta(client, rke1ClusterObject.Name)
				require.NoError(t, err)

				err = CreateAndInstallAWSExternalCharts(client, clusterMeta, false)
				require.NoError(t, err)

				podErrors := pods.StatusPods(client, rke1ClusterObject.ID)
				require.Empty(t, podErrors)
			}

			clusterObject, err = adminClient.Steve.SteveType(extensionscluster.ProvisioningSteveResourceType).ByID(provisioninginput.Namespace + "/" + rke1ClusterObject.ID)
			require.NoError(t, err)

			lbServiceResp := CreateCloudProviderWorkloadAndServicesLB(t, client, clusterObject)

			status := &provv1.ClusterStatus{}
			err = steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(t, err)

			services.VerifyAWSLoadBalancer(t, client, lbServiceResp, status.ClusterName)
		} else if strings.Contains(testClusterConfig.CloudProvider, "external") {
			rke1ClusterObject, err := adminClient.Management.Cluster.ByID(rke1ClusterObject.ID)
			require.NoError(t, err)

			if strings.Contains(rke1ClusterObject.AppliedSpec.DisplayName, provisioninginput.VsphereProviderName.String()) {
				chartConfig := new(charts.Config)
				config.LoadConfig(charts.ConfigurationFileKey, chartConfig)

				err := charts.InstallVsphereOutOfTreeCharts(client, catalog.RancherChartRepo, rke1ClusterObject.Name, !chartConfig.IsUpgradable)
				reports.TimeoutRKEReport(rke1ClusterObject, err)
				require.NoError(t, err)

				podErrors := pods.StatusPods(client, rke1ClusterObject.ID)
				require.Empty(t, podErrors)

				clusterObject, err := adminClient.Steve.SteveType(extensionscluster.ProvisioningSteveResourceType).ByID(provisioninginput.Namespace + "/" + rke1ClusterObject.ID)
				require.NoError(t, err)

				CreatePVCWorkload(t, client, clusterObject)
			}
		}
	} else if strings.Contains(clusterType, extensionscluster.RKE2ClusterType.String()) {
		adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
		require.NoError(t, err)

		if testClusterConfig.CloudProvider == provisioninginput.AWSProviderName.String() {
			clusterObject, err := adminClient.Steve.SteveType(extensionscluster.ProvisioningSteveResourceType).ByID(clusterObject.ID)
			require.NoError(t, err)

			lbServiceResp := CreateCloudProviderWorkloadAndServicesLB(t, client, clusterObject)

			status := &provv1.ClusterStatus{}
			err = steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(t, err)

			services.VerifyAWSLoadBalancer(t, client, lbServiceResp, status.ClusterName)
		}

		if testClusterConfig.CloudProvider == provisioninginput.VsphereCloudProviderName.String() {
			CreatePVCWorkload(t, client, clusterObject)
		}
	}
}

// GetClusterProvider returns a provider object given cluster type, nodeProviderName (for custom clusters) and the provisioningConfig
func GetClusterProvider(clusterType string, nodeProviderName string, provisioningConfig *provisioninginput.Config) (*provisioning.Provider, *provisioning.RKE1Provider, *provisioning.ExternalNodeProvider, []string) {
	var nodeProvider provisioning.Provider
	var rke1NodeProvider provisioning.RKE1Provider
	var customProvider provisioning.ExternalNodeProvider
	var kubeVersions []string

	switch clusterType {
	case RKE2ProvisionCluster:
		nodeProvider = provisioning.CreateProvider(nodeProviderName)
		kubeVersions = provisioningConfig.RKE2KubernetesVersions
	case K3SProvisionCluster:
		nodeProvider = provisioning.CreateProvider(nodeProviderName)
		kubeVersions = provisioningConfig.K3SKubernetesVersions
	case RKE1ProvisionCluster:
		rke1NodeProvider = provisioning.CreateRKE1Provider(nodeProviderName)
		kubeVersions = provisioningConfig.RKE1KubernetesVersions
	case RKE2CustomCluster:
		customProvider = provisioning.ExternalNodeProviderSetup(nodeProviderName)
		kubeVersions = provisioningConfig.RKE2KubernetesVersions
	case K3SCustomCluster:
		customProvider = provisioning.ExternalNodeProviderSetup(nodeProviderName)
		kubeVersions = provisioningConfig.K3SKubernetesVersions
	case RKE1CustomCluster:
		customProvider = provisioning.ExternalNodeProviderSetup(nodeProviderName)
		kubeVersions = provisioningConfig.RKE1KubernetesVersions
	case K3SAirgapCluster:
		kubeVersions = provisioningConfig.K3SKubernetesVersions
	case RKE1AirgapCluster:
		kubeVersions = provisioningConfig.RKE1KubernetesVersions
	case RKE2AirgapCluster:
		kubeVersions = provisioningConfig.RKE2KubernetesVersions
	default:
		panic("Cluster type not found")
	}
	return &nodeProvider, &rke1NodeProvider, &customProvider, kubeVersions
}

// CreateCloudProviderWorkloadAndServicesLB creates a test workload, clusterIP service and LoadBalancer service.
// This should be used when testing cloud provider with in-tree or out-of-tree set on the cluster.
func CreateCloudProviderWorkloadAndServicesLB(t *testing.T, client *rancher.Client, cluster *steveV1.SteveAPIObject) *steveV1.SteveAPIObject {
	status := &provv1.ClusterStatus{}

	err := steveV1.ConvertToK8sType(cluster.Status, status)
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	steveclient, err := adminClient.Steve.ProxyDownstream(status.ClusterName)
	require.NoError(t, err)

	nginxWorkload, err := createNginxDeployment(steveclient, status.ClusterName)
	require.NoError(t, err)

	nginxSpec := &appv1.DeploymentSpec{}
	err = steveV1.ConvertToK8sType(nginxWorkload.Spec, nginxSpec)
	require.NoError(t, err)

	clusterIPserviceName := namegenerator.AppendRandomString(clusterIPPrefix)
	clusterIPserviceTemplate := services.NewServiceTemplate(clusterIPserviceName, defaultNamespace, corev1.ServiceTypeClusterIP, []corev1.ServicePort{{Name: portName, Port: 80}}, nginxSpec.Selector.MatchLabels)
	_, err = steveclient.SteveType(services.ServiceSteveType).Create(clusterIPserviceTemplate)
	require.NoError(t, err)

	lbServiceName := namegenerator.AppendRandomString(loadBalancerPrefix)
	lbServiceTemplate := services.NewServiceTemplate(lbServiceName, defaultNamespace, corev1.ServiceTypeLoadBalancer, []corev1.ServicePort{{Name: portName, Port: 80}}, nginxSpec.Selector.MatchLabels)
	lbServiceResp, err := steveclient.SteveType(services.ServiceSteveType).Create(lbServiceTemplate)
	require.NoError(t, err)
	logrus.Info("loadbalancer created for nginx workload.")

	return lbServiceResp
}

// CreatePVCWorkload creates a workload with a PVC for storage. This helper should be used to test
// storage class functionality, i.e. for an in-tree / out-of-tree cloud provider
func CreatePVCWorkload(t *testing.T, client *rancher.Client, cluster *steveV1.SteveAPIObject) *steveV1.SteveAPIObject {
	status := &provv1.ClusterStatus{}
	err := steveV1.ConvertToK8sType(cluster.Status, status)
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	steveclient, err := adminClient.Steve.ProxyDownstream(status.ClusterName)
	require.NoError(t, err)

	dynamicClient, err := client.GetDownStreamClusterClient(status.ClusterName)
	require.NoError(t, err)

	storageClassVolumesResource := dynamicClient.Resource(storageclasses.StorageClassGroupVersionResource).Namespace("")

	ctx := context.Background()
	unstructuredResp, err := storageClassVolumesResource.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)

	storageClasses := &v1.StorageClassList{}

	err = scheme.Scheme.Convert(unstructuredResp, storageClasses, unstructuredResp.GroupVersionKind())
	require.NoError(t, err)

	storageClass := storageClasses.Items[0]

	logrus.Infof("creating PVC")

	accessModes := []corev1.PersistentVolumeAccessMode{
		"ReadWriteOnce",
	}

	persistentVolumeClaim, err := persistentvolumeclaims.CreatePersistentVolumeClaim(
		client,
		status.ClusterName,
		namegenerator.AppendRandomString("pvc"),
		"test-pvc-volume",
		defaultNamespace,
		1,
		accessModes,
		nil,
		&storageClass,
	)
	require.NoError(t, err)

	pvcStatus := &corev1.PersistentVolumeClaimStatus{}
	stevePvc := &steveV1.SteveAPIObject{}

	err = wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
		stevePvc, err = steveclient.SteveType(persistentvolumeclaims.PersistentVolumeClaimType).ByID(defaultNamespace + "/" + persistentVolumeClaim.Name)
		require.NoError(t, err)

		err = steveV1.ConvertToK8sType(stevePvc.Status, pvcStatus)
		require.NoError(t, err)

		if pvcStatus.Phase == persistentvolumeclaims.PersistentVolumeBoundStatus {
			return true, nil
		}
		return false, err
	})
	require.NoError(t, err)

	nginxResponse, err := createNginxDeploymentWithPVC(steveclient, "pvcwkld", persistentVolumeClaim.Name, string(stevePvc.Spec.(map[string]interface{})[persistentvolumeclaims.StevePersistentVolumeClaimVolumeName].(string)))
	require.NoError(t, err)

	return nginxResponse
}

// createNginxDeploymentWithPVC is a helper function that creates a nginx deployment in a cluster's default namespace
func createNginxDeploymentWithPVC(steveclient *steveV1.Client, containerNamePrefix, pvcName, volName string) (*steveV1.SteveAPIObject, error) {
	logrus.Infof("Vol: %s", volName)
	logrus.Infof("Pod: %s", pvcName)

	containerName := namegenerator.AppendRandomString(containerNamePrefix)
	volMount := *&corev1.VolumeMount{
		MountPath: "/auto-mnt",
		Name:      volName,
	}

	podVol := corev1.Volume{
		Name: volName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}

	containerTemplate := wloads.NewContainer(nginxName, nginxName, corev1.PullAlways, []corev1.VolumeMount{volMount}, []corev1.EnvFromSource{}, nil, nil, nil)
	podTemplate := wloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{podVol}, []corev1.LocalObjectReference{}, nil, nil)
	deployment := wloads.NewDeploymentTemplate(containerName, defaultNamespace, podTemplate, true, nil)

	deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	return deploymentResp, err
}

// createNginxDeployment is a helper function that creates a nginx deployment in a cluster's default namespace
func createNginxDeployment(steveclient *steveV1.Client, containerNamePrefix string) (*steveV1.SteveAPIObject, error) {
	containerName := namegenerator.AppendRandomString(containerNamePrefix)

	containerTemplate := wloads.NewContainer(nginxName, nginxName, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)
	podTemplate := wloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil, nil)
	deployment := wloads.NewDeploymentTemplate(containerName, defaultNamespace, podTemplate, true, nil)

	deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	return deploymentResp, err
}

// CreateAndInstallAWSExternalCharts is a helper function for rke1 external-aws cloud provider
// clusters that install the appropriate chart(s) and returns an error, if any.
func CreateAndInstallAWSExternalCharts(client *rancher.Client, cluster *extensionscluster.ClusterMeta, isLeaderMigration bool) error {
	steveclient, err := client.Steve.ProxyDownstream(cluster.ID)
	if err != nil {
		return err
	}

	repoName := namegenerator.AppendRandomString(provisioninginput.AWSProviderName.String())
	err = extensionscharts.CreateChartRepoFromGithub(steveclient, awsUpstreamCloudProviderRepo, masterBranch, repoName)
	if err != nil {
		return err
	}

	project, err := projects.GetProjectByName(client, cluster.ID, systemProject)
	if err != nil {
		return err
	}

	catalogClient, err := client.GetClusterCatalogClient(cluster.ID)
	if err != nil {
		return err
	}

	latestVersion, err := catalogClient.GetLatestChartVersion(awsUpstreamChartName, repoName)
	if err != nil {
		return err
	}

	installOptions := &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestVersion,
		ProjectID: project.ID,
	}
	err = charts.InstallAWSOutOfTreeChart(client, installOptions, repoName, cluster.ID, isLeaderMigration)
	return err
}
