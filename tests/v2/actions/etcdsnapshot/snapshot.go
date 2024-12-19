package etcdsnapshot

import (
	"errors"
	"strings"
	"testing"
	"time"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/services"
	deploy "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	scaling "github.com/rancher/rancher/tests/v2/validation/nodescaling"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	extdefault "github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	extensionsetcdsnapshot "github.com/rancher/shepherd/extensions/etcdsnapshot"
	"github.com/rancher/shepherd/extensions/ingresses"
	extensionsingress "github.com/rancher/shepherd/extensions/ingresses"
	nodestat "github.com/rancher/shepherd/extensions/nodes"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	InitialIngress  = "ingress-before-restore"
	InitialWorkload = "wload-before-restore"

	all                          = "all"
	containerImage               = "nginx"
	containerName                = "nginx"
	defaultNamespace             = "default"
	DeploymentSteveType          = "apps.deployment"
	isCattleLabeled              = true
	ingressSteveType             = "networking.k8s.io.ingress"
	ingressPath                  = "/index.html"
	K3S                          = "k3s"
	kubernetesVersion            = "kubernetesVersion"
	namespace                    = "fleet-default"
	port                         = "port"
	postWorkload                 = "wload-after-backup"
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
	RKE1                         = "rke1"
	RKE2                         = "rke2"
	serviceAppendName            = "service-"
	serviceType                  = "service"
	windowsContainerImage        = "mcr.microsoft.com/windows/servercore/iis"
	windowsContainerName         = "iis"
)

func createAndVerifyResources(steveclient *steveV1.Client) (*corev1.PodTemplateSpec, *v1.Deployment, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject, error) {

	var containerTemplate corev1.Container
	initialIngressName := namegen.AppendRandomString(InitialIngress)
	initialWorkloadName := namegen.AppendRandomString(InitialWorkload)

	containerTemplate = workloads.NewContainer(containerName, containerImage, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)

	podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil, map[string]string{})
	deploymentTemplate := workloads.NewDeploymentTemplate(initialWorkloadName, defaultNamespace, podTemplate, isCattleLabeled, nil)

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAppendName + initialWorkloadName,
			Namespace: defaultNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: port,
					Port: 80,
				},
			},
			Selector: deploymentTemplate.Spec.Template.Labels,
		},
	}

	deploymentResp, err := createDeployment(steveclient, initialWorkloadName, deploymentTemplate)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	err = deploy.VerifyDeployment(steveclient, deploymentResp)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if initialWorkloadName != deploymentResp.ObjectMeta.Name {
		return nil, nil, nil, nil, nil, errors.New("deployment name doesn't match spec")
	}

	serviceResp, err := services.CreateService(steveclient, service)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	err = services.VerifyService(steveclient, serviceResp)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if serviceAppendName+initialWorkloadName != serviceResp.ObjectMeta.Name {
		return nil, nil, nil, nil, nil, errors.New("service name doesn't match spec")
	}

	path := extensionsingress.NewIngressPathTemplate(networking.PathTypeExact, ingressPath, serviceAppendName+initialWorkloadName, 80)
	ingressTemplate := extensionsingress.NewIngressTemplate(initialIngressName, defaultNamespace, "", []networking.HTTPIngressPath{path})

	ingressResp, err := extensionsingress.CreateIngress(steveclient, initialIngressName, ingressTemplate)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	err = ingresses.WaitIngress(steveclient, ingressResp, initialIngressName)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if initialIngressName != ingressResp.ObjectMeta.Name {
		return nil, nil, nil, nil, nil, errors.New("ingress name doesn't match spec")
	}

	return &podTemplate, deploymentTemplate, deploymentResp, serviceResp, ingressResp, nil
}

func SnapshotRestore(t *testing.T, client *rancher.Client, clusterName string, etcdRestore *Config, containerImage string) {

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	var isRKE1 = false

	clusterObject, _, _ := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
	if clusterObject == nil {
		_, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		isRKE1 = true
	}

	podTemplate, deploymentTemplate, deploymentResp, serviceResp, ingressResp, err := createAndVerifyResources(steveclient)
	require.NoError(t, err)

	if isRKE1 {
		cluster, snapshotName, postDeploymentResp, postServiceResp := SnapshotRKE1(t, client, podTemplate, deploymentTemplate, clusterName, clusterID, etcdRestore, isRKE1)
		RestoreRKE1(t, client, snapshotName, etcdRestore, cluster, clusterID)

		_, err = steveclient.SteveType(DeploymentSteveType).ByID(postDeploymentResp.ID)
		require.Error(t, err)

		_, err = steveclient.SteveType(serviceType).ByID(postServiceResp.ID)
		require.Error(t, err)

	} else {
		cluster, snapshotName, postDeploymentResp, postServiceResp := SnapshotV2Prov(t, client, podTemplate, deploymentTemplate, clusterName, clusterID, etcdRestore, isRKE1)
		RestoreV2Prov(t, client, snapshotName, etcdRestore, cluster, clusterID)

		_, err = steveclient.SteveType(DeploymentSteveType).ByID(postDeploymentResp.ID)
		require.Error(t, err)

		_, err = steveclient.SteveType(serviceType).ByID(postServiceResp.ID)
		require.Error(t, err)
	}

	logrus.Infof("Deleting created workloads...")
	err = steveclient.SteveType(DeploymentSteveType).Delete(deploymentResp)
	require.NoError(t, err)

	err = steveclient.SteveType(serviceType).Delete(serviceResp)
	require.NoError(t, err)

	err = steveclient.SteveType(ingressSteveType).Delete(ingressResp)
	require.NoError(t, err)
}

func SnapshotRKE1(t *testing.T, client *rancher.Client, podTemplate *corev1.PodTemplateSpec, deployment *v1.Deployment, clusterName, clusterID string,
	etcdRestore *Config, isRKE1 bool) (*management.Cluster, string, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject) {
	existingSnapshots, err := extensionsetcdsnapshot.GetRKE1Snapshots(client, clusterID)
	require.NoError(t, err)

	err = extensionsetcdsnapshot.CreateRKE1Snapshot(client, clusterName)
	require.NoError(t, err)

	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	if etcdRestore.ReplaceRoles != nil && cluster.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
		scaling.ReplaceRKE1Nodes(t, client, clusterName, etcdRestore.ReplaceRoles.Etcd, etcdRestore.ReplaceRoles.ControlPlane, etcdRestore.ReplaceRoles.Worker)
	}

	podErrors := pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)

	postDeploymentResp, postServiceResp := createPostBackupWorkloads(t, client, clusterID, *podTemplate, deployment)

	etcdNodeCount, _ := MatchNodeToAnyEtcdRole(client, clusterID)
	snapshotToRestore, err := provisioning.VerifySnapshots(client, clusterName, etcdNodeCount+len(existingSnapshots), isRKE1)
	require.NoError(t, err)

	if etcdRestore.SnapshotRestore == kubernetesVersion || etcdRestore.SnapshotRestore == all {
		clusterID, err := clusters.GetClusterIDByName(client, clusterName)
		require.NoError(t, err)

		clusterResp, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if etcdRestore.UpgradeKubernetesVersion == "" {
			defaultVersion, err := kubernetesversions.Default(client, clusters.RKE1ClusterType.String(), nil)
			etcdRestore.UpgradeKubernetesVersion = defaultVersion[0]
			require.NoError(t, err)
		}

		clusterResp.RancherKubernetesEngineConfig.Version = etcdRestore.UpgradeKubernetesVersion

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneUnavailableValue != "" && etcdRestore.WorkerUnavailableValue != "" {
			clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane = etcdRestore.ControlPlaneUnavailableValue
			clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker = etcdRestore.WorkerUnavailableValue
		}

		_, err = client.Management.Cluster.Update(clusterResp, &clusterResp)
		require.NoError(t, err)

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		require.NoError(t, err)

		logrus.Infof("Cluster version is upgraded to: %s", clusterResp.RancherKubernetesEngineConfig.Version)

		nodestat.AllManagementNodeReady(client, clusterResp.ID, extdefault.ThirtyMinuteTimeout)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)
		require.Equal(t, etcdRestore.UpgradeKubernetesVersion, clusterResp.RancherKubernetesEngineConfig.Version)

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneUnavailableValue != "" && etcdRestore.WorkerUnavailableValue != "" {
			logrus.Infof("Control plane unavailable value is set to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			logrus.Infof("Worker unavailable value is set to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)

			require.Equal(t, etcdRestore.ControlPlaneUnavailableValue, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			require.Equal(t, etcdRestore.WorkerUnavailableValue, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)
		}
	}

	return cluster, snapshotToRestore, postDeploymentResp, postServiceResp
}

func RestoreRKE1(t *testing.T, client *rancher.Client, snapshotName string, etcdRestore *Config, oldCluster *management.Cluster, clusterID string) {
	// Give the option to restore the same snapshot multiple times. By default, it is set to 1.
	for i := 0; i < etcdRestore.RecurringRestores; i++ {
		snapshotRKE1Restore := &management.RestoreFromEtcdBackupInput{
			EtcdBackupID:     snapshotName,
			RestoreRkeConfig: etcdRestore.SnapshotRestore,
		}

		err := extensionsetcdsnapshot.RestoreRKE1Snapshot(client, oldCluster.Name, snapshotRKE1Restore)
		require.NoError(t, err)

		nodestat.AllManagementNodeReady(client, oldCluster.ID, extdefault.ThirtyMinuteTimeout)

		clusterResp, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		require.Equal(t, oldCluster.RancherKubernetesEngineConfig.Version, clusterResp.RancherKubernetesEngineConfig.Version)
		logrus.Infof("Cluster version is restored to: %s", clusterResp.RancherKubernetesEngineConfig.Version)

		client, err = client.ReLogin()
		require.NoError(t, err)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneUnavailableValue != "" && etcdRestore.WorkerUnavailableValue != "" {
			logrus.Infof("Control plane unavailable value is restored to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			logrus.Infof("Worker unavailable value is restored to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)

			require.Equal(t, oldCluster.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			require.Equal(t, oldCluster.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)
		}
	}
}

func SnapshotV2Prov(t *testing.T, client *rancher.Client, podTemplate *corev1.PodTemplateSpec, deployment *v1.Deployment, clusterName, clusterID string,
	etcdRestore *Config, isRKE1 bool) (*apisV1.Cluster, string, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject) {
	existingSnapshots, err := extensionsetcdsnapshot.GetRKE2K3SSnapshots(client, clusterName)
	require.NoError(t, err)

	err = extensionsetcdsnapshot.CreateRKE2K3SSnapshot(client, clusterName)
	require.NoError(t, err)

	cluster, _, err := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
	require.NoError(t, err)

	if etcdRestore.ReplaceRoles != nil && cluster.Spec.RKEConfig.ETCD.S3 != nil {
		scaling.ReplaceNodes(t, client, clusterName, etcdRestore.ReplaceRoles.Etcd, etcdRestore.ReplaceRoles.ControlPlane, etcdRestore.ReplaceRoles.Worker)
	}

	podErrors := pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)

	postDeploymentResp, postServiceResp := createPostBackupWorkloads(t, client, clusterID, *podTemplate, deployment)

	etcdNodeCount, _ := MatchNodeToAnyEtcdRole(client, clusterID)
	snapshotToRestore, err := provisioning.VerifySnapshots(client, clusterName, etcdNodeCount+len(existingSnapshots), isRKE1)
	require.NoError(t, err)

	if etcdRestore.SnapshotRestore == kubernetesVersion || etcdRestore.SnapshotRestore == all {
		clusterObject, clusterResponse, err := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
		require.NoError(t, err)

		initialKubernetesVersion := clusterObject.Spec.KubernetesVersion

		if etcdRestore.UpgradeKubernetesVersion == "" {
			if strings.Contains(initialKubernetesVersion, RKE2) {
				defaultVersion, err := kubernetesversions.Default(client, clusters.RKE2ClusterType.String(), nil)
				etcdRestore.UpgradeKubernetesVersion = defaultVersion[0]
				require.NoError(t, err)
			} else if strings.Contains(initialKubernetesVersion, K3S) {
				defaultVersion, err := kubernetesversions.Default(client, clusters.K3SClusterType.String(), nil)
				etcdRestore.UpgradeKubernetesVersion = defaultVersion[0]
				require.NoError(t, err)
			}
		}

		clusterObject.Spec.KubernetesVersion = etcdRestore.UpgradeKubernetesVersion

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneConcurrencyValue != "" && etcdRestore.WorkerConcurrencyValue != "" {
			clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency = etcdRestore.ControlPlaneConcurrencyValue
			clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency = etcdRestore.WorkerConcurrencyValue
		}

		_, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(clusterResponse, clusterObject)
		require.NoError(t, err)

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		require.NoError(t, err)

		logrus.Infof("Cluster version is upgraded to: %s", clusterObject.Spec.KubernetesVersion)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)
		require.Equal(t, etcdRestore.UpgradeKubernetesVersion, clusterObject.Spec.KubernetesVersion)

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneConcurrencyValue != "" && etcdRestore.WorkerConcurrencyValue != "" {
			logrus.Infof("Control plane concurrency value is set to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			logrus.Infof("Worker concurrency value is set to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)

			require.Equal(t, etcdRestore.ControlPlaneConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			require.Equal(t, etcdRestore.WorkerConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
		}
	}

	return cluster, snapshotToRestore, postDeploymentResp, postServiceResp
}

func RestoreV2Prov(t *testing.T, client *rancher.Client, snapshotName string, etcdRestore *Config, cluster *apisV1.Cluster, clusterID string) {
	clusterObject, _, err := clusters.GetProvisioningClusterByName(client, cluster.Name, namespace)
	require.NoError(t, err)

	// Give the option to restore the same snapshot multiple times. By default, it is set to 1.
	for i := 0; i < etcdRestore.RecurringRestores; i++ {
		generation := int(1)
		if clusterObject.Spec.RKEConfig.ETCDSnapshotRestore != nil {
			generation = clusterObject.Spec.RKEConfig.ETCDSnapshotRestore.Generation + 1
		}

		snapshotRKE2K3SRestore := &rkev1.ETCDSnapshotRestore{
			Name:             snapshotName,
			Generation:       generation,
			RestoreRKEConfig: etcdRestore.SnapshotRestore,
		}

		err := extensionsetcdsnapshot.RestoreRKE2K3SSnapshot(client, snapshotRKE2K3SRestore, clusterObject.Name)
		require.NoError(t, err)

		clusterObject, _, err = clusters.GetProvisioningClusterByName(client, cluster.Name, namespace)
		require.NoError(t, err)

		require.Equal(t, cluster.Spec.KubernetesVersion, clusterObject.Spec.KubernetesVersion)
		logrus.Infof("Cluster version is restored to: %s", clusterObject.Spec.KubernetesVersion)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneConcurrencyValue != "" && etcdRestore.WorkerConcurrencyValue != "" {
			logrus.Infof("Control plane concurrency value is restored to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			logrus.Infof("Worker concurrency value is restored to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)

			require.Equal(t, cluster.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			require.Equal(t, cluster.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
		}
	}
}

func createPostBackupWorkloads(t *testing.T, client *rancher.Client, clusterID string, podTemplate corev1.PodTemplateSpec, deployment *v1.Deployment) (*steveV1.SteveAPIObject, *steveV1.SteveAPIObject) {
	workloadNamePostBackup := namegen.AppendRandomString(postWorkload)

	postBackupDeployment := workloads.NewDeploymentTemplate(workloadNamePostBackup, defaultNamespace, podTemplate, isCattleLabeled, nil)
	postBackupService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAppendName + workloadNamePostBackup,
			Namespace: defaultNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: port,
					Port: 80,
				},
			},
			Selector: deployment.Spec.Template.Labels,
		},
	}

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	postDeploymentResp, err := createDeployment(steveclient, workloadNamePostBackup, postBackupDeployment)
	require.NoError(t, err)

	err = deploy.VerifyDeployment(steveclient, postDeploymentResp)
	require.NoError(t, err)
	require.Equal(t, workloadNamePostBackup, postDeploymentResp.ObjectMeta.Name)

	postServiceResp, err := services.CreateService(steveclient, postBackupService)
	require.NoError(t, err)

	err = services.VerifyService(steveclient, postServiceResp)
	require.NoError(t, err)
	require.Equal(t, serviceAppendName+workloadNamePostBackup, postServiceResp.ObjectMeta.Name)

	return postDeploymentResp, postServiceResp
}

// This function waits for retentionlimit+1 automatic snapshots to be taken before verifying that the retention limit is respected
func CreateSnapshotsUntilRetentionLimit(t *testing.T, client *rancher.Client, clusterName string, retentionLimit int, timeBetweenSnapshots int) {
	v1ClusterID, err := clusters.GetV1ProvisioningClusterByName(client, clusterName)
	if v1ClusterID == "" {
		v3ClusterID, err := clusters.GetClusterIDByName(client, clusterName)
		require.NoError(t, err)
		v1ClusterID = "fleet-default/" + v3ClusterID
	}
	require.NoError(t, err)

	fleetCluster, err := client.Steve.SteveType(stevetypes.FleetCluster).ByID(v1ClusterID)
	require.NoError(t, err)

	provider := fleetCluster.ObjectMeta.Labels["provider.cattle.io"]
	if provider == "rke" {
		sleepNum := (retentionLimit + 1) * timeBetweenSnapshots
		logrus.Infof("Waiting %v hours for %v automatic snapshots to be taken", sleepNum, (retentionLimit + 1))
		time.Sleep(time.Duration(sleepNum)*time.Hour + time.Minute*5)

		err := RKE1RetentionLimitCheck(client, clusterName)
		require.NoError(t, err)

	} else {
		sleepNum := (retentionLimit + 1) * timeBetweenSnapshots
		logrus.Infof("Waiting %v minutes for %v automatic snapshots to be taken", sleepNum, (retentionLimit + 1))
		time.Sleep(time.Duration(sleepNum)*time.Minute + time.Minute*5)

		err := RKE2K3SRetentionLimitCheck(client, clusterName)
		require.NoError(t, err)
	}
}

func createDeployment(steveclient *steveV1.Client, wlName string, deployment *v1.Deployment) (*steveV1.SteveAPIObject, error) {
	logrus.Infof("Creating deployment: %s", wlName)
	deploymentResp, err := steveclient.SteveType(DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	return deploymentResp, err
}
