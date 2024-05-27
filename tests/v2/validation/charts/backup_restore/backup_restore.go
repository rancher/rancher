package charts

import (
	"context"
	"fmt"
	"testing"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/secrets"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bv1 "github.com/rancher/backup-restore-operator/pkg/apis/resources.cattle.io/v1"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	backupName = namegen.AppendRandomString("backup")
	secretName = namegen.AppendRandomString("broSecretName")
	roleName   = namegen.AppendRandomString("broRole")
	userList   = []*management.User{}
	projList   = []*management.Project{}
	roleList   = []*management.RoleTemplate{}

	rules = []management.PolicyRule{
		{
			APIGroups: []string{"management.cattle.io"},
			Resources: []string{"projects"},
			Verbs:     []string{psaRole},
		},
	}
)

const (
	backupEnabled        = true
	backupChartNamespace = "cattle-resources-system"
	backupChartName      = "rancher-backup"
	backupSteveType      = "resources.cattle.io.backup"
	restoreSteveType     = "resources.cattle.io.restore"
	psaRole              = "updatepsa"
	roleContext          = "cluster"
	clusterProject       = "System"
	cluster              = "local"
	member               = "user"
)

func setBackupObject() bv1.Backup {
	broConfig := new(charts.Config)
	config.LoadConfig(charts.ConfigurationFileKey, broConfig)

	backup := bv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: backupName,
		},
		Spec: bv1.BackupSpec{
			ResourceSetName: broConfig.ResourceSetName,
		},
	}
	return backup
}

func setRestoreObject() bv1.Restore {
	broConfig := new(charts.Config)
	config.LoadConfig(charts.ConfigurationFileKey, broConfig)

	restore := bv1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "restore-",
		},
		Spec: bv1.RestoreSpec{
			BackupFilename: backupName,
			Prune:          &broConfig.Prune,
		},
	}
	return restore
}

func installBroChart(t *testing.T, client *rancher.Client, chartInstallOptions *charts.InstallOptions, chartFeatureOptions *charts.RancherBackupOpts, project *management.Project) {
	_, err := createOpaqueS3Secret(client.Steve)
	require.NoError(t, err)

	logrus.Info("Installing backup chart")
	err = charts.InstallRancherBackupChart(client, chartInstallOptions, chartFeatureOptions, true)
	require.NoError(t, err)

	logrus.Info("Waiting for backup chart deployments to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	require.NoError(t, err)

	logrus.Info("Waiting for backup chart DaemonSets to have expected number of available nodes")
	err = charts.WatchAndWaitDaemonSets(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	require.NoError(t, err)

	logrus.Info("Waiting for backup chart StatefulSets to have expected number of ready replicas")
	err = charts.WatchAndWaitStatefulSets(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	require.NoError(t, err)
}

func createOpaqueS3Secret(steveClient *v1.Client) (string, error) {
	broConfig := new(charts.Config)
	config.LoadConfig(charts.ConfigurationFileKey, broConfig)

	logrus.Infof("Creating an opaque secret with name: %v", secretName)
	secretTemplate := secrets.NewSecretTemplate(secretName, broConfig.CredentialSecretNamespace, map[string][]byte{"accessKey": []byte(broConfig.AccessKey), "secretKey": []byte(broConfig.SecretKey)}, corev1.SecretTypeOpaque)
	createdSecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)

	return createdSecret.Name, err
}

func createProject(client *rancher.Client, clusterID string) (*management.Project, error) {
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      clusterProject,
	}
	createProject, err := client.Management.Project.Create(projectConfig)
	if err != nil {
		return nil, err
	}

	return createProject, nil
}

func createRole(client *rancher.Client, context string, roleName string, rules []management.PolicyRule) (role *management.RoleTemplate, err error) {
	role, err = client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context: context,
			Name:    roleName,
			Rules:   rules,
		})

	return
}

func createBackup(client *rancher.Client, name string) (*v1.SteveAPIObject, error) {
	backup := setBackupObject()
	backupTemplate := bv1.NewBackup("", name, backup)
	completedBackup, err := client.Steve.SteveType(backupSteveType).Create(backupTemplate)

	return completedBackup, err
}

func createRestore(client *rancher.Client, fileName string) (*v1.SteveAPIObject, error) {
	restore := setRestoreObject()
	restoreTemplate := bv1.NewRestore("", "", restore)
	restoreTemplate.Spec.BackupFilename = fileName
	completedRestore, err := client.Steve.SteveType(restoreSteveType).Create(restoreTemplate)

	return completedRestore, err
}

func validateAWSS3BackupExists(t *testing.T, bucket string, folder string, key string) (bool, error) {
	if len(folder) > 0 {
		folder += key
		isPresent, err := checkAWSS3Object(t, bucket, folder)
		return isPresent, err
	}
	isPresent, err := checkAWSS3Object(t, bucket, key)
	return isPresent, err
}

func checkAWSS3Object(t *testing.T, bucket string, key string) (bool, error) {
	sdkConfig, err := awsConfig.LoadDefaultConfig(context.TODO())
	require.NoError(t, err)

	s3Client := s3.NewFromConfig(sdkConfig)
	_, err = s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				return false, nil
			default:
				return false, err
			}
		}
		return false, err
	}
	return true, nil
}

func deleteAWSBackup(t *testing.T, bucket string, folder string, key string) {
	sdkConfig, err := awsConfig.LoadDefaultConfig(context.TODO())
	require.NoError(t, err)
	svc := s3.NewFromConfig(sdkConfig)

	input := &s3.DeleteObjectInput{}

	input = &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	if len(folder) > 0 {
		folder += key
		input = &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(folder),
		}
	}

	_, err = svc.DeleteObject(context.TODO(), input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			fmt.Println(err.Error())
		}
		return
	}
}

func verifyUserResources(t *testing.T, client *rancher.Client, resource []*management.User) {
	logrus.Info("Verifying user resources...")
	for i := range userList {
		user, err := users.GetUserIDByName(client, resource[i].Name)
		if err != nil {
			assert.Equal(t, "", user)
		}
		assert.NotNil(t, user)
	}
}

func verifyProjectResources(t *testing.T, client *rancher.Client, resource []*management.Project) {
	logrus.Info("Verifying project resources...")
	for i := range projList {
		project, err := client.Management.Project.ByID(resource[i].ID)
		if err != nil {
			assert.ErrorContains(t, err, resource[i].ID)
		}
		assert.Equal(t, project.ID, resource[i].ID)
	}
}

func verifyRoleResources(t *testing.T, client *rancher.Client, resource []*management.RoleTemplate) {
	logrus.Info("Verifying role resources...")
	for i := range roleList {
		roleTemplate, err := client.Management.RoleTemplate.ByID(resource[i].ID)
		if err != nil {
			assert.ErrorContains(t, err, resource[i].ID)
		}
		assert.Equal(t, roleTemplate.ID, resource[i].ID)
	}
}
