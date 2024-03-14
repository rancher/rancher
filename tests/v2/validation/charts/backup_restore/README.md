# Backup Restore Operator (BRO)
Backup Restore Operator (BRO for short) is a disaster recovery chart that can be installed on the local cluster only, and is used to create backups of Rancher resources contained in the `rancher-resource-set`. The resource set becomes available after the chart is installed and can be edited as a user needs. Once a backup is created and stored in either a local volume or in an S3 storage location (such as AWS S3 or Minio S3), a restore operation can be created to restore Rancher back to the backed up version of the Rancher resources.

## Pre-requisites
- All tests require creating additional downstream clusters. Providing the provisioningInput parameter with appropriate values is mandatory.
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

broInput:
  backupName: ""
  s3BucketName: ""
  s3FolderName: ""
  s3Region: ""
  s3Endpoint: ""
  volumeName: ""
  credentialSecretName: ""
  credentialSecretNamespace: ""
  tlsSkipVerify: true
  endpointCA: ""
  deleteTimeoutSeconds: 300
  retentionCount: 0
  prune: true
  resourceSetName: ""
  encryptionConfigSecretName: ""
  schedule: ""
  accessKey: ""
  secretKey: ""

provisioningInput:
  nodePools:
  - nodeRoles:
      etcd: true
      quantity: 1
  - nodeRoles:
      controlplane: true
      quantity: 1
  - nodeRoles:
      worker: true
      quantity: 1
  machinePools:
  - nodeRoles:
      etcd: true
      quantity: 1
  - nodeRoles:
      controlplane: true
      quantity: 1
  - nodeRoles:
      worker: true
      quantity: 1
  rke1KubernetesVersion:
    - "v1.27.6-rancher1-1"
  rke2KubernetesVersion:
    - "v1.27.6+rke2r1"
  k3sKubernetesVersion:
    - "v1.27.6+k3s1"
  cni:
    - "canal"
  providers: 
    - "aws"
  nodeProviders: 
    - "ec2"
  hardened: false

awsCredentials:
  secretKey: "aws_secret_key"
  accessKey: "aws_access_key"
  defaultRegion: "us-east-2"

awsMachineConfig: 
  region: "us-east-2"
  instanceType: "t3a.xlarge"
  sshUser: "ubuntu"
  vpcId: ""
  volumeType: "gp2"
  zone: "a"
  retries: 5
  rootSize: 50
  securityGroup: 
    - "rancher-nodes"

awsEC2Configs:
  region: "us-east-2"
  awsSecretAccessKey: "aws_secret_ke"
  awsAccessKeyID: "aws_access_key"
  awsEC2Config:
    - instanceType: "t3a.xlarge"
      awsRegionAZ: ""
      awsAMI: "ami-053835e36b16f97d0"
      awsSecurityGroups: ["sg-0e753fd5550206e55"]
      awsSSHKeyName: "jenkins-elliptic-validation.pem"
      awsCICDInstanceTag: "rancher-validation"
      awsIAMProfile: "EngineeringUsersUS"
      awsUser: "ec2-user"
      volumeSize: 50
      roles: ["etcd", "controlplane", "worker"]
      isWindows: false
sshPath: 
  sshPath: "ssh_path"
```