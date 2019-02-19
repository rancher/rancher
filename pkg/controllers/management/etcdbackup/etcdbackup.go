package etcdbackup

import (
	"context"
	"fmt"
	"time"

	minio "github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"github.com/rancher/kontainer-engine/drivers/rke"
	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/rkedialerfactory"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	clusterBackupCheckInterval = 5 * time.Minute
)

type Controller struct {
	ctx                   context.Context
	clusterClient         v3.ClusterInterface
	clusterLister         v3.ClusterLister
	backupClient          v3.EtcdBackupInterface
	backupLister          v3.EtcdBackupLister
	backupDriver          service.EngineService
	secretsClient         v1.SecretInterface
	KontainerDriverLister v3.KontainerDriverLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := &Controller{
		ctx:                   ctx,
		clusterClient:         management.Management.Clusters(""),
		clusterLister:         management.Management.Clusters("").Controller().Lister(),
		backupClient:          management.Management.EtcdBackups(""),
		backupLister:          management.Management.EtcdBackups("").Controller().Lister(),
		backupDriver:          service.NewEngineService(clusterprovisioner.NewPersistentStore(management.Core.Namespaces(""), management.Core)),
		secretsClient:         management.Core.Secrets(""),
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
	}

	local := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
	}
	docker := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
		Docker:  true,
	}
	driver := service.Drivers[service.RancherKubernetesEngineDriverName]
	rkeDriver := driver.(*rke.Driver)
	rkeDriver.DockerDialer = docker.Build
	rkeDriver.LocalDialer = local.Build
	rkeDriver.WrapTransportFactory = docker.WrapTransport

	c.backupClient.AddLifecycle(ctx, "etcdbackup-controller", c)
	go c.clusterBackupSync(ctx, clusterBackupCheckInterval)
}

func (c *Controller) Create(b *v3.EtcdBackup) (runtime.Object, error) {
	cluster, err := c.clusterClient.Get(b.Spec.ClusterID, metav1.GetOptions{})
	if err != nil {
		return b, err
	}
	if cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig == nil {
		return b, fmt.Errorf("[etcd-backup] cluster doesn't have a backup config")
	}

	if !v3.BackupConditionCreated.IsTrue(b) {
		b.Spec.Filename = getBackupFilename(b.Name, cluster)
		b.Spec.BackupConfig = *cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig
		v3.BackupConditionCreated.True(b)
		b, err = c.backupClient.Update(b)
		if err != nil {
			return b, err
		}
	}

	kontainerDriver, err := c.KontainerDriverLister.Get("", service.RancherKubernetesEngineDriverName)
	if err != nil {
		return b, err
	}
	bObj, saveErr := v3.BackupConditionCompleted.Do(b, func() (runtime.Object, error) {
		err = c.backupDriver.ETCDSave(context.Background(), cluster.Name, kontainerDriver, cluster.Spec, b.Name)
		return b, err
	})
	b, err = c.backupClient.Update(bObj.(*v3.EtcdBackup))
	if err != nil {
		return b, err
	}
	if saveErr != nil {
		return b, fmt.Errorf("[etcd-backup] failed to perform etcd backup: %v", saveErr)
	}
	return b, nil
}

func (c *Controller) Remove(b *v3.EtcdBackup) (runtime.Object, error) {
	logrus.Infof("[etcd-backup] Deleteing backup %s ", b.Name)
	if b.Spec.BackupConfig.S3BackupConfig == nil {
		return b, nil
	}
	return b, c.deleteS3Snapshot(b)
}

func (c *Controller) Updated(b *v3.EtcdBackup) (runtime.Object, error) {
	return b, nil
}

func (c *Controller) clusterBackupSync(ctx context.Context, interval time.Duration) error {
	for range ticker.Context(ctx, interval) {
		clusters, err := c.clusterLister.List("", labels.NewSelector())
		if err != nil {
			logrus.Error(fmt.Errorf("[etcd-backup] clusterBackupSync faild: %v", err))
			return err
		}
		for _, cluster := range clusters {
			logrus.Debugf("[etcd-backup] Checking backups for cluster: %s", cluster.Name)
			if err := c.doClusterBackupSync(cluster); err != nil && !apierrors.IsConflict(err) {
				logrus.Error(fmt.Errorf("[etcd-backup] clusterBackupSync faild: %v", err))
			}
		}
	}
	return nil
}

func (c *Controller) doClusterBackupSync(cluster *v3.Cluster) error {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil
	}
	// check if the cluster is eligable for backup.
	if !shouldBackup(cluster) {
		return nil
	}

	clusterBackups, err := c.backupLister.List(cluster.Name, labels.NewSelector())
	if err != nil {
		return err
	}

	// cluster has no backups, we need to kick a new one.
	if len(clusterBackups) == 0 {
		logrus.Infof("[etcd-backup] Cluster [%s] has no backups, creating first backup", cluster.Name)
		newBackup, err := c.createNewBackup(cluster)
		if err != nil {
			return fmt.Errorf("[etcd-backup] Backup create failed:%v", err)
		}
		logrus.Infof("[etcd-backup] Cluster [%s] new backup is created: %s", cluster.Name, newBackup.Name)
		return nil
	}

	newestBackup := clusterBackups[0]
	for _, clusterBackup := range clusterBackups[1:] {
		if getBackupCompletedTime(clusterBackup).After(getBackupCompletedTime(newestBackup)) {
			newestBackup = clusterBackup
		}
	}

	// this cluster has backups, lets see if the last one is old enough
	// a new backup is due if this is true
	internvalHours := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.IntervalHours
	backupIntervalHours := time.Duration(internvalHours) * time.Hour

	if time.Since(getBackupCompletedTime(newestBackup)) > backupIntervalHours {
		newBackup, err := c.createNewBackup(cluster)
		if err != nil {
			return fmt.Errorf("[etcd-backup] Backup create failed:%v", err)
		}
		logrus.Infof("[etcd-backup] New backup created: %s", newBackup.Name)
	}

	// rotate old backups
	return c.rotateExpiredBackups(cluster, clusterBackups)
}

func (c *Controller) createNewBackup(cluster *v3.Cluster) (*v3.EtcdBackup, error) {
	newBackup := NewBackupObject(cluster)
	v3.BackupConditionCreated.CreateUnknownIfNotExists(newBackup)
	return c.backupClient.Create(newBackup)

}

func (c *Controller) rotateExpiredBackups(cluster *v3.Cluster, clusterBackups []*v3.EtcdBackup) error {
	retention := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.Retention
	internvalHours := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.IntervalHours
	expiredBackups := getExpiredBackups(retention, internvalHours, clusterBackups)
	for _, backup := range expiredBackups {
		if err := c.backupClient.DeleteNamespaced(backup.Namespace, backup.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func NewBackupObject(cluster *v3.Cluster) *v3.EtcdBackup {
	controller := true
	return &v3.EtcdBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    cluster.Name,
			GenerateName: fmt.Sprintf("%s-", cluster.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					UID:        cluster.UID,
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Controller: &controller,
				},
			},
		},
		Spec: v3.EtcdBackupSpec{
			ClusterID: cluster.Name,
		},
	}
}

func getBackupFilename(snapshotName string, cluster *v3.Cluster) string {
	if cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig == nil ||
		cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig == nil {
		return ""
	}
	target := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", target.Region, target.BucketName, snapshotName)
}

func (c *Controller) deleteS3Snapshot(b *v3.EtcdBackup) error {
	if b.Spec.BackupConfig.S3BackupConfig == nil {
		return fmt.Errorf("can't find S3 backup target configuration")
	}
	var err error
	s3Client := &minio.Client{}
	endpoint := b.Spec.BackupConfig.S3BackupConfig.Endpoint
	bucket := b.Spec.BackupConfig.S3BackupConfig.BucketName
	// no access credentials, we assume IAM roles
	if b.Spec.BackupConfig.S3BackupConfig.AccessKey == "" ||
		b.Spec.BackupConfig.S3BackupConfig.SecretKey == "" {
		creds := credentials.NewIAM("")
		s3Client, err = minio.NewWithCredentials(endpoint, creds, true, "")
	} else {
		accessKey := b.Spec.BackupConfig.S3BackupConfig.AccessKey
		secretKey := b.Spec.BackupConfig.S3BackupConfig.SecretKey

		s3Client, err = minio.New(endpoint, accessKey, secretKey, true)
	}
	if err != nil {
		return err
	}

	exists, err := s3Client.BucketExists(bucket)
	if err != nil {
		return fmt.Errorf("can't access bucket: %v", err)
	}
	if !exists {
		logrus.Errorf("bucket %s doesn't exist", bucket)
		return nil
	}

	return s3Client.RemoveObject(bucket, b.Name)
}

func getBackupCompletedTime(o runtime.Object) time.Time {
	t, _ := time.Parse(time.RFC3339, v3.BackupConditionCompleted.GetLastUpdated(o))
	return t
}

func getExpiredBackups(retention, internvalHours int, backups []*v3.EtcdBackup) []*v3.EtcdBackup {
	expiredList := []*v3.EtcdBackup{}
	toKeepDuration := time.Duration(retention*internvalHours) * time.Hour
	for _, backup := range backups {
		if time.Since(getBackupCompletedTime(backup)) > toKeepDuration {
			expiredList = append(expiredList, backup)
		}
	}
	return expiredList
}

func shouldBackup(cluster *v3.Cluster) bool {
	// not an rke cluster, we do nothing
	if cluster.Spec.RancherKubernetesEngineConfig == nil {
		logrus.Debugf("[etcd-backup] [%s] is not an rke cluster, skipping..", cluster.Name)
		return false
	}
	// we only work with ready clusters
	if !v3.ClusterConditionReady.IsTrue(cluster) {
		return false
	}
	// check the backup config
	etcdService := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd
	if etcdService.BackupConfig == nil {
		// no backend backup config
		logrus.Debugf("[etcd-backup] No backup config for cluster [%s]", cluster.Name)
		return false
	}
	return true
}
