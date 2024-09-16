package backup_restore

import (
  "context"
  "errors"
  "fmt"
  "strings"
  "testing"

  awsConfig "github.com/aws/aws-sdk-go-v2/config"
  "github.com/aws/aws-sdk-go-v2/service/s3"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/rancher/rancher/tests/v2/actions/charts"
  "github.com/rancher/rancher/tests/v2/actions/projects"
  "github.com/rancher/rancher/tests/v2/actions/secrets"
  "github.com/rancher/shepherd/extensions/clusters/kubernetesversions"

  "github.com/rancher/rancher/tests/v2/actions/provisioning"
  "github.com/rancher/rancher/tests/v2/actions/provisioninginput"
  shepClusters "github.com/rancher/shepherd/extensions/clusters"

  "github.com/rancher/rancher/tests/v2/actions/clusters"
  "github.com/rancher/shepherd/clients/rancher"
  shepCharts "github.com/rancher/shepherd/extensions/charts"
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
  secretName          = namegen.AppendRandomString("bro-secret")
  backupRestoreConfig = new(charts.BackupRestoreConfig)

  rules = []management.PolicyRule{
    {
      APIGroups: []string{"management.cattle.io"},
      Resources: []string{"projects"},
      Verbs:     []string{"backupRole"},
    },
  }
)

const (
  backupEnabled    = true
  backupSteveType  = "resources.cattle.io.backup"
  restoreSteveType = "resources.cattle.io.restore"
  cluster          = "local"
  resourceCount    = 2
  cniCalico        = "calico"
  provider         = "aws"
)

func setBackupObject(broConfig *charts.BackupRestoreConfig) bv1.Backup {
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

func setRestoreObject(broConfig *charts.BackupRestoreConfig) bv1.Restore {
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
  project, err := projects.GetProjectByName(client, cluster, "System")
  if err != nil {
    return err
  }

  cluster, err := shepClusters.NewClusterMeta(client, cluster)
  if err != nil {
    return err
  }

  latestBackupVersion, err := client.Catalog.GetLatestChartVersion("rancher-backup", catalog.RancherChartRepo)
  if err != nil {
    return err
  }

  config.LoadConfig(charts.BackupRestoreConfigurationFileKey, backupRestoreConfig)

  chartInstallOptions := charts.InstallOptions{
    Cluster:   cluster,
    Version:   latestBackupVersion,
    ProjectID: project.ID,
  }
  chartFeatureOptions := &charts.RancherBackupOpts{
    VolumeName:                backupRestoreConfig.VolumeName,
    BucketName:                backupRestoreConfig.S3BucketName,
    CredentialSecretName:      secretName,
    CredentialSecretNamespace: backupRestoreConfig.CredentialSecretNamespace,
    Enabled:                   backupEnabled,
    Endpoint:                  backupRestoreConfig.S3Endpoint,
    Folder:                    backupRestoreConfig.S3FolderName,
    Region:                    backupRestoreConfig.S3Region,
  }

  _, err = createOpaqueS3Secret(client.Steve)
  if err != nil {
    return err
  }

  logrus.Info("Installing backup chart")
  err = charts.InstallRancherBackupChart(client, &chartInstallOptions, chartFeatureOptions, true)
  if err != nil {
    return err
  }

  logrus.Info("Waiting for backup chart deployments to have expected number of available replicas")
  err = shepCharts.WatchAndWaitDeployments(client, project.ClusterID, "cattle-resources-system", metav1.ListOptions{})
  if err != nil {
    return err
  }

  return err
}

func createOpaqueS3Secret(steveClient *v1.Client) (string, error) {
  config.LoadConfig(charts.BackupRestoreConfigurationFileKey, backupRestoreConfig)

  logrus.Infof("Creating an opaque secret with name: %v", secretName)
  secretTemplate := secrets.NewSecretTemplate(secretName, backupRestoreConfig.CredentialSecretNamespace, map[string][]byte{"accessKey": []byte(backupRestoreConfig.AccessKey), "secretKey": []byte(backupRestoreConfig.SecretKey)}, corev1.SecretTypeOpaque)
  createdSecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)

  return createdSecret.Name, err
}

func createRancherResources(client *rancher.Client, clusterID string, context string) ([]*management.User, []*management.Project, []*management.RoleTemplate, error) {
  userList := []*management.User{}
  projList := []*management.Project{}
  roleList := []*management.RoleTemplate{}

  for i := 0; i < resourceCount; i++ {
    u, err := users.CreateUserWithRole(client, users.UserConfig(), "user")
    if err != nil {
      return userList, projList, roleList, err
    }
    userList = append(userList, u)

    p, _, err := projects.CreateProjectAndNamespace(client, clusterID)
    if err != nil {
      return userList, projList, roleList, err
    }
    projList = append(projList, p)

    rt, err := client.Management.RoleTemplate.Create(
      &management.RoleTemplate{
        Context: context,
        Name:    namegen.AppendRandomString("bro-role"),
        Rules:   rules,
      })
    if err != nil {
      return userList, projList, roleList, err
    }
    roleList = append(roleList, rt)
  }

  return userList, projList, roleList, nil
}

func createAndValidateBackup(client *rancher.Client, bucket string, config *charts.BackupRestoreConfig) (*v1.SteveAPIObject, string, error) {
  backup := setBackupObject(config)
  backupTemplate := bv1.NewBackup("", backupName, backup)
  completedBackup, err := client.Steve.SteveType(backupSteveType).Create(backupTemplate)
  if err != nil {
    return nil, "", err
  }

  charts.VerifyBackupCompleted(client, backupSteveType, completedBackup)

  backupObj, err := client.Steve.SteveType(backupSteveType).ByID(backupName)
  if err != nil {
    return nil, "", err
  }

  backupK8Obj := &bv1.Backup{}
  err = v1.ConvertToK8sType(backupObj.JSONResp, backupK8Obj)
  if err != nil {
    return nil, "", err
  }

  backupFileName := backupK8Obj.Status.Filename

  client.Session.RegisterCleanupFunc(func() error {
    err = deleteAWSObject(bucket, backupFileName)
    if err != nil {
      return err
    }

    _, err = checkAWSS3Object(bucket, backupFileName)
    if err != nil {
      s3Error := err.Error()
      errorText := strings.Contains(s3Error, "404")
      if !errorText {
        return err
      }
    }

    return nil
  })

  return completedBackup, backupFileName, err
}

func createRKE1dsCluster(t *testing.T, client *rancher.Client) (*management.Cluster, *clusters.ClusterConfig, error) {
  provisioningConfig := new(provisioninginput.Config)
  config.LoadConfig(provisioninginput.ConfigurationFileKey, provisioningConfig)

  if provisioningConfig.NodeProviders == nil {
    provisioningConfig.NodeProviders = []string{"ec2"}
  }

  externalNodeProvider := provisioning.ExternalNodeProviderSetup(provisioningConfig.NodeProviders[0])
  testClusterConfig := clusters.ConvertConfigToClusterConfig(provisioningConfig)

  if provisioningConfig.RKE1KubernetesVersions == nil {
    rke1Versions, err := kubernetesversions.ListRKE1AllVersions(client)
    if err != nil {
      return nil, nil, err
    }

    provisioningConfig.RKE1KubernetesVersions = []string{rke1Versions[len(rke1Versions)-1]}
  }

  if provisioningConfig.CNIs == nil {
    provisioningConfig.CNIs = []string{cniCalico}
  }

  nodeAndRoles := []provisioninginput.NodePools{
    provisioninginput.AllRolesNodePool,
  }

  testClusterConfig.NodePools = nodeAndRoles
  testClusterConfig.KubernetesVersion = provisioningConfig.RKE1KubernetesVersions[0]
  clusterObject, _, err := provisioning.CreateProvisioningRKE1CustomCluster(client, &externalNodeProvider, testClusterConfig)

  if err != nil {
    return nil, nil, err
  }

  provisioning.VerifyRKE1Cluster(t, client, testClusterConfig, clusterObject)

  return clusterObject, testClusterConfig, nil
}

func createRKE2dsCluster(t *testing.T, client *rancher.Client) (*v1.SteveAPIObject, *clusters.ClusterConfig, error) {
  provisioningConfig := new(provisioninginput.Config)
  config.LoadConfig(provisioninginput.ConfigurationFileKey, provisioningConfig)
  nodeProviders := provisioningConfig.NodeProviders[0]
  externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviders)

  testClusterConfig := clusters.ConvertConfigToClusterConfig(provisioningConfig)

  if provisioningConfig.RKE2KubernetesVersions == nil {
    rke2Versions, err := kubernetesversions.ListRKE2AllVersions(client)
    if err != nil {
      return nil, nil, err
    }

    provisioningConfig.RKE2KubernetesVersions = []string{rke2Versions[len(rke2Versions)-1]}
  }

  if provisioningConfig.CNIs == nil {
    provisioningConfig.CNIs = []string{cniCalico}
  }

  nodeAndRoles := []provisioninginput.MachinePools{
    provisioninginput.AllRolesMachinePool,
  }
  testClusterConfig.MachinePools = nodeAndRoles
  testClusterConfig.KubernetesVersion = provisioningConfig.RKE2KubernetesVersions[0]
  steveObject, err := provisioning.CreateProvisioningCustomCluster(client, &externalNodeProvider, testClusterConfig)

  if err != nil {
    return nil, nil, err
  }

  provisioning.VerifyCluster(t, client, testClusterConfig, steveObject)

  return steveObject, testClusterConfig, nil
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
    return false, err
  }
  return true, nil
}

func deleteAWSObject(bucket string, key string) error {
  sdkConfig, err := awsConfig.LoadDefaultConfig(context.TODO())
  if err != nil {
    return err
  }
  svc := s3.NewFromConfig(sdkConfig)

  input := &s3.DeleteObjectInput{
    Bucket: aws.String(bucket),
    Key:    aws.String(key),
  }

  _, err = svc.DeleteObject(context.TODO(), input)
  if err != nil {
    return err
  }

  return err
}

func verifyRancherResources(client *rancher.Client, curUserList []*management.User, curProjList []*management.Project, curRoleList []*management.RoleTemplate) error {
  var errs []error

  logrus.Info("Verifying user resources...")
  for _, user := range curUserList {
    userID, err := users.GetUserIDByName(client, user.Name)
    if err != nil {
      errs = append(errs, fmt.Errorf("user %s: %w", user.Name, err))
    } else if userID == "" {
      errs = append(errs, fmt.Errorf("user %s not found", user.Name))
    }
  }

  logrus.Info("Verifying project resources...")
  for _, proj := range curProjList {
    _, err := client.Management.Project.ByID(proj.ID)
    if err != nil {
      errs = append(errs, fmt.Errorf("project %s: %w", proj.ID, err))
    }
  }

  logrus.Info("Verifying role resources...")
  for _, role := range curRoleList {
    _, err := client.Management.RoleTemplate.ByID(role.ID)
    if err != nil {
      errs = append(errs, fmt.Errorf("role %s: %w", role.ID, err))
    }
  }

  return errors.Join(errs...)
}
