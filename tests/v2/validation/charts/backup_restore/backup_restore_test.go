//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package backup_restore

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	bv1 "github.com/rancher/backup-restore-operator/pkg/apis/resources.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	shepCharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackupTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	S3Client  *s3.Client
	broConfig *charts.BackupRestoreConfig
}

func (b *BackupTestSuite) TearDownSuite() {
	b.session.Cleanup()
}

func (b *BackupTestSuite) SetupSuite() {
	b.session = session.NewSession()

	client, err := rancher.NewClient("", b.session)
	require.NoError(b.T(), err)

	b.client = client

	subSession := b.session.NewSession()
	defer subSession.Cleanup()

	b.broConfig = backupRestoreConfig
}

func (b *BackupTestSuite) TestS3InPlaceRestore() {
	subSession := b.session.NewSession()
	defer subSession.Cleanup()

	project, err := projects.GetProjectByName(b.client, cluster, "System")
	require.NoError(b.T(), err)

	config.LoadConfig(charts.BackupRestoreConfigurationFileKey, backupRestoreConfig)

	logrus.Info("Checking if the backup chart is already installed...")
	initialBackupChart, err := shepCharts.GetChartStatus(b.client, project.ClusterID, "cattle-resources-system", "rancher-backup")
	require.NoError(b.T(), err)

	if !initialBackupChart.IsAlreadyInstalled {
		installBroChart(b.client)
	}

	b.client, err = b.client.ReLogin()
	require.NoError(b.T(), err)

	logrus.Info("Creating two users, projects, and role templates...")
	userList, projList, roleList, err := createRancherResources(b.client, project.ClusterID, "cluster")
	require.NoError(b.T(), err)

	logrus.Info("Provisioning a downstream RKE1 cluster...")
	rke1ClusterObj, rke1ClusterConfig, err := createRKE1dsCluster(b.T(), b.client)
	require.NoError(b.T(), err)

	logrus.Info("Provisioning a downstream RKE2 cluster...")
	rke2SteveObj, rke2ClusterConfig, err := createRKE2dsCluster(b.T(), b.client)
	require.NoError(b.T(), err)

	logrus.Info("Creating a backup of the local cluster...")
	s3StorageLocation := backupRestoreConfig.S3BucketName + "/" + backupRestoreConfig.S3FolderName
	_, backupFileName, err := createAndValidateBackup(b.client, s3StorageLocation, b.broConfig)
	require.NoError(b.T(), err)

	logrus.Info("Validating backup file is in AWS S3...")
	backupPresent, err := checkAWSS3Object(s3StorageLocation, backupFileName)
	require.NoError(b.T(), err)
	assert.True(b.T(), backupPresent)

	logrus.Info("Creating two more users, projects, and role templates...")
	userListPostBackup, projListPostBackup, roleListPostBackup, err := createRancherResources(b.client, project.ClusterID, "cluster")
	require.NoError(b.T(), err)

	logrus.Infof("Creating a restore using backup file: %v", backupFileName)
	restoreTemplate := bv1.NewRestore("", "", setRestoreObject(b.broConfig))
	restoreTemplate.Spec.BackupFilename = backupFileName
	createdRestore, err := b.client.Steve.SteveType(restoreSteveType).Create(restoreTemplate)
	require.NoError(b.T(), err)

	restoreObj, err := b.client.Steve.SteveType(restoreSteveType).ByID(createdRestore.ID)
	require.NoError(b.T(), err)

	charts.VerifyRestoreCompleted(b.client, restoreSteveType, restoreObj)

	logrus.Info("Validating Rancher resources...")
	err = verifyRancherResources(b.client, userList, projList, roleList)
	require.NoError(b.T(), err)

	err = verifyRancherResources(b.client, userListPostBackup, projListPostBackup, roleListPostBackup)
	assert.Error(b.T(), err)

	logrus.Info("Validating downstream clusters are in an Active status...")
	provisioning.VerifyRKE1Cluster(b.T(), b.client, rke1ClusterConfig, rke1ClusterObj)
	provisioning.VerifyCluster(b.T(), b.client, rke2ClusterConfig, rke2SteveObj)
}

func TestBackupTestSuite(t *testing.T) {
	suite.Run(t, new(BackupTestSuite))
}
