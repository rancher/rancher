package charts

import (
	"testing"

	bv1 "github.com/rancher/backup-restore-operator/pkg/apis/resources.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/sirupsen/logrus"

	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/extensions/provisioning"
	"github.com/rancher/shepherd/extensions/users"
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
	config := setBroUserConfig()

	project, err := projects.GetProjectByName(client, cluster, clusterProject)
	require.NoError(b.T(), err)

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(b.T(), clusterName, "Cluster name to install is not set")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(b.T(), err)

	latestBackupVersion, err := client.Catalog.GetLatestChartVersion(backupChartName, catalog.RancherChartRepo)
	require.NoError(b.T(), err)

	b.chartInstallOptions = &charts.InstallOptions{
		ClusterName: clusterName,
		ClusterID:   clusterID,
		Version:     latestBackupVersion,
		ProjectID:   project.ID,
	}
	b.chartFeatureOptions = &charts.RancherBackupOpts{
		VolumeName:                config["volumeName"].(string),
		BucketName:                config["bucket"].(string),
		CredentialSecretName:      secretName,
		CredentialSecretNamespace: config["secretNamespace"].(string),
		Enabled:                   backupEnabled,
		Endpoint:                  config["endpoint"].(string),
		Folder:                    config["folder"].(string),
		Region:                    config["region"].(string),
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

	project, err := projects.GetProjectByName(b.client, cluster, clusterProject)
	require.NoError(b.T(), err)

	b.project = project
	config := setBroUserConfig()

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
	u2, err := users.CreateUserWithRole(client, users.UserConfig(), member)
	require.NoError(b.T(), err)

	logrus.Info("Creating two projects...")
	p1, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)
	p2, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)

	logrus.Info("Creating two role templates...")
	rt1, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)
	rt2, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)

	logrus.Info("Deploying downstream RKE1 cluster...")
	clusterObject, _, rke1ClusterConfig, err := createDownstreamCluster(client, "RKE1")
	require.NoError(b.T(), err)
	logrus.Info("Verifying RKE1 cluster...")
	provisioning.VerifyRKE1Cluster(b.T(), client, rke1ClusterConfig, clusterObject)

	logrus.Info("Deploying downstream RKE2 cluster...")
	_, steveObject, rke2ClusterConfig, err := createDownstreamCluster(client, "RKE2")
	require.NoError(b.T(), err)
	logrus.Info("Verifying RKE2 cluster...")
	provisioning.VerifyCluster(b.T(), client, rke2ClusterConfig, steveObject)

	logrus.Info("Creating a backup of the local cluster...")
	firstBackup, err := createBackup(client, backupName)
	require.NoError(b.T(), err)
	charts.VerifyBackupCompleted(b.T(), client, backupSteveType, firstBackup)

	backupObj, err := client.Steve.SteveType(backupSteveType).ByID(backupName)
	require.NoError(b.T(), err)
	backupK8Obj := &bv1.Backup{}
	error := v1.ConvertToK8sType(backupObj.JSONResp, backupK8Obj)
	require.NoError(b.T(), error)
	firstBackupFileName := backupK8Obj.Status.Filename

	logrus.Info("Validating backup file is in AWS S3...")
	backupPresent, err := validateAWSS3BackupExists(b.T(), config["bucket"].(string), config["folder"].(string), firstBackupFileName)
	require.NoError(b.T(), err)
	assert.True(b.T(), backupPresent)

	logrus.Info("Creating two more users...")
	u3, err := users.CreateUserWithRole(client, users.UserConfig(), member)
	require.NoError(b.T(), err)
	u4, err := users.CreateUserWithRole(client, users.UserConfig(), member)
	require.NoError(b.T(), err)

	logrus.Info("Creating two more projects...")
	p3, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)
	p4, err := createProject(client, b.project.ClusterID)
	require.NoError(b.T(), err)

	logrus.Info("Creating two more role templates...")
	rt3, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)
	rt4, err := createRole(client, roleContext, roleName, rules)
	require.NoError(b.T(), err)

	logrus.Info("Creating a new backup of the local cluster...")
	secondBackup, err := createBackup(client, secondBackupName)
	require.NoError(b.T(), err)
	charts.VerifyBackupCompleted(b.T(), client, backupSteveType, secondBackup)

	backupObj, err = client.Steve.SteveType(backupSteveType).ByID(secondBackupName)
	require.NoError(b.T(), err)
	error = v1.ConvertToK8sType(backupObj.JSONResp, backupK8Obj)
	require.NoError(b.T(), error)
	secondBackupFileName := backupK8Obj.Status.Filename

	logrus.Infof("Creating a restore using backup file: %v", firstBackupFileName)
	completedRestore, err := createRestore(client, firstBackupFileName)
	require.NoError(b.T(), err)
	charts.VerifyRestoreCompleted(b.T(), client, restoreSteveType, completedRestore)

	logrus.Info("Restore complete, validating Rancher resources...")
	verifyUserResourcesExist(b.T(), client, u1)
	verifyUserResourcesExist(b.T(), client, u2)
	verifyUserResourcesNotExist(b.T(), client, u3)
	verifyUserResourcesNotExist(b.T(), client, u4)

	verifyProjectResourceExist(b.T(), client, p1)
	verifyProjectResourceExist(b.T(), client, p2)
	verifyProjectResourceNotExist(b.T(), client, p3)
	verifyProjectResourceNotExist(b.T(), client, p4)

	verifyRoleResourcesExist(b.T(), client, rt1)
	verifyRoleResourcesExist(b.T(), client, rt2)
	verifyRoleResourcesNotExist(b.T(), client, rt3)
	verifyRoleResourcesNotExist(b.T(), client, rt4)

	logrus.Info("Verifying downstream clusters post-restore...")
	provisioning.VerifyRKE1Cluster(b.T(), client, rke1ClusterConfig, clusterObject)
	provisioning.VerifyCluster(b.T(), client, rke2ClusterConfig, steveObject)

	logrus.Info("Validations complete, deleting first backup from S3 bucket...")
	deleteAWSBackup(b.T(), config["bucket"].(string), config["folder"].(string), firstBackupFileName)
	firstBackupDeleted, err := validateAWSS3BackupExists(b.T(), config["bucket"].(string), config["folder"].(string), firstBackupFileName)
	assert.ErrorContains(b.T(), err, "404")
	assert.False(b.T(), firstBackupDeleted)

	logrus.Info("Deleting second backup from S3 bucket...")
	deleteAWSBackup(b.T(), config["bucket"].(string), config["folder"].(string), secondBackupFileName)
	secondBackupDeleted, err := validateAWSS3BackupExists(b.T(), config["bucket"].(string), config["folder"].(string), firstBackupFileName)
	assert.ErrorContains(b.T(), err, "404")
	assert.False(b.T(), secondBackupDeleted)
}

func TestBackupTestSuite(t *testing.T) {
	suite.Run(t, new(BackupTestSuite))
}
