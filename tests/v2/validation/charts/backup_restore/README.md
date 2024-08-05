# Backup Restore Operator (BRO)
Backup Restore Operator (BRO for short) is a disaster recovery chart that can be installed on the local cluster only, and is used to create backups of Rancher resources contained in the `rancher-resource-set`. The resource set becomes available after the chart is installed and can be edited as a user needs. Once a backup is created and stored in either a local volume or in an S3 storage location (such as AWS S3 or Minio S3), a restore operation can be created to restore Rancher back to the backed up version of the Rancher resources.

## Pre-requisites
- Downstream RKE1 and RKE2 clusters are required before running the test(s)
- All tests require configs pulled in from the broInput parameter.

## Test Setup
In your config file, set the following:
```
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  userToken: "rancher_user_token"
  insecure: True
  cleanup: True
  clusterName: "downstream_cluster_name"
<<<<<<< HEAD
broInput:
=======
backupRestoreInput:
>>>>>>> 0b773e519 ([2.9] BRO In-Place Restore P0)
  backupName: ""
  s3BucketName: ""
  s3FolderName: ""
  s3Region: ""
  s3Endpoint: ""
  volumeName: ""
  credentialSecretNamespace: ""
  prune: true/false
  resourceSetName: ""
  accessKey: ""
  secretKey: ""
```