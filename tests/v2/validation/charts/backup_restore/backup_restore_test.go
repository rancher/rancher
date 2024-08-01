package charts

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	bv1 "github.com/rancher/backup-restore-operator/pkg/apis/resources.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/sirupsen/logrus"

	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackupTestSuite struct {
	suite.Suite
	client   *rancher.Client
	session  *session.Session
	project  *management.Project
	S3Client *s3.Client
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
}

func (b *BackupTestSuite) TestBackupChart() {
	subSession := b.session.NewSession()
	defer subSession.Cleanup()

	client, err := b.client.WithSession(subSession)
	require.NoError(b.T(), err)

	b.client = client

	broConfig := backupRestoreConfig
	config.LoadConfig(charts.BackupRestoreConfigurationFileKey, broConfig)

	project, err := projects.GetProjectByName(b.client, localCluster, systemProject)
	require.NoError(b.T(), err)

	b.project = project

	logrus.Info("Checking if the backup chart is already installed...")
	initialBackupChart, err := charts.GetChartStatus(b.client, b.project.ClusterID, backupChartNamespace, backupChartName)
	require.NoError(b.T(), err)

	if !initialBackupChart.IsAlreadyInstalled {
		installBroChart(b.client)
	}

	client, err = b.client.ReLogin()
	require.NoError(b.T(), err)

	logrus.Info("Creating two users, projects, and role templates...")
	userList, projList, roleList, err := createRancherResources(client, 2, b.project.ClusterID)
	require.NoError(b.T(), err)

	logrus.Info("Creating a backup of the local cluster...")
	firstBackup, err := createBackup(client, backupName)
	require.NoError(b.T(), err)
	charts.VerifyBackupCompleted(client, backupSteveType, firstBackup)

	backupObj, err := client.Steve.SteveType(backupSteveType).ByID(backupName)
	require.NoError(b.T(), err)
	backupK8Obj := &bv1.Backup{}
	error := v1.ConvertToK8sType(backupObj.JSONResp, backupK8Obj)
	require.NoError(b.T(), error)
	backupFileName := backupK8Obj.Status.Filename

	logrus.Info("Validating backup file is in AWS S3...")
	backupPresent, err := validateAWSS3BackupExists(broConfig.S3BucketName, broConfig.S3FolderName, backupFileName)
	require.NoError(b.T(), err)
	assert.True(b.T(), backupPresent)

	userListPostBackup, projListPostBackup, roleListPostBackup, err := createRancherResources(client, 2, b.project.ClusterID)
	require.NoError(b.T(), err)

	logrus.Infof("Creating a restore using backup file: %v", backupFileName)
	completedRestore, err := createRestore(client, backupFileName)
	require.NoError(b.T(), err)
	restoreObj, err := client.Steve.SteveType(restoreSteveType).ByID(completedRestore.ID)
	require.NoError(b.T(), err)
	charts.VerifyRestoreCompleted(client, restoreSteveType, restoreObj)

	logrus.Info("Restore complete, validating Rancher resources...")
	err = verifyRancherResources(client, userList, projList, roleList)
	require.NoError(b.T(), err)

	err = verifyRancherResources(client, userListPostBackup, projListPostBackup, roleListPostBackup)
	assert.Error(b.T(), err)

	logrus.Info("Validating downstream clusters are in an Active status...")
	rke1ClusterID, err := clusters.GetClusterIDByName(client, broConfig.Rke1ClusterName)
	require.NoError(b.T(), err)
	rke2ClusterID, err := clusters.GetClusterIDByName(client, broConfig.Rke2ClusterName)
	require.NoError(b.T(), err)

	// Note: The function name mentions upgrade, but it's waiting for the cluster to become active
	// and it logs the cluster status while waiting, therefore it's still useful for post-restore checks
	err = clusters.WaitClusterUntilUpgrade(client, rke1ClusterID)
	require.NoError(b.T(), err)
	err = clusters.WaitClusterUntilUpgrade(client, rke2ClusterID)
	require.NoError(b.T(), err)

	logrus.Info("Validations complete, deleting backup from S3 bucket...")
	err = deleteAWSBackup(broConfig.S3BucketName, broConfig.S3FolderName, backupFileName)
	require.NoError(b.T(), err)
	backupDeleted, err := validateAWSS3BackupExists(broConfig.S3BucketName, broConfig.S3FolderName, backupFileName)
	assert.ErrorContains(b.T(), err, "404")
	assert.False(b.T(), backupDeleted)
}

func TestBackupTestSuite(t *testing.T) {
	suite.Run(t, new(BackupTestSuite))
}
