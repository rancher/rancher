package upgrade

import (
	"os"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rancherShellSettingID       = "shell-image"
	controlPlaneMatchLabelKey   = "rke.cattle.io/control-plane-role"
	kubeControllerManagerArgKey = "kube-controller-manager-arg"
	enableLeaderMigration       = "enable-leader-migration"
	fleetNamespace              = "fleet-default"

	outOfTreeAWSYamlPath = "../provisioning/resources/out-of-tree/aws.yml"
)

var (
	group int64
	user  int64
)

// runKubectlCommand is a helper that takes a client of a cluster and a command, then creates all needed
// resources in order to run the command on the cluster via a corev1.job
func runKubectlCommand(client *rancher.Client, cmd, v3ClusterName string) error {
	jobName := namegen.AppendRandomString("job")
	imageSetting, err := client.Management.Setting.ByID(rancherShellSettingID)
	if err != nil {
		return err
	}

	jobTemplate := workloads.NewJobTemplate(jobName, kubectl.Namespace)
	args := []string{
		cmd,
	}

	command := []string{"/bin/sh", "-c"}
	securityContext := &corev1.SecurityContext{
		RunAsUser:  &user,
		RunAsGroup: &group,
	}
	volumeMount := []corev1.VolumeMount{
		{Name: "config", MountPath: "/root/.kube/"},
	}
	container := workloads.NewContainer(
		jobName, imageSetting.Value, corev1.PullAlways, volumeMount, nil, command, securityContext, args)
	jobTemplate.Spec.Template.Spec.Containers = append(jobTemplate.Spec.Template.Spec.Containers, container)

	return kubectl.CreateJobAndRunKubectlCommands(v3ClusterName, jobName, jobTemplate, client)
}

// enableLeaderMigrationInMachineSelector adds the enable-leader-migration flag to the kube-controller-manager-arg-key
// in the provided []RKESystemConfig in order to enable leader migration. Returns the updated []RKESystemConfig.
func enableLeaderMigrationInMachineSelector(existingMachineSelectorConfig []v1.RKESystemConfig) []v1.RKESystemConfig {
	isExistingControlPlaneSelector := false
	for _, selectorConfig := range existingMachineSelectorConfig {
		if selectorConfig.MachineLabelSelector != nil {
			for key := range selectorConfig.MachineLabelSelector.MatchLabels {
				logrus.Info(key)
				if key == controlPlaneMatchLabelKey {
					selectorConfig.Config.Data[kubeControllerManagerArgKey] = append(
						selectorConfig.Config.Data[kubeControllerManagerArgKey].([]string), enableLeaderMigration)
					logrus.Infof("Output1: %v", selectorConfig)
					isExistingControlPlaneSelector = true
					break
				}
			}
		}
	}

	if !isExistingControlPlaneSelector {
		controlPlaneSelector := v1.RKESystemConfig{
			MachineLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					controlPlaneMatchLabelKey: "true",
				},
			},
			Config: v1.GenericMap{
				Data: map[string]interface{}{
					kubeControllerManagerArgKey: []string{enableLeaderMigration},
				},
			},
		}
		existingMachineSelectorConfig = append(existingMachineSelectorConfig, controlPlaneSelector)
	}

	return existingMachineSelectorConfig
}

// enableLeaderMigrationRKE1 adds the kubeControllerService to the cluster with settings to enable leader migration
// returns the updated management.Cluster
func enableLeaderMigrationRKE1(rke1Cluster *management.Cluster) *management.Cluster {
	rke1ClusterUpdates := rke1Cluster
	rke1ClusterUpdates.RancherKubernetesEngineConfig.Services.KubeController = &management.KubeControllerService{
		ExtraArgs: map[string]string{
			enableLeaderMigration: "true",
		},
	}
	return rke1Cluster
}

// rke1AWSCloudProviderMigration is a helper function to migrate from aws in-tree to out-of-tree on rke1 clusters
func rke1AWSCloudProviderMigration(t *testing.T, client *rancher.Client, clusterName string) {
	clusterID, err := extensionscluster.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)

	rke1Cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	clusterName = rke1Cluster.ID

	_, steveClusterObject, err := extensionscluster.GetProvisioningClusterByName(client, clusterName, fleetNamespace)
	require.NoError(t, err)

	lbServiceResponse := permutations.CreateCloudProviderWorkloadAndServicesLB(t, client, steveClusterObject)

	status := &provv1.ClusterStatus{}
	require.NotNil(t, steveClusterObject)
	err = steveV1.ConvertToK8sType(steveClusterObject.Status, status)
	require.NoError(t, err)

	services.VerifyAWSLoadBalancer(t, client, lbServiceResponse, status.ClusterName)

	spec := &provv1.ClusterSpec{}
	require.NotNil(t, steveClusterObject)
	err = steveV1.ConvertToK8sType(steveClusterObject.Spec, spec)
	require.NoError(t, err)

	newRKE1Cluster := rke1Cluster

	logrus.Info("Enabling leader migration on the cluster.")

	newRKE1Cluster = enableLeaderMigrationRKE1(newRKE1Cluster)
	rke1Cluster, err = client.Management.Cluster.Update(rke1Cluster, newRKE1Cluster)
	require.NoError(t, err)

	err = extensionscluster.WaitClusterToBeUpgraded(client, status.ClusterName)
	require.NoError(t, err)

	podErrors := pods.StatusPods(client, status.ClusterName)
	require.Empty(t, podErrors)

	logrus.Info("Cordoning all control plane nodes in the cluster.")

	err = runKubectlCommand(
		client, "kubectl cordon -l \"node-role.kubernetes.io/controlplane=true\"", status.ClusterName)
	require.NoError(t, err)

	logrus.Info("Upgrading the cluster to preform in-tree to out-of-tree migration.")

	clusterMeta, err := extensionscluster.NewClusterMeta(client, status.ClusterName)
	require.NoError(t, err)

	err = permutations.CreateAndInstallAWSExternalCharts(client, clusterMeta, true)
	require.NoError(t, err)

	newRKE1Cluster = rke1Cluster
	trueBool := true
	newCloudProvider := management.CloudProvider{
		UseInstanceMetadataHostname: &trueBool,
		Name:                        "external-aws",
	}

	newRKE1Cluster.RancherKubernetesEngineConfig.CloudProvider = &newCloudProvider

	_, err = client.Management.Cluster.Update(rke1Cluster, newRKE1Cluster)
	require.NoError(t, err)

	podErrors = pods.StatusPods(client, status.ClusterName)
	require.Empty(t, podErrors)

	err = extensionscluster.WaitClusterToBeUpgraded(client, status.ClusterName)
	require.NoError(t, err)

	podErrors = pods.StatusPods(client, status.ClusterName)
	require.Empty(t, podErrors)

	// rke1 clusters go have a false positive during the upgrade, running it 2x adresses this issue
	err = extensionscluster.WaitClusterToBeUpgraded(client, status.ClusterName)
	require.NoError(t, err)
	_, steveClusterObject, err = extensionscluster.GetProvisioningClusterByName(client, clusterName, fleetNamespace)
	require.NoError(t, err)

	logrus.Info("Verifying in-tree LB persists.")

	services.VerifyAWSLoadBalancer(t, client, lbServiceResponse, status.ClusterName)

	lbServiceResponseOOT := permutations.CreateCloudProviderWorkloadAndServicesLB(t, client, steveClusterObject)

	services.VerifyAWSLoadBalancer(t, client, lbServiceResponseOOT, status.ClusterName)
}

// rke2AWSCloudProviderMigration is a helper function to migrate from aws in-tree to out-of-tree on rke2 clusters
func rke2AWSCloudProviderMigration(t *testing.T, client *rancher.Client, steveClusterObject *steveV1.SteveAPIObject) {
	lbServiceResponse := permutations.CreateCloudProviderWorkloadAndServicesLB(t, client, steveClusterObject)

	status := &provv1.ClusterStatus{}
	require.NotNil(t, steveClusterObject)
	err := steveV1.ConvertToK8sType(steveClusterObject.Status, status)
	require.NoError(t, err)

	services.VerifyAWSLoadBalancer(t, client, lbServiceResponse, status.ClusterName)

	spec := &provv1.ClusterSpec{}
	require.NotNil(t, steveClusterObject)
	err = steveV1.ConvertToK8sType(steveClusterObject.Spec, spec)
	require.NoError(t, err)

	newSteveCluster := steveClusterObject

	logrus.Info("Enabling leader migration on the cluster.")

	spec.RKEConfig.MachineSelectorConfig = enableLeaderMigrationInMachineSelector(spec.RKEConfig.MachineSelectorConfig)
	newSteveCluster.Spec = spec

	steveClusterObject, err = client.Steve.SteveType(
		extensionscluster.ProvisioningSteveResourceType).Update(steveClusterObject, newSteveCluster)
	require.NoError(t, err)

	err = extensionscluster.WaitClusterToBeUpgraded(client, status.ClusterName)
	require.NoError(t, err)

	podErrors := pods.StatusPods(client, status.ClusterName)
	require.Empty(t, podErrors)

	logrus.Info("Cordoning all control plane nodes in the cluster.")

	err = runKubectlCommand(
		client, "kubectl cordon -l \"node-role.kubernetes.io/control-plane=true\"", status.ClusterName)
	require.NoError(t, err)

	logrus.Info("Upgrading the cluster to preform in-tree to out-of-tree migration.")

	require.NotNil(t, steveClusterObject)
	err = steveV1.ConvertToK8sType(steveClusterObject.Spec, spec)
	require.NoError(t, err)

	outOfTreeSystemConfig := clusters.OutOfTreeSystemConfig("aws")
	spec.RKEConfig.MachineSelectorConfig = enableLeaderMigrationInMachineSelector(outOfTreeSystemConfig)

	byteYaml, err := os.ReadFile(outOfTreeAWSYamlPath)
	require.NoError(t, err)
	spec.RKEConfig.AdditionalManifest = string(byteYaml)

	logrus.Info("Enabling out-of-tree provider on the cluster")

	_, steveClusterObject, err = extensionscluster.GetProvisioningClusterByName(client, steveClusterObject.Name, fleetNamespace)
	require.NoError(t, err)

	newSteveCluster = steveClusterObject
	newSteveCluster.Spec = spec

	steveClusterObject, err = client.Steve.SteveType(
		extensionscluster.ProvisioningSteveResourceType).Update(steveClusterObject, newSteveCluster)
	require.NoError(t, err)

	err = extensionscluster.WaitClusterToBeUpgraded(client, status.ClusterName)
	require.NoError(t, err)

	logrus.Info("Uncordoning rke2 after an upgrade doesn't happen automatically - manually uncording control-plane nodes.")

	err = runKubectlCommand(
		client, "kubectl uncordon -l \"node-role.kubernetes.io/control-plane=true\"", status.ClusterName)
	require.NoError(t, err)

	podErrors = pods.StatusPods(client, status.ClusterName)
	require.Empty(t, podErrors)

	logrus.Info("Verifying in-tree LB persists.")

	services.VerifyAWSLoadBalancer(t, client, lbServiceResponse, status.ClusterName)

	lbServiceResponseOOT := permutations.CreateCloudProviderWorkloadAndServicesLB(t, client, steveClusterObject)

	services.VerifyAWSLoadBalancer(t, client, lbServiceResponseOOT, status.ClusterName)
}
