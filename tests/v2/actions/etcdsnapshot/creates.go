package etcdsnapshot

import (
	"errors"
	"fmt"
	"strings"
	"time"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	"github.com/rancher/rancher/tests/v2/actions/services"
	deploy "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	extdefault "github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	shepherdsnapshot "github.com/rancher/shepherd/extensions/etcdsnapshot"
	extensionsingress "github.com/rancher/shepherd/extensions/ingresses"
	nodestat "github.com/rancher/shepherd/extensions/nodes"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	InitialIngress  = "ingress-before-restore"
	InitialWorkload = "wload-before-restore"

	all                          = "all"
	nginxImage                   = "nginx"
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

// CreateAndValidateSnapshotRestore is an e2e helper that determines the engine type of the cluster, then takes a snapshot, and finally restores the cluster to the original snapshot
func CreateAndValidateSnapshotRestore(client *rancher.Client, clusterName string, etcdRestore *Config, containerImage string) error {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	var isRKE1 = false

	clusterObject, _, _ := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
	if clusterObject == nil {
		_, err := client.Management.Cluster.ByID(clusterID)
		if err != nil {
			return err
		}

		isRKE1 = true
	}

	podTemplate, deploymentTemplate, deploymentResp, serviceResp, ingressResp, err := createAndVerifyResources(steveclient, containerImage)
	if err != nil {
		return err
	}

	if isRKE1 {
		cluster, snapshotName, postDeploymentResp, postServiceResp, err := CreateAndValidateSnapshotRKE1(client, podTemplate, deploymentTemplate, clusterName, clusterID, etcdRestore, isRKE1)
		if err != nil {
			return err
		}

		err = RestoreAndValidateSnapshotRKE1(client, snapshotName, etcdRestore, cluster, clusterID)
		if err != nil {
			return err
		}

		_, err = steveclient.SteveType(DeploymentSteveType).ByID(postDeploymentResp.ID)
		if err == nil {
			return errors.New("expecting cluster restore to remove resource")
		}

		_, err = steveclient.SteveType(serviceType).ByID(postServiceResp.ID)
		if err == nil {
			return errors.New("expecting cluster restore to remove resource")
		}

	} else {
		cluster, snapshotName, postDeploymentResp, postServiceResp, err := CreateAndValidateSnapshotV2Prov(client, podTemplate, deploymentTemplate, clusterName, clusterID, etcdRestore, isRKE1)
		if err != nil {
			return err
		}

		err = RestoreAndValidateSnapshotV2Prov(client, snapshotName, etcdRestore, cluster, clusterID)
		if err != nil {
			return err
		}

		_, err = steveclient.SteveType(DeploymentSteveType).ByID(postDeploymentResp.ID)
		if err == nil {
			return errors.New("expecting cluster restore to remove resource")
		}

		_, err = steveclient.SteveType(serviceType).ByID(postServiceResp.ID)
		if err == nil {
			return errors.New("expecting cluster restore to remove resource")
		}
	}

	logrus.Infof("Deleting created workloads...")
	err = steveclient.SteveType(DeploymentSteveType).Delete(deploymentResp)
	if err != nil {
		return err
	}

	err = steveclient.SteveType(serviceType).Delete(serviceResp)
	if err != nil {
		return err
	}

	err = steveclient.SteveType(ingressSteveType).Delete(ingressResp)
	if err != nil {
		return err
	}

	return err
}

// CreateAndValidateSnapshotRKE1 is a helper that takes a snapshot of a given rke1 cluster and validates is resources after the snapshot
func CreateAndValidateSnapshotRKE1(client *rancher.Client, podTemplate *corev1.PodTemplateSpec, deployment *v1.Deployment, clusterName, clusterID string,
	etcdRestore *Config, isRKE1 bool) (*management.Cluster, string, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject, error) {

	createdSnapshots, err := shepherdsnapshot.CreateRKE1Snapshot(client, clusterName)
	if err != nil {
		return nil, "", nil, nil, err
	}

	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return nil, "", nil, nil, err
	}

	if etcdRestore.ReplaceRoles != nil && cluster.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
		err = scalinginput.ReplaceRKE1Nodes(client, clusterName, etcdRestore.ReplaceRoles.Etcd, etcdRestore.ReplaceRoles.ControlPlane, etcdRestore.ReplaceRoles.Worker)
		if err != nil {
			return nil, "", nil, nil, err
		}
	}

	snapshotToRestore := createdSnapshots[0].ID
	createdSnapshotIDs := []string{}
	isSnapshotS3 := false

	// prioritize s3 snapshots over local.
	for _, snapshot := range createdSnapshots {
		if snapshot.BackupConfig.S3BackupConfig != nil {
			snapshotToRestore = snapshot.ID
			isSnapshotS3 = true
		}
		createdSnapshotIDs = append(createdSnapshotIDs, snapshot.ID)
	}

	if cluster.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil && !isSnapshotS3 {
		return nil, "", nil, nil, fmt.Errorf("s3 is enabled for the cluster, but selected snapshot is not from s3")
	}

	podErrors := pods.StatusPods(client, clusterID)
	if len(podErrors) != 0 {
		return nil, "", nil, nil, errors.New("cluster's pods not in good health post snapshot")
	}

	postDeploymentResp, postServiceResp, err := createPostBackupWorkloads(client, clusterID, *podTemplate, deployment)
	if err != nil {
		return nil, "", nil, nil, err
	}

	err = VerifyRKE1Snapshots(client, clusterName, createdSnapshotIDs)
	if err != nil {
		return nil, "", nil, nil, err
	}

	if etcdRestore.SnapshotRestore == kubernetesVersion || etcdRestore.SnapshotRestore == all {
		clusterID, err := clusters.GetClusterIDByName(client, clusterName)
		if err != nil {
			return nil, "", nil, nil, err
		}

		clusterResp, err := client.Management.Cluster.ByID(clusterID)
		if err != nil {
			return nil, "", nil, nil, err
		}

		if etcdRestore.UpgradeKubernetesVersion == "" {
			defaultVersion, err := kubernetesversions.Default(client, clusters.RKE1ClusterType.String(), nil)
			etcdRestore.UpgradeKubernetesVersion = defaultVersion[0]
			if err != nil {
				return nil, "", nil, nil, err
			}
		}

		clusterResp.RancherKubernetesEngineConfig.Version = etcdRestore.UpgradeKubernetesVersion

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneUnavailableValue != "" && etcdRestore.WorkerUnavailableValue != "" {
			clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane = etcdRestore.ControlPlaneUnavailableValue
			clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker = etcdRestore.WorkerUnavailableValue
		}

		_, err = client.Management.Cluster.Update(clusterResp, &clusterResp)
		if err != nil {
			return nil, "", nil, nil, err
		}

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		if err != nil {
			return nil, "", nil, nil, err
		}

		logrus.Infof("Cluster version is upgraded to: %s", clusterResp.RancherKubernetesEngineConfig.Version)

		nodestat.AllManagementNodeReady(client, clusterResp.ID, extdefault.ThirtyMinuteTimeout)

		// getting a false positive when restoring rke1. fixing by re-checking the upgrade
		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		if err != nil {
			return nil, "", nil, nil, err
		}
		nodestat.AllManagementNodeReady(client, clusterResp.ID, extdefault.ThirtyMinuteTimeout)

		podErrors := pods.StatusPods(client, clusterID)
		if len(podErrors) != 0 {
			return nil, "", nil, nil, errors.New("cluster's pods not in good health post upgrade")
		}

		if etcdRestore.UpgradeKubernetesVersion != clusterResp.RancherKubernetesEngineConfig.Version {
			return nil, "", nil, nil, fmt.Errorf("K8s Version after upgrade %s does not match expected version %s", clusterResp.RancherKubernetesEngineConfig.Version, etcdRestore.UpgradeKubernetesVersion)
		}

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneUnavailableValue != "" && etcdRestore.WorkerUnavailableValue != "" {
			logrus.Infof("Control plane unavailable value is set to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			logrus.Infof("Worker unavailable value is set to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)

			if etcdRestore.ControlPlaneUnavailableValue != clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane {
				return nil, "", nil, nil, fmt.Errorf("cpUnavailable after upgrade %s does not match expected version %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane, etcdRestore.ControlPlaneUnavailableValue)
			}

			if etcdRestore.WorkerUnavailableValue != clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker {
				return nil, "", nil, nil, fmt.Errorf("cpUnavailable after upgrade %s does not match expected version %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker, etcdRestore.WorkerUnavailableValue)
			}
		}
	}

	return cluster, snapshotToRestore, postDeploymentResp, postServiceResp, nil
}

// RestoreAndValidateSnapshotRKE1 restores a given snapshot for an rke1 cluster and validates its resources after the restore against the original cluster object
func RestoreAndValidateSnapshotRKE1(client *rancher.Client, snapshotName string, etcdRestore *Config, oldCluster *management.Cluster, clusterID string) error {
	// Give the option to restore the same snapshot multiple times. By default, it is set to 1.
	for i := 0; i < etcdRestore.RecurringRestores; i++ {
		snapshotRKE1Restore := &management.RestoreFromEtcdBackupInput{
			EtcdBackupID:     snapshotName,
			RestoreRkeConfig: etcdRestore.SnapshotRestore,
		}

		err := shepherdsnapshot.RestoreRKE1Snapshot(client, oldCluster.Name, snapshotRKE1Restore)
		if err != nil {
			return err
		}

		nodestat.AllManagementNodeReady(client, oldCluster.ID, extdefault.ThirtyMinuteTimeout)

		clusterResp, err := client.Management.Cluster.ByID(clusterID)
		if err != nil {
			return err
		}

		if oldCluster.RancherKubernetesEngineConfig.Version != clusterResp.RancherKubernetesEngineConfig.Version {
			return fmt.Errorf("K8s version after restore %s does not match expected version %s", clusterResp.RancherKubernetesEngineConfig.Version, oldCluster.RancherKubernetesEngineConfig.Version)
		}

		logrus.Infof("Cluster version is restored to: %s", clusterResp.RancherKubernetesEngineConfig.Version)

		client, err = client.ReLogin()
		if err != nil {
			return err
		}

		podErrors := pods.StatusPods(client, clusterID)
		if len(podErrors) != 0 {
			return errors.New("cluster's pods not in good health post restore")
		}

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneUnavailableValue != "" && etcdRestore.WorkerUnavailableValue != "" {
			logrus.Infof("Control plane unavailable value is restored to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			logrus.Infof("Worker unavailable value is restored to: %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)

			if oldCluster.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane != clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane {
				return fmt.Errorf("cpUnavailable after restore %s does not match expected version %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane, oldCluster.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableControlplane)
			}
			if oldCluster.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker != clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker {
				return fmt.Errorf("workerUnavailable after restore %s does not match expected version %s", clusterResp.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker, oldCluster.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker)
			}
		}
	}
	return nil
}

// CreateAndValidateSnapshotV2Prov is a helper that takes a snapshot of a given v2prov cluster and validates is resources after the snapshot
func CreateAndValidateSnapshotV2Prov(client *rancher.Client, podTemplate *corev1.PodTemplateSpec, deployment *v1.Deployment, clusterName, clusterID string,
	etcdRestore *Config, isRKE1 bool) (*apisV1.Cluster, string, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject, error) {

	createdSnapshots, err := shepherdsnapshot.CreateRKE2K3SSnapshot(client, clusterName)
	if err != nil {
		return nil, "", nil, nil, err
	}

	snapshotToRestore := createdSnapshots[0].ID
	createdSnapshotIDs := []string{}
	// prioritize s3 snapshots over local.
	s3Found := false

	for _, snapshot := range createdSnapshots {
		store, ok := snapshot.Annotations["etcdsnapshot.rke.io/storage"]
		if ok && store == "s3" {
			snapshotToRestore = snapshot.ID
			s3Found = true
		}
		createdSnapshotIDs = append(createdSnapshotIDs, snapshot.ID)
	}

	cluster, _, err := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
	if err != nil {
		return nil, "", nil, nil, err
	}

	if cluster.Spec.RKEConfig.ETCD.S3 != nil && !s3Found {
		return nil, "", nil, nil, fmt.Errorf("s3 is enabled for the cluster, but selected snapshot is not from s3")
	}

	if etcdRestore.ReplaceRoles != nil && cluster.Spec.RKEConfig.ETCD.S3 != nil {
		err = scalinginput.ReplaceNodes(client, clusterName, etcdRestore.ReplaceRoles.Etcd, etcdRestore.ReplaceRoles.ControlPlane, etcdRestore.ReplaceRoles.Worker)
		if err != nil {
			return nil, "", nil, nil, err
		}
	}

	podErrors := pods.StatusPods(client, clusterID)
	if len(podErrors) != 0 {
		return nil, "", nil, nil, errors.New("cluster's pods not in good health post snapshot")
	}

	postDeploymentResp, postServiceResp, err := createPostBackupWorkloads(client, clusterID, *podTemplate, deployment)
	if err != nil {
		return nil, "", nil, nil, err
	}

	err = VerifyV2ProvSnapshots(client, clusterName, createdSnapshotIDs)
	if err != nil {
		return nil, "", nil, nil, err
	}

	if etcdRestore.SnapshotRestore == kubernetesVersion || etcdRestore.SnapshotRestore == all {
		clusterObject, clusterResponse, err := clusters.GetProvisioningClusterByName(client, clusterName, namespace)
		if err != nil {
			return nil, "", nil, nil, err
		}

		initialKubernetesVersion := clusterObject.Spec.KubernetesVersion

		if etcdRestore.UpgradeKubernetesVersion == "" {
			if strings.Contains(initialKubernetesVersion, RKE2) {
				defaultVersion, err := kubernetesversions.Default(client, clusters.RKE2ClusterType.String(), nil)
				etcdRestore.UpgradeKubernetesVersion = defaultVersion[0]
				if err != nil {
					return nil, "", nil, nil, err
				}
			} else if strings.Contains(initialKubernetesVersion, K3S) {
				defaultVersion, err := kubernetesversions.Default(client, clusters.K3SClusterType.String(), nil)
				etcdRestore.UpgradeKubernetesVersion = defaultVersion[0]
				if err != nil {
					return nil, "", nil, nil, err
				}
			}
		}

		clusterObject.Spec.KubernetesVersion = etcdRestore.UpgradeKubernetesVersion

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneConcurrencyValue != "" && etcdRestore.WorkerConcurrencyValue != "" {
			clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency = etcdRestore.ControlPlaneConcurrencyValue
			clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency = etcdRestore.WorkerConcurrencyValue
		}

		_, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(clusterResponse, clusterObject)
		if err != nil {
			return nil, "", nil, nil, err
		}

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		if err != nil {
			return nil, "", nil, nil, err
		}

		logrus.Infof("Cluster version is upgraded to: %s", clusterObject.Spec.KubernetesVersion)

		podErrors := pods.StatusPods(client, clusterID)
		if len(podErrors) != 0 {
			return nil, "", nil, nil, errors.New("cluster's pods not in good health post upgrade")
		}

		if etcdRestore.UpgradeKubernetesVersion != clusterObject.Spec.KubernetesVersion {
			return nil, "", nil, nil, fmt.Errorf("K8s Version after upgrade %s does not match expected version %s", clusterObject.Spec.KubernetesVersion, etcdRestore.UpgradeKubernetesVersion)
		}

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneConcurrencyValue != "" && etcdRestore.WorkerConcurrencyValue != "" {
			logrus.Infof("Control plane concurrency value is set to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			logrus.Infof("Worker concurrency value is set to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)

			if etcdRestore.ControlPlaneConcurrencyValue != clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency {
				return nil, "", nil, nil, fmt.Errorf("controlPlaneConcurrency after upgrade %s does not match expected version %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency, etcdRestore.ControlPlaneConcurrencyValue)
			}

			if etcdRestore.WorkerConcurrencyValue != clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency {
				return nil, "", nil, nil, fmt.Errorf("wokerConcurrency after upgrade %s does not match expected version %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency, etcdRestore.WorkerUnavailableValue)
			}
		}
		// sometimes we get a false positive on the cluster's state where it briefly goes 'active'. This is a way to mitigate that.
		clusterSteveObject, err := client.Steve.SteveType(ProvisioningSteveResouceType).ByID(clusterID)
		if err != nil {
			return nil, "", nil, nil, err
		}

		if clusterSteveObject.State == nil {
			err = clusters.WaitClusterUntilUpgrade(client, clusterID)
			if err != nil {
				return nil, "", nil, nil, err
			}

			podErrors := pods.StatusPods(client, clusterID)
			if len(podErrors) != 0 {
				return nil, "", nil, nil, errors.New("cluster's pods not in good health post upgrade")
			}
		}
	}

	return cluster, snapshotToRestore, postDeploymentResp, postServiceResp, err
}

// RestoreAndValidateSnapshotV2Prov restores a given snapshot for a v2prov cluster and validates its resources
// after the restore against the original cluster object
func RestoreAndValidateSnapshotV2Prov(client *rancher.Client, snapshotID string, etcdRestore *Config, cluster *apisV1.Cluster, clusterID string) error {
	clusterObject, _, err := clusters.GetProvisioningClusterByName(client, cluster.Name, namespace)
	if err != nil {
		return err
	}

	// Give the option to restore the same snapshot multiple times. By default, it is set to 1.
	for i := 0; i < etcdRestore.RecurringRestores; i++ {
		generation := int(1)
		if clusterObject.Spec.RKEConfig.ETCDSnapshotRestore != nil {
			generation = clusterObject.Spec.RKEConfig.ETCDSnapshotRestore.Generation + 1
		}

		splitSnapshot := strings.Split(snapshotID, "/")
		snapshotID = splitSnapshot[0]

		if len(splitSnapshot) > 1 {
			snapshotID = splitSnapshot[1]
		}

		snapshotRKE2K3SRestore := &rkev1.ETCDSnapshotRestore{
			Name:             snapshotID,
			Generation:       generation,
			RestoreRKEConfig: etcdRestore.SnapshotRestore,
		}

		err := shepherdsnapshot.RestoreRKE2K3SSnapshot(client, snapshotRKE2K3SRestore, clusterObject.Name)
		if err != nil {
			return err
		}

		clusterObject, _, err = clusters.GetProvisioningClusterByName(client, cluster.Name, namespace)
		if err != nil {
			return err
		}

		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		if err != nil {
			return err
		}

		podErrors := pods.StatusPods(client, clusterID)
		if len(podErrors) != 0 {
			return errors.New("cluster's pods not in good health post restore")
		}

		if cluster.Spec.KubernetesVersion != clusterObject.Spec.KubernetesVersion {
			return fmt.Errorf("K8s Version after upgrade %s does not match expected version %s after restore", clusterObject.Spec.KubernetesVersion, etcdRestore.UpgradeKubernetesVersion)
		}

		if etcdRestore.SnapshotRestore == all && etcdRestore.ControlPlaneConcurrencyValue != "" && etcdRestore.WorkerConcurrencyValue != "" {
			logrus.Infof("Control plane concurrency value is restored to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			logrus.Infof("Worker concurrency value is restored to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)

			if cluster.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency != clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency {
				return fmt.Errorf("controlPlaneConcurrency after restore %s does not match expected version %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency, cluster.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			}

			if cluster.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency != clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency {
				return fmt.Errorf("wokerConcurrency after restore %s does not match expected version %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency, cluster.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			}
		}
	}

	return nil
}

// This function waits for retentionlimit+1 automatic snapshots to be taken before verifying that the retention limit is respected
func CreateSnapshotsUntilRetentionLimit(client *rancher.Client, clusterName string, retentionLimit int, timeBetweenSnapshots int) error {
	v1ClusterID, err := clusters.GetV1ProvisioningClusterByName(client, clusterName)
	if v1ClusterID == "" {
		v3ClusterID, err := clusters.GetClusterIDByName(client, clusterName)
		if err != nil {
			return err
		}
		v1ClusterID = "fleet-default/" + v3ClusterID
	}
	if err != nil {
		return err
	}

	fleetCluster, err := client.Steve.SteveType(stevetypes.FleetCluster).ByID(v1ClusterID)
	if err != nil {
		return err
	}

	provider := fleetCluster.ObjectMeta.Labels["provider.cattle.io"]
	if provider == "rke" {
		sleepNum := (retentionLimit + 1) * timeBetweenSnapshots
		logrus.Infof("Waiting %v hours for %v automatic snapshots to be taken", sleepNum, (retentionLimit + 1))
		time.Sleep(time.Duration(sleepNum)*time.Hour + time.Minute*5)

		err := RKE1RetentionLimitCheck(client, clusterName)
		if err != nil {
			return err
		}

	} else {
		sleepNum := (retentionLimit + 1) * timeBetweenSnapshots
		logrus.Infof("Waiting %v minutes for %v automatic snapshots to be taken", sleepNum, (retentionLimit + 1))
		time.Sleep(time.Duration(sleepNum)*time.Minute + time.Minute*5)

		err := RKE2K3SRetentionLimitCheck(client, clusterName)
		if err != nil {
			return err
		}
	}

	return nil
}

func createPostBackupWorkloads(client *rancher.Client, clusterID string, podTemplate corev1.PodTemplateSpec, deployment *v1.Deployment) (*steveV1.SteveAPIObject, *steveV1.SteveAPIObject, error) {
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
	if err != nil {
		return nil, nil, err
	}

	postDeploymentResp, err := createDeployment(steveclient, workloadNamePostBackup, postBackupDeployment)
	if err != nil {
		return nil, nil, err
	}

	err = deploy.VerifyDeployment(steveclient, postDeploymentResp)
	if err != nil {
		return nil, nil, err
	}

	if workloadNamePostBackup != postDeploymentResp.ObjectMeta.Name {
		return nil, nil, fmt.Errorf("PostBackup deployment name %s does not match created deployment %s ", workloadNamePostBackup, postDeploymentResp.ObjectMeta.Name)
	}

	postServiceResp, err := services.CreateService(steveclient, postBackupService)
	if err != nil {
		return nil, nil, err
	}

	err = services.VerifyService(steveclient, postServiceResp)
	if err != nil {
		return nil, nil, err
	}

	if serviceAppendName+workloadNamePostBackup != postServiceResp.ObjectMeta.Name {
		return nil, nil, fmt.Errorf("PostBackup service name %s does not match created deployment %s ", serviceAppendName+workloadNamePostBackup, postServiceResp.ObjectMeta.Name)
	}

	return postDeploymentResp, postServiceResp, nil
}

func createAndVerifyResources(steveclient *steveV1.Client, containerImage string) (*corev1.PodTemplateSpec, *v1.Deployment, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject, error) {
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

	err = extensionsingress.WaitIngress(steveclient, ingressResp, initialIngressName)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	if initialIngressName != ingressResp.ObjectMeta.Name {
		return nil, nil, nil, nil, nil, errors.New("ingress name doesn't match spec")
	}

	return &podTemplate, deploymentTemplate, deploymentResp, serviceResp, ingressResp, nil
}

func createDeployment(steveclient *steveV1.Client, wlName string, deployment *v1.Deployment) (*steveV1.SteveAPIObject, error) {
	logrus.Infof("Creating deployment: %s", wlName)
	deploymentResp, err := steveclient.SteveType(DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	return deploymentResp, err
}
