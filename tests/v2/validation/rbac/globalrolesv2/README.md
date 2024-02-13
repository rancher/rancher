# Global Roles v2
Global Roles v2 introduces enhanced capabilities, allowing users to define permissions across all downstream clusters. This update aims to address the limitations of predefined roles, particularly challenges associated with the Restricted Admin role. 

## Pre-requisites
- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.
- Some tests require creating additional downstream cluster. Providing the provisioningInput parameter with appropriate values is mandatory unless you are skipping those tests.

## Test Setup
Your GO suite should be set to `-run ^TestGlobalRolesV2TestSuite$`. You can find specific tests by checking the test file you plan to run.

In your config file, set the following:
```
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  userToken: "rancher_user_token"
  insecure: True
  cleanup: True
  clusterName: "downstream_cluster_name"
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
    - "v1.27.10+k3s2"
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