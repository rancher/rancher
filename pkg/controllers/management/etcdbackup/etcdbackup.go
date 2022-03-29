package etcdbackup

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	rketypes "github.com/rancher/rke/types"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/rke"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/rkedialerfactory"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/rancher/wrangler/pkg/ticker"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	clusterBackupCheckInterval = 5 * time.Minute
	compressedExtension        = "zip"
	s3Endpoint                 = "s3.amazonaws.com"
)

type Controller struct {
	ctx                   context.Context
	clusterClient         v3.ClusterInterface
	clusterLister         v3.ClusterLister
	backupClient          v3.EtcdBackupInterface
	backupLister          v3.EtcdBackupLister
	backupDriver          *service.EngineService
	KontainerDriverLister v3.KontainerDriverLister
	secretLister          v1.SecretLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := &Controller{
		ctx:                   ctx,
		clusterClient:         management.Management.Clusters(""),
		clusterLister:         management.Management.Clusters("").Controller().Lister(),
		backupClient:          management.Management.EtcdBackups(""),
		backupLister:          management.Management.EtcdBackups("").Controller().Lister(),
		backupDriver:          service.NewEngineService(clusterprovisioner.NewPersistentStore(management.Core.Namespaces(""), management.Core)),
		secretLister:          management.Core.Secrets("").Controller().Lister(),
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
	}

	local := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
		Ctx:     ctx,
	}
	docker := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
		Docker:  true,
		Ctx:     ctx,
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
	if rketypes.BackupConditionCompleted.IsFalse(b) || rketypes.BackupConditionCompleted.IsTrue(b) {
		return b, nil
	}

	cluster, err := c.clusterClient.Get(b.Spec.ClusterID, metav1.GetOptions{})
	if err != nil {
		return b, err
	}

	if !isBackupSet(cluster.Spec.RancherKubernetesEngineConfig) {
		return b, fmt.Errorf("[etcd-backup] cluster doesn't have a backup config")
	}

	if !rketypes.BackupConditionCreated.IsTrue(b) {
		b.Spec.Filename = generateBackupFilename(b.Name, cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig)
		b.Spec.BackupConfig = *cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig
		rketypes.BackupConditionCreated.True(b)
		// we set ConditionCompleted to Unknown to avoid incorrect "active" state
		rketypes.BackupConditionCompleted.Unknown(b)
		b, err = c.backupClient.Update(b)
		if err != nil {
			return b, err
		}
	}
	bObj, saveErr := c.etcdSaveWithBackoff(b)
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
	logrus.Debugf("[etcd-backup] deleting backup %s ", b.Name)
	if err := c.etcdRemoveSnapshotWithBackoff(b); err != nil {
		logrus.Errorf("[etcd-backup] giving up on deleting backup [%s]: %v", b.Name, err)
	}
	return b, nil
}

func (c *Controller) Updated(b *v3.EtcdBackup) (runtime.Object, error) {
	return b, nil
}

func (c *Controller) clusterBackupSync(ctx context.Context, interval time.Duration) error {
	for range ticker.Context(ctx, interval) {
		clusters, err := c.clusterLister.List("", labels.NewSelector())
		if err != nil {
			logrus.Error(fmt.Errorf("[etcd-backup] error while listing clusters: %v", err))
			return err
		}
		for _, cluster := range clusters {
			logrus.Debugf("[etcd-backup] checking backups for cluster [%s]", cluster.Name)
			if err := c.doClusterBackupSync(cluster); err != nil && !apierrors.IsConflict(err) {
				logrus.Error(fmt.Errorf("[etcd-backup] error while syncing cluster backups for cluster [%s]: %v", cluster.Name, err))
			}
		}
	}
	return nil
}

func (c *Controller) doClusterBackupSync(cluster *v3.Cluster) error {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil
	}
	// check if the cluster is eligible for backup.
	if !shouldBackup(cluster) {
		return nil
	}

	backups, err := c.getBackupsList(cluster)
	if err != nil {
		return err
	}

	// cluster has no backups, we need to kick a new one.
	if len(backups) == 0 {
		logrus.Debugf("[etcd-backup] cluster [%s] has no backups, creating first backup", cluster.Name)
		newBackup, err := c.createNewBackup(cluster)
		if err != nil {
			return fmt.Errorf("[etcd-backup] error while creating backup for cluster [%s]: %v", cluster.Name, err)
		}
		logrus.Debugf("[etcd-backup] cluster [%s] new backup is created: %s", cluster.Name, newBackup.Name)
		return nil
	}

	// check for backups in progress
	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		clusterTimeout := time.Duration(cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.Timeout) * time.Second
		for _, clusterBackup := range backups {
			backupCreationTime, err := getBackupCreatedTime(clusterBackup)
			if err != nil {
				logrus.Errorf("[etcd-backup] cluster [%s] backup [%s] is missing creation time: %v", cluster.Name, clusterBackup.Name, err)
				continue
			}
			// cluster backup is younger than its timeout and completion is unknown
			// therefore, it's currently running
			if time.Since(backupCreationTime) < clusterTimeout && !rketypes.BackupConditionCompleted.IsTrue(clusterBackup) {
				logrus.Debugf("[etcd-backup] cluster [%s] is currently creating a backup, skipping", cluster.Name)
				return nil
			}
		}
	}

	recurringBackups := getRecurringBackupsList(backups)
	// cluster has no recurring backups, because the user created one before we could.
	if len(recurringBackups) == 0 {
		logrus.Debugf("[etcd-backup] cluster [%s] has no backups, creating first backup", cluster.Name)
		newBackup, err := c.createNewBackup(cluster)
		if err != nil {
			return fmt.Errorf("[etcd-backup] error while creating backup for cluster [%s]: %v", cluster.Name, err)
		}
		logrus.Debugf("[etcd-backup] cluster [%s] new backup is created: %s", cluster.Name, newBackup.Name)
		return nil
	}

	newestBackup := recurringBackups[0]
	for _, clusterBackup := range recurringBackups[1:] {
		if getBackupCompletedTime(clusterBackup).After(getBackupCompletedTime(newestBackup)) {
			newestBackup = clusterBackup
		}
	}

	// this cluster has backups, lets see if the last one is old enough
	// a new backup is due if this is true
	intervalHours := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.IntervalHours
	backupIntervalHours := time.Duration(intervalHours) * time.Hour

	if time.Since(getBackupCompletedTime(newestBackup)) > backupIntervalHours {
		newBackup, err := c.createNewBackup(cluster)
		if err != nil {
			return fmt.Errorf("[etcd-backup] error while create new backup for cluster [%s]: %v", cluster.Name, err)
		}
		logrus.Debugf("[etcd-backup] new backup created: %s", newBackup.Name)
	}

	// rotate old backups
	return c.rotateExpiredBackups(cluster, recurringBackups)
}

func (c *Controller) createNewBackup(cluster *v3.Cluster) (*v3.EtcdBackup, error) {
	newBackup, err := NewBackupObject(cluster, false)
	if err != nil {
		return nil, err
	}
	rketypes.BackupConditionCreated.CreateUnknownIfNotExists(newBackup)
	return c.backupClient.Create(newBackup)

}

func (c *Controller) etcdSaveWithBackoff(b *v3.EtcdBackup) (runtime.Object, error) {
	backoff := getBackoff()
	kontainerDriver, err := c.KontainerDriverLister.Get("", service.RancherKubernetesEngineDriverName)
	if err != nil {
		return b, err
	}

	snapshotName := clusterprovisioner.GetBackupFilename(b)
	bObj, err := rketypes.BackupConditionCompleted.Do(b, func() (runtime.Object, error) {
		cluster, err := c.clusterClient.Get(b.Spec.ClusterID, metav1.GetOptions{})
		if err != nil {
			return b, err
		}
		cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig = b.Spec.BackupConfig.DeepCopy()
		spec := *cluster.Spec.DeepCopy()
		spec, err = secretmigrator.AssembleS3Credential(cluster.Status.S3CredentialSecret, secretmigrator.ClusterType, cluster.Name, spec, c.secretLister)
		if err != nil {
			return b, err
		}
		var inErr error
		err = wait.ExponentialBackoff(backoff, func() (bool, error) {
			if inErr = c.backupDriver.ETCDSave(c.ctx, cluster.Name, kontainerDriver, spec, snapshotName); inErr != nil {
				logrus.Warnf("%v", inErr)
				return false, nil
			}
			return true, nil
		})

		return b, inErr
	})
	if err != nil {
		rketypes.BackupConditionCompleted.False(bObj)
		rketypes.BackupConditionCompleted.ReasonAndMessageFromError(bObj, err)
		return bObj, err
	}
	return bObj, nil
}

func (c *Controller) etcdRemoveSnapshotWithBackoff(b *v3.EtcdBackup) error {
	backoff := getBackoff()

	kontainerDriver, err := c.KontainerDriverLister.Get("", service.RancherKubernetesEngineDriverName)
	if err != nil {
		return err
	}
	cluster, err := c.clusterClient.Get(b.Spec.ClusterID, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig = b.Spec.BackupConfig.DeepCopy()
	spec := *cluster.Spec.DeepCopy()
	spec, err = secretmigrator.AssembleS3Credential(cluster.Status.S3CredentialSecret, secretmigrator.ClusterType, cluster.Name, spec, c.secretLister)
	if err != nil {
		return err
	}
	snapshotName := clusterprovisioner.GetBackupFilename(b)
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		if inErr := c.backupDriver.ETCDRemoveSnapshot(c.ctx, cluster.Name, kontainerDriver, spec, snapshotName); inErr != nil {
			logrus.Warnf("%v", inErr)
			return false, nil
		}
		return true, nil
	})
}

func (c *Controller) rotateExpiredBackups(cluster *v3.Cluster, clusterBackups []*v3.EtcdBackup) error {
	retention := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.Retention
	intervalHours := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.IntervalHours
	expiredBackups := getExpiredBackups(retention, intervalHours, clusterBackups)
	for _, backup := range expiredBackups {
		if backup.Spec.Manual {
			continue
		}
		if err := c.backupClient.DeleteNamespaced(backup.Namespace, backup.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func NewBackupObject(cluster *v3.Cluster, manual bool) (*v3.EtcdBackup, error) {
	controller := true
	typeFlag := "r"     // recurring is the default
	providerFlag := "l" // local is the default

	if manual {
		typeFlag = "m" // manual backup
	}
	if cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
		providerFlag = "s" // s3 backup
	}
	prefix := fmt.Sprintf("%s-%s%s-", cluster.Name, typeFlag, providerFlag)

	compressedCluster, err := CompressCluster(cluster)
	if err != nil {
		return nil, err
	}

	return &v3.EtcdBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    cluster.Name,
			GenerateName: prefix,
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
		Spec: rketypes.EtcdBackupSpec{
			ClusterID: cluster.Name,
			Manual:    manual,
		},
		Status: rketypes.EtcdBackupStatus{
			KubernetesVersion: cluster.Spec.RancherKubernetesEngineConfig.Version,
			ClusterObject:     compressedCluster,
		},
	}, nil
}

func CompressCluster(cluster *v3.Cluster) (string, error) {
	jsonCluster, err := json.Marshal(cluster)
	if err != nil {
		return "", err
	}

	var gzCluster bytes.Buffer
	gz := gzip.NewWriter(&gzCluster)
	defer gz.Close()

	_, err = gz.Write([]byte(jsonCluster))
	if err != nil {
		return "", err
	}

	if err := gz.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(gzCluster.Bytes()), nil
}

func DecompressCluster(cluster string) (*v3.Cluster, error) {
	clusterGzip, err := base64.StdEncoding.DecodeString(cluster)
	if err != nil {
		return nil, fmt.Errorf("error base64.DecodeString: %v", err)
	}

	buffer := bytes.NewBuffer(clusterGzip)

	var gz io.Reader
	gz, err = gzip.NewReader(buffer)
	if err != nil {
		return nil, err
	}

	var clusterJSON bytes.Buffer
	_, err = io.Copy(&clusterJSON, gz)
	if err != nil {
		return nil, err
	}

	c := v3.Cluster{}
	err = json.Unmarshal(clusterJSON.Bytes(), &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func generateBackupFilename(snapshotName string, backupConfig *rketypes.BackupConfig) string {
	// no backup config
	if backupConfig == nil {
		return ""
	}
	filename := fmt.Sprintf("%s_%s.%s", snapshotName, time.Now().Format(time.RFC3339), compressedExtension)
	if backupConfig.SafeTimestamp {
		filename = strings.ReplaceAll(filename, ":", "-")
	}
	// s3 backup
	if backupConfig != nil &&
		backupConfig.S3BackupConfig != nil {
		if len(backupConfig.S3BackupConfig.Folder) != 0 {
			return fmt.Sprintf("https://%s/%s/%s/%s", backupConfig.S3BackupConfig.Endpoint, backupConfig.S3BackupConfig.BucketName, backupConfig.S3BackupConfig.Folder, filename)
		}
		return fmt.Sprintf("https://%s/%s/%s", backupConfig.S3BackupConfig.Endpoint, backupConfig.S3BackupConfig.BucketName, filename)
	}
	// local backup
	return filename

}

func GetS3Client(sbc *rketypes.S3BackupConfig, timeout int, dialer dialer.Dialer) (*minio.Client, error) {
	if sbc == nil {
		return nil, fmt.Errorf("Can't find S3 backup target configuration")
	}
	var s3Client = &minio.Client{}
	var creds *credentials.Credentials
	var tr http.RoundTripper = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	endpoint := sbc.Endpoint
	// no access credentials, we assume IAM roles
	if sbc.AccessKey == "" ||
		sbc.SecretKey == "" {
		creds = credentials.NewIAM("")
		if sbc.Endpoint == "" {
			endpoint = s3Endpoint
		}
	} else {
		accessKey := sbc.AccessKey
		secretKey := sbc.SecretKey
		creds = credentials.NewStatic(accessKey, secretKey, "", credentials.SignatureDefault)
	}

	bucketLookup := getBucketLookupType(endpoint)
	opt := minio.Options{
		Creds:        creds,
		Region:       sbc.Region,
		Secure:       true,
		BucketLookup: bucketLookup,
		Transport:    tr,
	}
	if sbc.CustomCA != "" {
		opt.Transport = getCustomCATransport(tr, sbc.CustomCA)
	}
	s3Client, err := minio.New(endpoint, &opt)
	if err != nil {
		return nil, err
	}
	return s3Client, nil
}

func getRecurringBackupsList(backups []*v3.EtcdBackup) []*v3.EtcdBackup {
	retList := []*v3.EtcdBackup{}
	for _, backup := range backups {
		if !backup.Spec.Manual {
			retList = append(retList, backup)
		}
	}
	return retList
}

func (c *Controller) getBackupsList(cluster *v3.Cluster) ([]*v3.EtcdBackup, error) {
	backups, err := c.backupLister.List(cluster.Name, labels.NewSelector())
	return backups, err
}

func getBucketLookupType(endpoint string) minio.BucketLookupType {
	if endpoint == "" {
		return minio.BucketLookupAuto
	}
	if strings.Contains(endpoint, "aliyun") {
		return minio.BucketLookupDNS
	}
	return minio.BucketLookupAuto
}

func getBackupCompletedTime(o runtime.Object) time.Time {
	t, _ := time.Parse(time.RFC3339, rketypes.BackupConditionCompleted.GetLastUpdated(o))
	return t
}

func getBackupCreatedTime(o runtime.Object) (time.Time, error) {
	return time.Parse(time.RFC3339, rketypes.BackupConditionCreated.GetLastUpdated(o))
}

func getExpiredBackups(retention, intervalHours int, backups []*v3.EtcdBackup) []*v3.EtcdBackup {
	expiredList := []*v3.EtcdBackup{}
	toKeepDuration := time.Duration(retention*intervalHours) * time.Hour
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
	if !isBackupSet(cluster.Spec.RancherKubernetesEngineConfig) {
		// no backend backup config
		logrus.Debugf("[etcd-backup] no backup config for cluster [%s]", cluster.Name)
		return false
	}
	// we only work with ready clusters
	if !v32.ClusterConditionReady.IsTrue(cluster) {
		return false
	}

	if !isRecurringBackupEnabled(cluster.Spec.RancherKubernetesEngineConfig) {
		logrus.Debugf("[etcd-backup] recurring backup is disabled cluster [%s]", cluster.Name)
		return false
	}
	return true
}

func getBackoff() wait.Backoff {
	return wait.Backoff{
		Duration: 1000 * time.Millisecond,
		Factor:   2,
		Jitter:   0,
		Steps:    5,
	}
}

func isBackupSet(rkeConfig *rketypes.RancherKubernetesEngineConfig) bool {
	return rkeConfig != nil && // rke cluster
		rkeConfig.Services.Etcd.BackupConfig != nil // backupConfig is set
}

func isRecurringBackupEnabled(rkeConfig *rketypes.RancherKubernetesEngineConfig) bool {
	return isBackupSet(rkeConfig) && rkeConfig.Services.Etcd.BackupConfig.Enabled != nil && *rkeConfig.Services.Etcd.BackupConfig.Enabled
}

func getCustomCATransport(tr http.RoundTripper, ca string) http.RoundTripper {
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM([]byte(ca))
	tr.(*http.Transport).TLSClientConfig = &tls.Config{
		RootCAs: certPool,
	}
	return tr
}
