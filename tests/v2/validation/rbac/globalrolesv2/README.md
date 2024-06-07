# Global Roles v2
Global Roles v2 introduces enhanced capabilities, allowing users to define permissions across all downstream clusters. This update aims to address the limitations of predefined roles, particularly challenges associated with the Restricted Admin role. 

## Pre-requisites
- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.
- Some tests require creating additional downstream cluster. Providing the provisioningInput parameter with appropriate values is mandatory unless you are skipping those tests.

## Test Setup
* For [globalroles_v2 checks](globalroles_v2_test.go), your GO suite should be set to `-run ^TestGlobalRolesV2TestSuite$`. You can find specific tests by checking the test file you plan to run.
* For [globalroles_v2 webhook checks](globalroles_v2_webhook_test.go), your GO suite should be set to `-run ^TestGlobalRolesV2WebhookTestSuite$`. You can find specific tests by checking the test file you plan to run.

In your config file, set the following:
```yaml
rancher:
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True #optional
  cleanup: True #optional
  clusterName: "downstream_cluster_name"
provisioningInput:
 machinePools:
 - machinePoolConfig:                    
     etcd: true                            
     controlplane: true
     worker: true
     quantity: 1
 nodePools:
 - nodeRoles:
     etcd: true
     controlplane: true
     worker: true
     quantity: 1
 rke1KubernetesVersion:
   - "v1.28.10-rancher1-1"
 rke2KubernetesVersion:
   - "v1.28.10+rke2r1"
 k3sKubernetesVersion:
   - "v1.28.10+k3s1"
 cni:
   - "calico"
 providers:
   - "aws"
 nodeProviders:
   - "ec2"
awsCredentials:
 accessKey: "<Your Access Key>"
 secretKey: "<Your Secret Key>"
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
 awsAccessKeyID: "<Your Access Key>"
 awsSecretAccessKey: "<Your Secret Key>"
 awsEC2Config:
   - instanceType: "t3a.xlarge"
     awsRegionAZ: ""
     awsAMI: "<AMI>"
     awsSecurityGroups: ["sg-0e753fd5550206e55"]
     awsSSHKeyName: "<Your ssh key>"
     awsCICDInstanceTag: "rancher-validation"
     awsIAMProfile: "EngineeringUsersUS"
     awsUser: "ubuntu"
     volumeSize: 50
     roles: ["etcd", "controlplane", "worker"]
     isWindows: false
sshPath:
 sshPath: "<Your ssh path>"
```
