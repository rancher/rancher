package charts

import (
	"context"
	"fmt"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/extensions/secrets"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/sirupsen/logrus"

	bv1 "github.com/rancher/backup-restore-operator/pkg/apis/resources.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	backupName          = namegen.AppendRandomString("backup")
	secretName          = namegen.AppendRandomString("brosecretname")
	roleName            = namegen.AppendRandomString("broRole")
	backupRestoreConfig = new(charts.BackupRestoreConfig)

	rules = []management.PolicyRule{
		{
			APIGroups: []string{"management.cattle.io"},
			Resources: []string{"projects"},
			Verbs:     []string{backupRole},
		},
	}
)

const (
	backupEnabled        = true
	backupChartNamespace = "cattle-resources-system"
	backupChartName      = "rancher-backup"
	backupSteveType      = "resources.cattle.io.backup"
	restoreSteveType     = "resources.cattle.io.restore"
	backupRole           = "backupRole"
	clusterRoleContext   = "cluster"
	systemProject        = "System"
	localCluster         = "local"
	globalUserRole       = "user"
)

func setBackupObject() bv1.Backup {
	broConfig := backupRestoreConfig
	config.LoadConfig(charts.BackupRestoreConfigurationFileKey, broConfig)

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
	broConfig := backupRestoreConfig
	config.LoadConfig(charts.BackupRestoreConfigurationFileKey, broConfig)

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

func installBroChart(client *rancher.Client) error {
	project, err := projects.GetProjectByName(client, localCluster, systemProject)
	if err != nil {
		return err
	}

	// Get clusterName from config yaml
	clusterName := client.RancherConfig.ClusterName

	// Get cluster meta
	cluster, err := clusters.NewClusterMeta(client, clusterName)
	if err != nil {
		return err
	}

	// Get latest version of Rancher Backup chart
	latestBackupVersion, err := client.Catalog.GetLatestChartVersion(backupChartName, catalog.RancherChartRepo)
	if err != nil {
		return err
	}

	broConfig := backupRestoreConfig
	config.LoadConfig(charts.BackupRestoreConfigurationFileKey, broConfig)

	chartInstallOptions := &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestBackupVersion,
		ProjectID: project.ID,
	}
	chartFeatureOptions := &charts.RancherBackupOpts{
		VolumeName:                broConfig.VolumeName,
		BucketName:                broConfig.S3BucketName,
		CredentialSecretName:      secretName,
		CredentialSecretNamespace: broConfig.CredentialSecretNamespace,
		Enabled:                   backupEnabled,
		Endpoint:                  broConfig.S3Endpoint,
		Folder:                    broConfig.S3FolderName,
		Region:                    broConfig.S3Region,
	}

	_, err = createOpaqueS3Secret(client.Steve)
	if err != nil {
		return err
	}

	logrus.Info("Installing backup chart")
	err = charts.InstallRancherBackupChart(client, chartInstallOptions, chartFeatureOptions, true)
	if err != nil {
		return err
	}

	logrus.Info("Waiting for backup chart deployments to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	logrus.Info("Waiting for backup chart DaemonSets to have expected number of available nodes")
	err = charts.WatchAndWaitDaemonSets(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	logrus.Info("Waiting for backup chart StatefulSets to have expected number of ready replicas")
	err = charts.WatchAndWaitStatefulSets(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	return err
}

func createOpaqueS3Secret(steveClient *v1.Client) (string, error) {
	broConfig := backupRestoreConfig
	config.LoadConfig(charts.BackupRestoreConfigurationFileKey, broConfig)

	logrus.Infof("Creating an opaque secret with name: %v", secretName)
	secretTemplate := secrets.NewSecretTemplate(secretName, broConfig.CredentialSecretNamespace, map[string][]byte{"accessKey": []byte(broConfig.AccessKey), "secretKey": []byte(broConfig.SecretKey)}, corev1.SecretTypeOpaque)
	createdSecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)

	return createdSecret.Name, err
}

func createProject(client *rancher.Client, clusterID string) (*management.Project, error) {
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      systemProject,
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

func createRancherResources(client *rancher.Client, resourceCount int, clusterID string) ([]*management.User, []*management.Project, []*management.RoleTemplate, error) {
	userList := []*management.User{}
	projList := []*management.Project{}
	roleList := []*management.RoleTemplate{}

	for i := 0; i < resourceCount; i++ {
		u, err := users.CreateUserWithRole(client, users.UserConfig(), globalUserRole)
		if err != nil {
			return userList, projList, roleList, err
		}
		userList = append(userList, u)

		p, err := createProject(client, clusterID)
		if err != nil {
			return userList, projList, roleList, err
		}
		projList = append(projList, p)

		rt, err := createRole(client, clusterRoleContext, roleName, rules)
		if err != nil {
			return userList, projList, roleList, err
		}
		roleList = append(roleList, rt)
	}

	return userList, projList, roleList, nil
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

func validateAWSS3BackupExists(bucket string, folder string, key string) (bool, error) {
	if len(folder) > 0 {
		folder += key
		isPresent, err := checkAWSS3Object(bucket, folder)
		return isPresent, err
	}
	isPresent, err := checkAWSS3Object(bucket, key)
	return isPresent, err
}

func checkAWSS3Object(bucket string, key string) (bool, error) {
	sdkConfig, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return false, err
	}

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

func deleteAWSBackup(bucket string, folder string, key string) error {
	sdkConfig, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}
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
		return err
	}

	return err
}

func verifyRancherResources(client *rancher.Client, userList []*management.User, projList []*management.Project, roleList []*management.RoleTemplate) error {
	logrus.Info("Verifying user resources...")
	for i := range userList {
		_, err := users.GetUserIDByName(client, userList[i].Name)
		if err != nil {
			return err
		}
	}

	logrus.Info("Verifying project resources...")
	for i := range projList {
		_, err := client.Management.Project.ByID(projList[i].ID)
		if err != nil {
			return err
		}
	}

	logrus.Info("Verifying role resources...")
	for i := range roleList {
		_, err := client.Management.RoleTemplate.ByID(roleList[i].ID)
		if err != nil {
			return err
		}
	}

	return nil
}
