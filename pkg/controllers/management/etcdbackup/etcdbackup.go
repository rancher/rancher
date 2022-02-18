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
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/rke"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/rkedialerfactory"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	rkecluster "github.com/rancher/rke/cluster"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/wrangler/pkg/ticker"
	log "github.com/sirupsen/logrus"
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

	rketypes.BackupConditionCreated.Unknown(b)
	b, err = c.backupClient.Update(b)
	if err != nil {
		return b, err
	}

	log.Infof("[etcd-backup] cluster [%s] backup added to queue: %s", cluster.Name, b.Name)

	backups, err := c.getBackupsList(cluster)
	if err != nil {
		return b, err
	}

	if anyBackupsRunning(cluster, backups) {
		return b, nil
	}

	if next := nextBackup(backups); next == nil || next.Name == b.Name {
		bObj, err := c.createBackupForCluster(b, cluster)
		if err != nil {
			return bObj, fmt.Errorf("[etcd-backup] failed to perform etcd backup: %v", err)
		}
		return bObj, nil
	}

	return b, nil
}

func (c *Controller) Remove(b *v3.EtcdBackup) (runtime.Object, error) {
	if !rketypes.BackupConditionCreated.IsTrue(b) {
		return b, nil
	}
	log.Debugf("[etcd-backup] deleting backup %s ", b.Name)
	if err := c.etcdRemoveSnapshotWithBackoff(b); err != nil {
		log.Errorf("[etcd-backup] unable to delete backup backup [%s]: %v", b.Name, err)
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
			log.Error(fmt.Errorf("[etcd-backup] error while listing clusters: %v", err))
			return err
		}
		for _, cluster := range clusters {
			log.Debugf("[etcd-backup] checking backups for cluster [%s]", cluster.Name)
			if err = c.runWaitingBackups(cluster); err != nil {
				log.Error(fmt.Errorf("[etcd-backup] error running waiting cluster backups for cluster [%s]: %v", cluster.Name, err))
			}
			if err = c.createRecurringBackup(cluster); err != nil {
				log.Error(fmt.Errorf("[etcd-backup] error while syncing cluster backups for for cluster [%s]: %v", cluster.Name, err))
			}
		}
	}
	return nil
}

func (c *Controller) runWaitingBackups(cluster *v3.Cluster) error {
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

	if anyBackupsRunning(cluster, backups) {
		return nil
	}

	var next *v3.EtcdBackup
	if next = nextBackup(backups); next != nil {
		log.Infof("[etcd-backup] cluster [%s] backup starting from queue: %s", cluster.Name, next.Name)
		if _, err := c.createBackupForCluster(next, cluster); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) createRecurringBackup(cluster *v3.Cluster) error {
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
	recurringBackups := getRecurringBackups(backups)

	// cluster has no recurring backups, we need to create initial backup
	if len(recurringBackups) == 0 {
		log.Debugf("[etcd-backup] cluster [%s] has no backups, creating first backup", cluster.Name)
		newBackup, err := c.createNewBackup(cluster)
		if err != nil {
			return fmt.Errorf("error while creating backup for cluster [%s]: %v", cluster.Name, err)
		}
		log.Debugf("[etcd-backup] cluster [%s] new backup is created: %s", cluster.Name, newBackup.Name)
		return nil
	}

	if anyBackupsRunning(cluster, recurringBackups) || anyBackupsQueued(recurringBackups) {
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
			return fmt.Errorf("error while create new backup for cluster [%s]: %v", cluster.Name, err)
		}
		log.Debugf("[etcd-backup] new backup created: %s", newBackup.Name)
	}

	return c.rotateExpiredBackups(cluster, recurringBackups)
}

func (c *Controller) createBackupForCluster(b *v3.EtcdBackup, cluster *v3.Cluster) (runtime.Object, error) {
	var err error
	if b.DeletionTimestamp != nil || rketypes.BackupConditionCreated.IsUnknown(b) {
		b.Spec.Filename = generateBackupFilename(b.Name, cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig)
		cluster.Spec, err = secretmigrator.AssembleS3Credential(cluster, cluster.Spec, c.secretLister)
		if err != nil {
			return b, err
		}
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
		return b, fmt.Errorf("failed to perform etcd backup: %v", saveErr)
	}
	// try to rotate old backups on successful recurring backup, if not clusterBackupSync will take care of it
	if !b.Spec.Manual {
		if backups, err := c.getBackupsList(cluster); err == nil {
			_ = c.rotateExpiredBackups(cluster, getRecurringBackups(backups))
		}
	}
	return b, nil
}

func anyBackupsRunning(cluster *v3.Cluster, backups []*v3.EtcdBackup) bool {
	clusterTimeout := getTimeout(cluster)
	for _, backup := range backups {
		if !rketypes.BackupConditionCreated.IsTrue(backup) {
			continue
		}
		// cluster backup is younger than its timeout and completion is unknown
		// therefore, it's currently running
		if time.Since(getBackupCreatedTime(backup)) < clusterTimeout && rketypes.BackupConditionCompleted.IsUnknown(backup) {
			log.Debugf("[etcd-backup] cluster [%s] is currently creating a backup, skipping", cluster.Name)
			return true
		}
	}
	return false
}

func anyBackupsQueued(backups []*v3.EtcdBackup) bool {
	for _, backup := range backups {
		if rketypes.BackupConditionCreated.IsTrue(backup) &&
			!rketypes.BackupConditionCompleted.IsTrue(backup) &&
			!rketypes.BackupConditionCompleted.IsFalse(backup) {
			return true
		}
	}
	return false
}

func getTimeout(cluster *v3.Cluster) time.Duration {
	if rkeCfg := cluster.Spec.RancherKubernetesEngineConfig; rkeCfg != nil && rkeCfg.Services.Etcd.BackupConfig.Timeout > 0 {
		return time.Duration(rkeCfg.Services.Etcd.BackupConfig.Timeout) * time.Second
	}
	return time.Duration(rkecluster.DefaultEtcdBackupConfigTimeout) * time.Second
}

func nextBackup(backups []*v3.EtcdBackup) *v3.EtcdBackup {
	var next *v3.EtcdBackup

	// check for backups in queue
	for _, backup := range backups {
		if !rketypes.BackupConditionCreated.IsUnknown(backup) {
			continue
		}
		if next == nil || backup.CreationTimestamp.Time.Before(next.CreationTimestamp.Time) {
			next = backup
		}
	}
	return next
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
		var inErr error
		err = wait.ExponentialBackoff(backoff, func() (bool, error) {
			if inErr = c.backupDriver.ETCDSave(c.ctx, cluster.Name, kontainerDriver, cluster.Spec, snapshotName); inErr != nil {
				log.Warnf("%v", inErr)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return b, err
		}
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
	snapshotName := clusterprovisioner.GetBackupFilename(b)
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		if inErr := c.backupDriver.ETCDRemoveSnapshot(c.ctx, cluster.Name, kontainerDriver, cluster.Spec, snapshotName); inErr != nil {
			log.Warnf("%v", inErr)
			return false, nil
		}
		return true, nil
	})
}

// rotateExpiredBackups removes backups that are older than the expiration period, while retaining the desired number of etcd backups.
// This function expects backups to be sorted from newest to oldest. In practice this function should only delete the last backup,
func (c *Controller) rotateExpiredBackups(cluster *v3.Cluster, backups []*v3.EtcdBackup) error {
	retention := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.Retention
	backups = getCompletedBackups(backups)
	if len(backups) <= retention {
		return nil
	}
	for _, backup := range backups[retention:] {
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

// getRecurringBackups returns the list of recurring backups, sorted newest to oldest by time the backup started
func getRecurringBackups(backups []*v3.EtcdBackup) []*v3.EtcdBackup {
	retList := []*v3.EtcdBackup{}
	for _, backup := range backups {
		if !backup.Spec.Manual {
			retList = append(retList, backup)
		}
	}
	sort.Slice(retList, func(i, j int) bool {
		return getBackupCreatedTime(retList[i]).After(getBackupCreatedTime(retList[j]))
	})
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

func getBackupCreatedTime(o runtime.Object) time.Time {
	t, _ := time.Parse(time.RFC3339, rketypes.BackupConditionCreated.GetLastUpdated(o))
	return t
}

// getCompletedBackups returns the list of completed backups
func getCompletedBackups(backups []*v3.EtcdBackup) []*v3.EtcdBackup {
	completedList := []*v3.EtcdBackup{}
	for _, backup := range backups {
		if rketypes.BackupConditionCompleted.IsTrue(backup) {
			completedList = append(completedList, backup)
		}
	}
	return completedList
}

func shouldBackup(cluster *v3.Cluster) bool {
	// not an rke cluster, we do nothing
	if cluster.Spec.RancherKubernetesEngineConfig == nil {
		log.Debugf("[etcd-backup] [%s] is not an rke cluster, skipping..", cluster.Name)
		return false
	}
	if !isBackupSet(cluster.Spec.RancherKubernetesEngineConfig) {
		// no backend backup config
		log.Debugf("[etcd-backup] no backup config for cluster [%s]", cluster.Name)
		return false
	}
	// we only work with ready clusters
	if !v32.ClusterConditionReady.IsTrue(cluster) {
		return false
	}

	if !isRecurringBackupEnabled(cluster.Spec.RancherKubernetesEngineConfig) {
		log.Debugf("[etcd-backup] recurring backup is disabled cluster [%s]", cluster.Name)
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
