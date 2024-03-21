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
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/provisioning"
	"github.com/rancher/shepherd/extensions/provisioninginput"
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
	broConfig = setBroUserConfig()

	backupName       = namegen.AppendRandomString(broConfig["backupName"].(string))
	secondBackupName = namegen.AppendRandomString(broConfig["backupName"].(string))
	secretName       = broConfig["secretname"]
	member           = "user"
	roleName         = namegen.AppendRandomString("psarole")
	prune            = broConfig["prune"].(bool)

	rules = []management.PolicyRule{
		{
			APIGroups: []string{"management.cattle.io"},
			Resources: []string{"projects"},
			Verbs:     []string{psaRole},
		},
	}

	backup = bv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: backupName,
		},
		Spec: bv1.BackupSpec{
			ResourceSetName:            broConfig["resourceSetName"].(string),
			EncryptionConfigSecretName: broConfig["encryptionConfigSecretName"].(string),
			Schedule:                   broConfig["schedule"].(string),
			RetentionCount:             broConfig["retentionCount"].(int64),
		},
	}

	restore = bv1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "restore-",
		},
		Spec: bv1.RestoreSpec{
			BackupFilename:             backupName,
			Prune:                      &prune,
			DeleteTimeoutSeconds:       broConfig["deleteTimeoutSeconds"].(int),
			EncryptionConfigSecretName: broConfig["encryptionConfigSecretName"].(string),
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
)

type BucketBasics struct {
	S3Client *s3.Client
}

func installBroChart(t *testing.T, client *rancher.Client, chartInstallOptions *charts.InstallOptions, chartFeatureOptions *charts.RancherBackupOpts, project *management.Project) {
	_, err := createOpaqueS3Secret(client.Steve)
	require.NoError(t, err)

	logrus.Info("Installing backup chart")
	err = charts.InstallRancherBackupChart(client, chartInstallOptions, chartFeatureOptions, true)
	require.NoError(t, err)

	logrus.Info("Waiting backup chart deployments to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	require.NoError(t, err)

	logrus.Info("Waiting backup chart DaemonSets to have expected number of available nodes")
	err = charts.WatchAndWaitDaemonSets(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	require.NoError(t, err)

	logrus.Info("Waiting backup chart StatefulSets to have expected number of ready replicas")
	err = charts.WatchAndWaitStatefulSets(client, project.ClusterID, backupChartNamespace, metav1.ListOptions{})
	require.NoError(t, err)
}

func setBroUserConfig() map[string]interface{} {
	broConfig := new(charts.Config)
	config.LoadConfig(charts.ConfigurationFileKey, broConfig)

	config := map[string]interface{}{
		"backupName":                 broConfig.BackupName,
		"bucket":                     broConfig.S3BucketName,
		"folder":                     broConfig.S3FolderName,
		"region":                     broConfig.S3Region,
		"endpoint":                   broConfig.S3Endpoint,
		"secretName":                 broConfig.CredentialSecretName,
		"secretNamespace":            broConfig.CredentialSecretNamespace,
		"accessKey":                  broConfig.AccessKey,
		"secretKey":                  broConfig.SecretKey,
		"volumeName":                 broConfig.VolumeName,
		"endpointCA":                 broConfig.EndpointCA,
		"prune":                      broConfig.Prune,
		"tlsSkipVerify":              broConfig.TlsSkipVerify,
		"retentionCount":             broConfig.RetentionCount,
		"resourceSetName":            broConfig.ResourceSetName,
		"schedule":                   broConfig.Schedule,
		"encryptionConfigSecretName": broConfig.EncryptionConfigSecretName,
		"deleteTimeoutSeconds":       broConfig.DeleteTimoutSeconds,
	}

	return config
}

func createOpaqueS3Secret(steveClient *v1.Client) (string, error) {
	logrus.Infof("Creating an opaque secret with name: %v", secretName)
	config := setBroUserConfig()
	secretTemplate := secrets.NewSecretTemplate(secretName, config["secretNamespace"].(string), map[string][]byte{"accessKey": []byte(config["accessKey"].(string)), "secretKey": []byte(config["secretKey"].(string))}, corev1.SecretTypeOpaque)
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

func createDownstreamCluster(client *rancher.Client, clusterType string) (*management.Cluster, *v1.SteveAPIObject, *clusters.ClusterConfig, error) {
	provisioningConfig := new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, provisioningConfig)
	nodeProviders := provisioningConfig.NodeProviders[0]
	externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviders)
	testClusterConfig := clusters.ConvertConfigToClusterConfig(provisioningConfig)
	testClusterConfig.CNI = provisioningConfig.CNIs[0]

	var clusterObject *management.Cluster
	var steveObject *v1.SteveAPIObject
	var err error

	switch clusterType {
	case "RKE1":
		nodeAndRoles := []provisioninginput.NodePools{
			provisioninginput.WorkerNodePool,
			provisioninginput.WorkerNodePool,
			provisioninginput.WorkerNodePool,
			provisioninginput.ControlPlaneNodePool,
			provisioninginput.EtcdNodePool,
		}
		testClusterConfig.NodePools = nodeAndRoles
		testClusterConfig.KubernetesVersion = provisioningConfig.RKE1KubernetesVersions[0]
		clusterObject, _, err = provisioning.CreateProvisioningRKE1CustomCluster(client, &externalNodeProvider, testClusterConfig)
	case "RKE2":
		nodeAndRoles := []provisioninginput.MachinePools{
			provisioninginput.WorkerMachinePool,
			provisioninginput.WorkerMachinePool,
			provisioninginput.WorkerMachinePool,
			provisioninginput.ControlPlaneMachinePool,
			provisioninginput.ControlPlaneMachinePool,
			provisioninginput.EtcdMachinePool,
			provisioninginput.EtcdMachinePool,
			provisioninginput.EtcdMachinePool,
		}
		testClusterConfig.MachinePools = nodeAndRoles
		testClusterConfig.KubernetesVersion = provisioningConfig.RKE2KubernetesVersions[0]
		steveObject, err = provisioning.CreateProvisioningCustomCluster(client, &externalNodeProvider, testClusterConfig)
	default:
		return nil, nil, nil, fmt.Errorf("unsupported cluster type: %s", clusterType)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	return clusterObject, steveObject, testClusterConfig, nil
}

func createBackup(client *rancher.Client, name string) (*v1.SteveAPIObject, error) {
	backupTemplate := bv1.NewBackup(backupChartNamespace, name, backup)
	backupTemplate.Namespace = "" //sets the namespace to a blank string because if a namespace is set the create request fails
	completedBackup, err := client.Steve.SteveType(backupSteveType).Create(backupTemplate)

	return completedBackup, err
}

func createRestore(client *rancher.Client, fileName string) (*v1.SteveAPIObject, error) {
	restoreTemplate := bv1.NewRestore(backupChartNamespace, "", restore)
	restoreTemplate.Namespace = "" //sets the namespace to a blank string because if a namespace is set the create request fails
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

func verifyUserResourcesExist(t *testing.T, client *rancher.Client, resource *management.User) {
	logrus.Info("Verifying user resources exist...")
	user, err := users.GetUserIDByName(client, resource.Name)
	require.NoError(t, err)
	assert.NotNil(t, user)
}

func verifyUserResourcesNotExist(t *testing.T, client *rancher.Client, resource *management.User) {
	logrus.Info("Verifying user resources don't exist...")
	user, err := users.GetUserIDByName(client, resource.Name)
	require.NoError(t, err)
	assert.Equal(t, "", user)
}

func verifyProjectResourceExist(t *testing.T, client *rancher.Client, resource *management.Project) {
	logrus.Info("Verifying project resources exist...")
	project, err := client.Management.Project.ByID(resource.ID)
	require.NoError(t, err)
	assert.Equal(t, project.ID, resource.ID)
}

func verifyProjectResourceNotExist(t *testing.T, client *rancher.Client, resource *management.Project) {
	logrus.Info("Verifying project resources don't exist...")
	_, err := client.Management.Project.ByID(resource.ID)
	assert.ErrorContains(t, err, resource.ID)
}

func verifyRoleResourcesExist(t *testing.T, client *rancher.Client, resource *management.RoleTemplate) {
	logrus.Info("Verifying role resources exist...")
	roleTemplate, err := client.Management.RoleTemplate.ByID(resource.ID)
	require.NoError(t, err)
	assert.Equal(t, roleTemplate.ID, resource.ID)
}

func verifyRoleResourcesNotExist(t *testing.T, client *rancher.Client, resource *management.RoleTemplate) {
	logrus.Info("Verifying role resources don't exist...")
	_, err := client.Management.RoleTemplate.ByID(resource.ID)
	assert.ErrorContains(t, err, resource.ID)
}
