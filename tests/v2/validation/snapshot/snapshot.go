package snapshot

import (
	"fmt"
	"strings"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	extdefault "github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/etcdsnapshot"
	"github.com/rancher/rancher/tests/framework/extensions/ingresses"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/provisioning"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	all                          = "all"
	concurrencyDefaultValue      = "10%"
	containerImage               = "nginx"
	containerName                = "nginx"
	cpConcurrencyValue           = "15%"
	cpUnavailalbleValue          = "1"
	defaultNamespace             = "default"
	DeploymentSteveType          = "apps.deployment"
	isCattleLabeled              = true
	IngressSteveType             = "networking.k8s.io.ingress"
	ingressPath                  = "/index.html"
	initialIngressName           = "ingress-before-restore"
	initialWorkloadName          = "wload-before-restore"
	localClusterName             = "local"
	K3S                          = "k3s"
	kubernetesVersion            = "kubernetesVersion"
	namespace                    = "fleet-default"
	port                         = "port"
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
	RKE1                         = "rke1"
	RKE2                         = "rke2"
	serviceAppendName            = "service-"
	ServiceType                  = "service"
	workerConcurrencyValue       = "20%"
	workerUnavailalbleValue      = "10%"
	WorkloadNamePostBackup       = "wload-after-backup"
)

func snapshotRestore(t *testing.T, client *rancher.Client, clusterName string, etcdRestore *etcdsnapshot.Config, strategy bool) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	localClusterID, err := clusters.GetClusterIDByName(client, localClusterName)
	require.NoError(t, err)

	var isRKE1 bool

	clusterObject, _, _ := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
	if clusterObject == nil {
		_, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		isRKE1 = true
	} else {
		isRKE1 = false
	}

	containerTemplate := workloads.NewContainer(containerName, containerImage, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)
	podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil)
	deployment := workloads.NewDeploymentTemplate(initialWorkloadName, defaultNamespace, podTemplate, isCattleLabeled, nil)

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
			Selector: deployment.Spec.Template.Labels,
		},
	}

	deploymentResp, serviceResp, err := workloads.CreateDeploymentWithService(steveclient, initialWorkloadName, deployment, service)
	require.NoError(t, err)

	err = workloads.VerifyDeployment(steveclient, deploymentResp)
	require.NoError(t, err)
	require.Equal(t, initialWorkloadName, deploymentResp.ObjectMeta.Name)

	path := ingresses.NewIngressPathTemplate(networking.PathTypeExact, ingressPath, serviceAppendName+initialWorkloadName, 80)
	ingressTemplate := ingresses.NewIngressTemplate(initialIngressName, defaultNamespace, "", []networking.HTTPIngressPath{path})

	ingressResp, err := ingresses.CreateIngress(steveclient, initialIngressName, ingressTemplate)
	require.NoError(t, err)
	require.Equal(t, initialIngressName, ingressResp.ObjectMeta.Name)

	if isRKE1 {
		snapshotRestoreRKE1(t, client, podTemplate, deployment, clusterName, clusterID, localClusterID, etcdRestore, strategy, isRKE1)
	} else {
		snapshotRestoreRKE2K3S(t, client, podTemplate, deployment, clusterName, clusterID, localClusterID, etcdRestore, strategy, isRKE1)
	}

	logrus.Infof("Deleting created workloads...")
	err = steveclient.SteveType(DeploymentSteveType).Delete(deploymentResp)
	require.NoError(t, err)

	err = steveclient.SteveType(ServiceType).Delete(serviceResp)
	require.NoError(t, err)

	err = steveclient.SteveType(IngressSteveType).Delete(ingressResp)
	require.NoError(t, err)
}

func snapshotRestoreRKE1(t *testing.T, client *rancher.Client, podTemplate corev1.PodTemplateSpec, deployment *v1.Deployment, clusterName, clusterID, localClusterID string, etcdRestore *etcdsnapshot.Config, strategy, isRKE1 bool) {
	existingSnapshots, err := etcdsnapshot.GetRKE1Snapshots(client, clusterID)
	require.NoError(t, err)

	err = etcdsnapshot.CreateRKE1Snapshot(client, clusterName)
	require.NoError(t, err)

	clusterResp, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	watchInterface, err := client.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	checkFunc := clusters.IsHostedProvisioningClusterReady
	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(t, err)

	podErrors := pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)

	initialKubernetesVersion := clusterResp.RancherKubernetesEngineConfig.Version
	require.Equal(t, initialKubernetesVersion, clusterResp.RancherKubernetesEngineConfig.Version)

	createPostBackupWorkloads(t, client, clusterID, podTemplate, deployment)

	etcdNodeCount, _ := etcdsnapshot.MatchNodeToAnyEtcdRole(client, clusterID)
	snapshotToRestore, err := provisioning.VerifySnapshots(client, localClusterID, clusterName, etcdNodeCount+len(existingSnapshots), isRKE1)
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

		if strategy {
			clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane = cpUnavailalbleValue
			clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker = workerUnavailalbleValue
		}

		_, err = client.Management.Cluster.Update(clusterResp, &clusterResp)
		require.NoError(t, err)

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		require.NoError(t, err)

		nodestat.AllManagementNodeReady(client, clusterResp.ID, extdefault.ThirtyMinuteTimeout)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)
		require.Equal(t, etcdRestore.UpgradeKubernetesVersion, clusterResp.RancherKubernetesEngineConfig.Version)

		if strategy {
			require.Equal(t, cpUnavailalbleValue, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			require.Equal(t, workerUnavailalbleValue, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)
		}
	}

	snapshotRKE1Restore := &management.RestoreFromEtcdBackupInput{
		EtcdBackupID:     snapshotToRestore,
		RestoreRkeConfig: etcdRestore.SnapshotRestore,
	}

	err = etcdsnapshot.RestoreRKE1Snapshot(client, clusterName, snapshotRKE1Restore, initialKubernetesVersion)
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	clusterResp, err = client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	nodestat.AllManagementNodeReady(client, clusterResp.ID, extdefault.ThirtyMinuteTimeout)

	podErrors = pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)
	require.Equal(t, initialKubernetesVersion, clusterResp.RancherKubernetesEngineConfig.Version)

	if etcdRestore.SnapshotRestore == kubernetesVersion || etcdRestore.SnapshotRestore == all {
		clusterResp, err = client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)
		require.Equal(t, initialKubernetesVersion, clusterResp.RancherKubernetesEngineConfig.Version)

		if strategy {
			require.Equal(t, cpUnavailalbleValue, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			require.Equal(t, workerUnavailalbleValue, clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)
		}
	}
}

func snapshotRestoreRKE2K3S(t *testing.T, client *rancher.Client, podTemplate corev1.PodTemplateSpec, deployment *v1.Deployment, clusterName, clusterID, localClusterID string, etcdRestore *etcdsnapshot.Config, strategy, isRKE1 bool) {
	existingSnapshots, err := etcdsnapshot.GetRKE2K3SSnapshots(client, localClusterID, clusterName)
	require.NoError(t, err)

	err = etcdsnapshot.CreateRKE2K3SSnapshot(client, clusterName)
	require.NoError(t, err)

	clusterObject, _, err := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
	require.NoError(t, err)

	steveID := fmt.Sprintf("%s/%s", clusterObject.Namespace, clusterObject.Name)
	err = clusters.WatchAndWaitForCluster(client, steveID)
	require.NoError(t, err)

	podErrors := pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)

	initialKubernetesVersion := clusterObject.Spec.KubernetesVersion
	require.Equal(t, initialKubernetesVersion, clusterObject.Spec.KubernetesVersion)

	createPostBackupWorkloads(t, client, clusterID, podTemplate, deployment)

	etcdNodeCount, _ := etcdsnapshot.MatchNodeToAnyEtcdRole(client, clusterID)
	snapshotToRestore, err := provisioning.VerifySnapshots(client, localClusterID, clusterName, etcdNodeCount+len(existingSnapshots), isRKE1)
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

		if strategy {
			clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency = cpConcurrencyValue
			clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency = workerConcurrencyValue
		}

		_, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(clusterResponse, clusterObject)
		require.NoError(t, err)

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		require.NoError(t, err)

		podErrors := pods.StatusPods(client, clusterID)
		assert.Empty(t, podErrors)
		require.Equal(t, etcdRestore.UpgradeKubernetesVersion, clusterObject.Spec.KubernetesVersion)

		if strategy {
			require.Equal(t, cpConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			require.Equal(t, workerConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
		}
	}

	snapshotRKE2K3SRestore := &rkev1.ETCDSnapshotRestore{
		Name:             snapshotToRestore,
		Generation:       clusterObject.Spec.RKEConfig.ETCDSnapshotCreate.Generation,
		RestoreRKEConfig: etcdRestore.SnapshotRestore,
	}

	err = etcdsnapshot.RestoreRKE2K3SSnapshot(client, clusterName, snapshotRKE2K3SRestore)
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	clusterObject, _, err = clusters.GetProvisioningClusterByName(client, clusterName, namespace)
	require.NoError(t, err)

	podErrors = pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)
	require.Equal(t, initialKubernetesVersion, clusterObject.Spec.KubernetesVersion)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	deploymentList, err := steveclient.SteveType(workloads.DeploymentSteveType).NamespacedSteveClient(defaultNamespace).List(nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(deploymentList.Data))
	require.Equal(t, initialWorkloadName, deploymentList.Data[0].ObjectMeta.Name)

	if etcdRestore.SnapshotRestore == kubernetesVersion || etcdRestore.SnapshotRestore == all {
		clusterObject, _, err := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
		require.NoError(t, err)
		require.Equal(t, initialKubernetesVersion, clusterObject.Spec.KubernetesVersion)

		if strategy {
			require.Equal(t, concurrencyDefaultValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			require.Equal(t, concurrencyDefaultValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
		}
	}
}

func createPostBackupWorkloads(t *testing.T, client *rancher.Client, clusterID string, podTemplate corev1.PodTemplateSpec, deployment *v1.Deployment) {
	postBackupDeployment := workloads.NewDeploymentTemplate(WorkloadNamePostBackup, defaultNamespace, podTemplate, isCattleLabeled, nil)
	postBackupService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAppendName + WorkloadNamePostBackup,
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

	postDeploymentResp, _, err := workloads.CreateDeploymentWithService(steveclient, WorkloadNamePostBackup, postBackupDeployment, postBackupService)
	require.NoError(t, err)

	err = workloads.VerifyDeployment(steveclient, postDeploymentResp)
	require.NoError(t, err)
	require.Equal(t, WorkloadNamePostBackup, postDeploymentResp.ObjectMeta.Name)
}
