# Backup Restore Operator (BRO)
Backup Restore Operator (BRO for short) is a disaster recovery chart that can be installed on the local cluster only, and is used to create backups of Rancher resources contained in the `rancher-resource-set`. The resource set becomes available after the chart is installed and can be edited as a user needs. Once a backup is created and stored in either a local volume or in an S3 storage location (such as AWS S3 or Minio S3), a restore operation can be created to restore Rancher to the backed up version of the Rancher resources.

# Tests
- TestS3InPlaceRestore installs the BRO chart, creates two users, projects, and role templates in the local cluster, provisions a custom RKE1 and custom RKE2 cluster both with single nodes and all roles, creates a backup, verifies the backup exists within the given S3 bucket, creates two more users, projects, and role templates, runs an in-place restore, validates the first set of Rancher resources exists and the second set doesn't, and validates that the custom RKE1 and RKE2 clusters come back into the `Active` status.

## Pre-requisites
- All tests require configs pulled in from the backupRestoreInput, provisioningInput, awsEC2Configs, and sshPath parameters.
- The tests provision custom clusters, so you will need to provide custom cluster configs for the tests to run properly.

## Test Setup
In your config file, set the following:
```
rancher: 
  host: ""
  adminToken: ""
  insecure: true/false
  cleanup: true/false
  clusterName: ""
  
backupRestoreInput:
  backupName: ""
  s3BucketName: ""
  s3FolderName: ""
  s3Region: ""
  s3Endpoint: ""
  volumeName: "" # Optional
  credentialSecretNamespace: ""
  prune: true/false
  resourceSetName: ""
  accessKey: ""
  secretKey: ""

provisioningInput:
  rke1KubernetesVersion:
    - ""
  rke2KubernetesVersion:
    - ""
  k3sKubernetesVersion:
    - ""
  nodeProviders:
    - ""

awsEC2Configs:
  region: ""
  awsSecretAccessKey: ""
  awsAccessKeyID: ""
  awsEC2Config:
    - instanceType: ""
      awsRegionAZ: ""
      awsAMI: ""
      awsSecurityGroups: [""]
      awsSSHKeyName: ""
      awsCICDInstanceTag: ""
      awsIAMProfile: ""
      awsUser: ""
      volumeSize: # int
      roles: ["", "", ""] # etcd, controlplane, and worker are the options

sshPath:
  sshPath: ""
```