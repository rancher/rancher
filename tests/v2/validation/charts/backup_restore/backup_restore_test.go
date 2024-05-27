package charts

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	bv1 "github.com/rancher/backup-restore-operator/pkg/apis/resources.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/sirupsen/logrus"

	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackupTestSuite struct {
	suite.Suite
	client              *rancher.Client
	session             *session.Session
	project             *management.Project
	chartInstallOptions *charts.InstallOptions
	chartFeatureOptions *charts.RancherBackupOpts
	S3Client            *s3.Client
}

func (b *BackupTestSuite) TearDownSuite() {
	b.session.Cleanup()
}

func (b *BackupTestSuite) SetupSuite() {
	testSession := session.NewSession()
	b.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(b.T(), err)

	b.client = client

	broConfig := new(charts.Config)
	config.LoadConfig(charts.ConfigurationFileKey, broConfig)

	project, err := projects.GetProjectByName(client, cluster, clusterProject)
	require.NoError(b.T(), err)

	// Get clusterName from config yaml
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(b.T(), clusterName, "Cluster name to install is not set")

	// Get cluster meta
	cluster, err := clusters.NewClusterMeta(client, clusterName)
	require.NoError(b.T(), err)

	// Get latest version of Rancher Backup chart
	latestBackupVersion, err := client.Catalog.GetLatestChartVersion(backupChartName, catalog.RancherChartRepo)
	require.NoError(b.T(), err)

	b.chartInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestBackupVersion,
		ProjectID: project.ID,
	}
	b.chartFeatureOptions = &charts.RancherBackupOpts{
		VolumeName:                broConfig.VolumeName,
		BucketName:                broConfig.S3BucketName,
		CredentialSecretName:      secretName,
		CredentialSecretNamespace: broConfig.CredentialSecretNamespace,
		Enabled:                   backupEnabled,
		Endpoint:                  broConfig.S3Endpoint,
		Folder:                    broConfig.S3FolderName,
		Region:                    broConfig.S3Region,
	}

	subSession := b.session.NewSession()
	defer subSession.Cleanup()
}

func (b *BackupTestSuite) TestBackupChart() {
	subSession := b.session.NewSession()
	defer subSession.Cleanup()

	client, err := b.client.WithSession(subSession)
	require.NoError(b.T(), err)

	b.client = client

	broConfig := new(charts.Config)
	config.LoadConfig(charts.ConfigurationFileKey, broConfig)

	project, err := projects.GetProjectByName(b.client, cluster, clusterProject)
	require.NoError(b.T(), err)

	b.project = project

	logrus.Info("Checking if the backup chart is already installed...")
	initialBackupChart, err := charts.GetChartStatus(b.client, b.project.ClusterID, backupChartNamespace, backupChartName)
	require.NoError(b.T(), err)

	if !initialBackupChart.IsAlreadyInstalled {
		installBroChart(b.T(), b.client, b.chartInstallOptions, b.chartFeatureOptions, b.project)
	}

	client, err = b.client.ReLogin()
	require.NoError(b.T(), err)

	logrus.Info("Creating two users...")
	u1, err := users.CreateUserWithRole(client, users.UserConfig(), member)
	require.NoError(b.T(), err)
	userList = append(userList, u1)

	u2, err := users.CreateUserWithRole(client, users.UserConfig(), member)
	require.NoError(b.T(), err)
	userList = append(userList, u2)

	logrus.Info("Creating two projects...")
	p1, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)
	projList = append(projList, p1)

	p2, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)
	projList = append(projList, p2)

	logrus.Info("Creating two role templates...")
	rt1, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)
	roleList = append(roleList, rt1)

	rt2, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)
	roleList = append(roleList, rt2)

	logrus.Info("Creating a backup of the local cluster...")
	firstBackup, err := createBackup(client, backupName)
	require.NoError(b.T(), err)
	charts.VerifyBackupCompleted(b.T(), client, backupSteveType, firstBackup)

	backupObj, err := client.Steve.SteveType(backupSteveType).ByID(backupName)
	require.NoError(b.T(), err)
	backupK8Obj := &bv1.Backup{}
	error := v1.ConvertToK8sType(backupObj.JSONResp, backupK8Obj)
	require.NoError(b.T(), error)
	backupFileName := backupK8Obj.Status.Filename

	logrus.Info("Validating backup file is in AWS S3...")
	backupPresent, err := validateAWSS3BackupExists(b.T(), broConfig.S3BucketName, broConfig.S3FolderName, backupFileName)
	require.NoError(b.T(), err)
	assert.True(b.T(), backupPresent)

	logrus.Info("Creating two more users...")
	u3, err := users.CreateUserWithRole(client, users.UserConfig(), member)
	require.NoError(b.T(), err)
	userList = append(userList, u3)

	u4, err := users.CreateUserWithRole(client, users.UserConfig(), member)
	require.NoError(b.T(), err)
	userList = append(userList, u4)

	logrus.Info("Creating two more projects...")
	p3, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)
	projList = append(projList, p3)

	p4, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)
	projList = append(projList, p4)

	logrus.Info("Creating two more role templates...")
	rt3, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)
	roleList = append(roleList, rt3)

	rt4, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)
	roleList = append(roleList, rt4)

	logrus.Infof("Creating a restore using backup file: %v", backupFileName)
	completedRestore, err := createRestore(client, backupFileName)
	require.NoError(b.T(), err)
	charts.VerifyRestoreCompleted(b.T(), client, restoreSteveType, completedRestore)

	logrus.Info("Restore complete, validating Rancher resources...")
	verifyUserResources(b.T(), client, userList)
	verifyProjectResources(b.T(), client, projList)
	verifyRoleResources(b.T(), client, roleList)

	logrus.Info("Validations complete, deleting backup from S3 bucket...")
	deleteAWSBackup(b.T(), broConfig.S3BucketName, broConfig.S3FolderName, backupFileName)
	backupDeleted, err := validateAWSS3BackupExists(b.T(), broConfig.S3BucketName, broConfig.S3FolderName, backupFileName)
	assert.ErrorContains(b.T(), err, "404")
	assert.False(b.T(), backupDeleted)
}

func TestBackupTestSuite(t *testing.T) {
	suite.Run(t, new(BackupTestSuite))
}
