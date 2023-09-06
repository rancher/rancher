# RKE2 Provisioning Configs

For your config, you will need everything in the Prerequisites section on the previous readme, [Define your test](#provisioning-input), and at least one [Cloud Credential](#cloud-credentials) and [Node Driver Machine Config](#machine-rke2-config) or [Custom Cluster Template](#custom-cluster), which should match what you have specified in `provisioningInput`. 

Your GO test_package should be set to `provisioning/rke2`.
Your GO suite should be set to `-run ^TestRKE2ProvisioningTestSuite$`.
Please see below for more details for your config. Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Prerequisites](../README.md)
2. [Define your test](#provisioning-input)
3. [Cloud Credential](#cloud-credentials)
4. [Configure providers to use for Node Driver Clusters](#machine-rke2-config)
5. [Configuring Custom Clusters](#custom-cluster)
6. [Advanced Cluster Settings](#advanced-settings)
7. [Back to general provisioning](../README.md)

## Provisioning Input
provisioningInput is needed to the run the RKE2 tests, specifically kubernetesVersion, cni, and providers. nodesAndRoles is only needed for the TestProvisioningDynamicInput test, node pools are divided by "{nodepool},". psact is optional and takes values `rancher-privileged`, `rancher-restricted` or `rancher-baseline`.

**nodeProviders is only needed for custom cluster tests; the framework only supports custom clusters through aws/ec2 instances.**
```yaml
provisioningInput:
  machinePools:
  - nodeRoles:
      etcd: true
      controlplane: true
      worker: true
      quantity: 1
  - nodeRoles:
      worker: true
      quantity: 2
  - nodeRoles:
      windows: true
      quantity: 1
  flags:
    desiredflags: "Short|Long" #These flags are for running TestProvisioningRKE2Cluster or TestProvisioningRKE2CustomCluster it is not needed for the dynamic tests.
  rke2KubernetesVersion: ["v1.26.8+rke2r1"]
  cni: ["calico"]
  providers: ["linode", "aws", "do", "harvester"]
  nodeProviders: ["ec2"]
  hardened: false
  psact: ""
  clusterSSHTests: [""]
```

## Cloud Credentials
These are the inputs needed for the different node provider cloud credentials, inlcuding linode, aws, digital ocean, harvester, azure, and google.

### Digital Ocean
```yaml
digitalOceanCredentials:
  accessToken": ""
```
### Linode
```yaml
linodeCredentials:
  token: ""
```
### Azure
```yaml
azureCredentials:
  clientId: ""
  clientSecret: ""
  subscriptionId": ""
  environment: "AzurePublicCloud"
```
### AWS
```yaml
awsCredentials:
  secretKey: "",
  accessKey: "",
  defaultRegion: ""
```
### Harvester
```yaml
harvesterCredentials:
  clusterId: "",
  clusterType: "",
  kubeconfigContent: ""
```
### Google
```yaml
googleCredentials:
  authEncodedJson: ""
```
### VSphere
```yaml
vmwarevsphereCredentials:
  password: ""
  username: ""
  vcenter: ""
  vcenterPort: ""
```

## Machine RKE2 Config
Machine RKE2 config is the final piece needed for the config to run RKE2 provisioning tests.

### AWS RKE2 Machine Config
```yaml
awsMachineConfig:
  region: "us-east-2"
  ami: ""
  instanceType: "t3a.medium"
  sshUser: "ubuntu"
  vpcId: ""
  volumeType: "gp2"
  zone: "a"
  retries: "5"
  rootSize: "60"
  securityGroup: [""]
```
### Digital Ocean RKE2 Machine Config
```yaml
doMachineConfig:
  image: "ubuntu-20-04-x64"
  backups: false
  ipv6: false
  monitoring: false
  privateNetworking: false
  region: "nyc3"
  size: "s-2vcpu-4gb"
  sshKeyContents: ""
  sshKeyFingerprint: ""
  sshPort: "22"
  sshUser: "root"
  tags: ""
  userdata: ""
```
### Linode RKE2 Machine Config
```yaml
linodeMachineConfig:
  authorizedUsers: ""
  createPrivateIp: true
  dockerPort: "2376"
  image: "linode/ubuntu22.04"
  instanceType: "g6-standard-8"
  region: "us-west"
  rootPass: ""
  sshPort: "22"
  sshUser: ""
  stackscript: ""
  stackscriptData: ""
  swapSize: "512"
  tags: ""
  uaPrefix: "Rancher"
```
### Azure RKE2 Machine Config
```yaml
azureMachineConfig:
  availabilitySet: "docker-machine"
  diskSize: "30"
  environment: "AzurePublicCloud"
  faultDomainCount: "3"
  image: "canonical:UbuntuServer:22.04-LTS:latest"
  location: "westus"
  managedDisks: false
  noPublicIp: false
  nsg: ""
  openPort: ["6443/tcp", "2379/tcp", "2380/tcp", "8472/udp", "4789/udp", "9796/tcp", "10256/tcp", "10250/tcp", "10251/tcp", "10252/tcp"]
  resourceGroup: "docker-machine"
  size: "Standard_D2_v2"
  sshUser: "docker-user"
  staticPublicIp: false
  storageType: "Standard_LRS"
  subnet: "docker-machine"
  subnetPrefix: "192.168.0.0/16"
  updateDomainCount: "5"
  usePrivateIp: false
  vnet: "docker-machine-vnet"
```
### Harvester RKE2 Machine Config
```yaml
harvesterMachineConfig":
  diskSize: "40"
  cpuCount: "2"
  memorySize: "8"
  networkName: "default/ctw-network-1"
  imageName: "default/image-rpj98"
  vmNamespace: "default"
  sshUser: "ubuntu"
  diskBus: "virtio
```
## Vsphere RKE2 Machine Config
```yaml
vmwarevsphereMachineConfig:
  cfgparam: ["disk.enableUUID=TRUE"]
  cloneFrom: ""
  cloudinit: ""
  contentLibrary: ""
  cpuCount: "4"
  creationType: ""
  datacenter: ""
  datastore: ""
  datastoreCluster: ""
  diskSize: "20000"
  folder: ""
  hostSystem: ""
  memorySize: "4096"
  network: [""]
  os: "linux"
  password: ""
  pool: ""
  sshPassword: ""
  sshPort: "22"
  sshUser: ""
  sshUserGroup: ""
  username: ""
  vcenter: ""
  vcenterPort: "443"
```

These tests utilize Go build tags. Due to this, see the below examples on how to run the node driver tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke2 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE2ProvisioningTestSuite/TestProvisioningRKE2Cluster"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke2 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE2ProvisioningTestSuite/TestProvisioningRKE2ClusterDynamicInput"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.

## Custom Cluster
For custom clusters, no machineConfig or credentials are needed. Currently only supported for ec2.

Dependencies:
* **Ensure you have nodeProviders in provisioningInput**
* make sure that all roles are entered at least once
* windows pool(s) should always be last in the config
```yaml
  awsEC2Configs:
  region: "us-east-2"
  awsSecretAccessKey: ""
  awsAccessKeyID: ""
  awsEC2Config:
    - instanceType: "t3a.medium"
      awsRegionAZ: ""
      awsAMI: ""
      awsSecurityGroups: [""]
      awsSSHKeyName: ""
      awsCICDInstanceTag: "rancher-validation"
      awsIAMProfile: ""
      awsUser: "ubuntu"
      volumeSize: 50
      roles: ["etcd", "controlplane"]
    - instanceType: "t3a.medium"
      awsRegionAZ: ""
      awsAMI: ""
      awsSecurityGroups: [""]
      awsSSHKeyName: ""
      awsCICDInstanceTag: "rancher-validation"
      awsIAMProfile: ""
      awsUser: "ubuntu"
      volumeSize: 50
      roles: ["worker"]
    - instanceType: "t3a.xlarge"
      awsAMI: ""
      awsSecurityGroups: [""]
      awsSSHKeyName: ""
      awsCICDInstanceTag: "rancher-validation"
      awsUser: "Administrator"
      volumeSize: 50
      roles: ["windows"]
```

These tests utilize Go build tags. Due to this, see the below examples on how to run the custom cluster tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke2 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCustomClusterRKE2ProvisioningTestSuite/TestProvisioningRKE2CustomCluster"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke2 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCustomClusterRKE2ProvisioningTestSuite/TestProvisioningRKE2CustomClusterDynamicInput"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.

## Advanced Settings
This encapsulates any other setting that is applied in the cluster.spec. Currently we have support for:
* cluster agent customization 
* fleet agent customization

Please read up on general k8s to get an idea of correct formatting for:
* resource requests
* resource limits
* node affinity
* tolerations

```yaml
advancedOptions:
  clusterAgent: # change this to fleetAgent for fleet agent
    appendTolerations:
    - key: "Testkey"
      value: "testValue"
      effect: "NoSchedule"
    overrideResourceRequirements:
      limits:
        cpu: "750m"
        memory: "500Mi"
      requests:
        cpu: "250m"
        memory: "250Mi"
      overrideAffinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - preference:
                matchExpressions:
                  - key: "cattle.io/cluster-agent"
                    operator: "In"
                    values:
                      - "true"
              weight: 1
```