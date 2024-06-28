# RKE1 Provisioning Configs

For your config, you will need everything in the [Prerequisites](../README.md) section on the previous readme, [Define your test](#Provisioning-Input), and at least one [Node Driver Cluster Template](#NodeTemplateConfigs) or [Custom Cluster Template](#Custom-Cluster), which should match what you have specified in `provisioningInput`. 

Your GO test_package should be set to `provisioning/rke1`.
Your GO suite should be set to `-run ^TestRKE1ProvisioningTestSuite$`.
Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Prerequisites](../README.md)
2. [Configuring test flags](#Flags)
3. [Define your test](#Provisioning-Input)
4. [Cloud Credential](#cloud-credentials)
5. [Configuring providers to use for Node Driver Clusters](#NodeTemplateConfigs)
6. [Configuring Custom Clusters](#Custom-Cluster)
7. [Static test cases](#static-test-cases)
8. [Cloud Provider](#Cloud-Provider)
9. [Advanced Cluster Settings](#advanced-settings)
10. [Back to general provisioning](../README.md)


## Flags
Flags are used to determine which static table tests are run (has no effect on dynamic tests) 
`Long` Will run the long version of the table tests (usually all of them)
`Short` Will run the subset of table tests with the short flag.

```yaml
flags:
  desiredflags: "Long"
```

## Provisioning Input
provisioningInput is needed to the run the RKE1 tests, specifically kubernetesVersion, cni, and providers. nodesAndRoles is only needed for the TestProvisioningDynamicInput test, node pools are divided by "{nodepool},". psact is optional and takes values `rancher-privileged`, `rancher-restricted` or `rancher-baseline`.

**nodeProviders is only needed for custom cluster tests; the framework only supports custom clusters through aws/ec2 instances.**
```yaml
provisioningInput:
  nodePools:
  - nodeRoles:
      etcd: true
      controlplane: true
      worker: true
      quantity: 1
  - nodeRoles:
      worker: true
      drainBeforeDelete: true
      quantity: 2
  rke1KubernetesVersion: ["v1.28.10-rancher1-1"]
  cni: ["calico"]
  providers: ["linode", "aws", "do", "harvester", "vsphere", "azure"]
  cloudProvider: "" # either: external-aws|rancher-vsphere
  nodeProviders: ["ec2"]
  psact: ""
  criDockerd: false
  etcdRKE1:
    backupConfig:
      enabled: true
      intervalHours: 12
      safeTimestamp: true
      timeout: 120
      s3BackupConfig:
        accessKey: ""
        bucketName: ""
        endpoint: ""
        folder: ""
        region: ""
        secretKey: ""
    retention: "72h"
    snapshot: false
chartUpgrade: # will install a version of the out-of-tree chart (latest - 1) that can later be upgraded to the latest version. This is used for upgrade testing on cloud provider tests.
  isUpgradable: false
```

## Cloud Credentials
These are the inputs needed for the different node provider cloud credentials, including linode, aws, harvester, azure, and google.

### Digital Ocean
```yaml
digitalOceanCredentials:               
  accessToken: ""                     #required
```
### Linode
```yaml
linodeCredentials:                   
  token: ""                           #required
```
### Azure
```yaml
azureCredentials:                     
  clientId: ""                        #required
  clientSecret: ""                    #required
  subscriptionId: ""                  #required
  environment: "AzurePublicCloud"     #required
```
### AWS
```yaml
awsCredentials:                       
  secretKey: ""                       #required
  accessKey: ""                       #required
  defaultRegion: ""                   #required
```
### Harvester
```yaml
harvesterCredentials:                 
  clusterId: ""                       #required
  clusterType: ""                     #required
  kubeconfigContent: ""               #required
```
### Google
```yaml
googleCredentials:                    
  authEncodedJson: ""                 #required
```
### VSphere
```yaml
vmwarevsphereCredentials:             
  password: ""                        #required
  username: ""                        #required
  vcenter: ""                         #required
  vcenterPort: ""                     #required
```


## Cloud Provider
Cloud Provider enables additional options through the cloud provider, like cloud persistent storage or cloud provisioned load balancers.

Names of cloud provider options are typically controlled by rancher product. Hence the discrepancy in rke2 vs. rke1 AWS in-tree and out-of-tree options. 
To use automation with a cloud provider, simply enter one of the following options in the `cloudProvider` field in the config. 

### RKE1 Cloud Provider Options
* external-aws
* aws
* rancher-vsphere

### RKE1 Chart Upgrade Options
At the root level of your config.yaml, you can provide the following option:
```yaml
chartUpgrade:
  isUpgradable: true
```
which will install `latest-1` chart version for CPI and CSI charts so that you may run upgrade tests later, if you wish. 
This is currently only available for Vsphere RKE1

## NodeTemplateConfigs
RKE1 specifically needs a node template config to run properly. These are the inputs needed for the different node providers.
Top level node template config entries can be set. The top level nodeTemplate is optional, and is not need for the different node
providers to work.
```yaml
  nodeTemplate:
    engineInstallURL: "testNT"
    name:             "testNTName"
```
### AWS
```yaml
  awsNodeConfig:
    ami: ""
    blockDurationMinutes: "0"
    encryptEbsVolume: false
    endpoint: ""
    httpEndpoint: "enabled"
    httpTokens: "optional"
    iamInstanceProfile: ""
    insecureTransport: false
    instanceType: "t2.2xlarge"
    keypairName: "your-key-name"
    kmsKey: ""
    monitoring: false
    privateAddressOnly: false
    region: "us-east-2"
    requestSpotInstance: true
    retries: "5"
    rootSize: "16"
    securityGroup: ["open-all"]
    securityGroupReadonly: false
    sessionToken: ""
    spotPrice: "0.50"
    sshKeyContents: ""
    sshUser: "ec2-user"
    subnetId: ""
    tags: ""
    type: "amazonec2Config"
    useEbsOptimizedInstance: false
    usePrivateAddress: false
    userdata: ""
    volumeType: "gp2"
    vpcId: ""
    zone: "a"
```

### Azure
```yaml
azureNodeConfig:
  availabilitySet: "docker-machine"
  customData: ""
  diskSize: "30"
  dns: ""
  dockerPort: "2376"
  environment: "AzurePublicCloud"
  faultDomainCount: "3"
  image: "canonical:UbuntuServer:22.04-LTS:latest"
  location: "eastus2"
  managedDisks: false
  noPublicIp: false
  openPort: ["6443/tcp","2379/tcp","2380/tcp","8472/udp","4789/udp","9796/tcp","10256/tcp","10250/tcp","10251/tcp","10252/tcp"]
  plan: ""
  privateIpAddress: ""
  resourceGroup: ""
  size: "Standard_D2_v2"
  sshUser: "azureuser"
  staticPublicIp: false
  storageType: "Standard_LRS"
  subnet: "docker-machine"
  subnetPrefix: "192.168.0.0/16"
  subscriptionId: ""
  type: "azureConfig"
  updateDomainCount: "5"
  vnet: "docker-machine-vnet"
```

### Harvester
```yaml
harvesterNodeConfig":
  cloudConfig: ""
  cpuCount: "2"
  diskBus: "virtio"
  diskSize: "40"
  imageName: "default/image-gchq8"
  keyPairName: ""
  memorySize: "4"
  networkData: ""
  networkModel: "virtio"
  networkName: ""
  networkType: "dhcp"
  sshPassword: ""
  sshPort: "22"
  sshPrivateKeyPath: ""
  sshUser: "ubuntu"
  type: "harvesterConfig"
  userData: ""
  vmAffinity: ""
  vmNamespace: "default
```

### Linode
```yaml
linodeNodeConfig:
  authorizedUsers: ""
  createPrivateIp: true
  dockerPort: "2376"
  image: "linode/ubuntu22.04"
  instanceType: "g6-dedicated-8"
  label: ""
  region: "us-west"
  rootPass: ""
  sshPort: "22"
  sshUser: "root"
  stackscript: ""
  stackscriptData: ""
  swapSize: "512"
  tags: ""
  type: "linodeConfig"
  uaPrefix: "Rancher"
```

### Vsphere
```yaml
vmwarevsphereNodeConfig:
  cfgparam: ["disk.enableUUID=TRUE"]
  cloneFrom: ""
  cloudinit: "#cloud-config\n\n"
  contentLibrary: ""
  cpuCount: "4"
  creationType: ""
  datacenter: ""
  datastore: ""
  datastoreURL: ""
  datastoreCluster: ""
  diskSize: "20000"
  folder: ""
  hostSystem: ""
  memorySize: "4096"
  network: [""]
  os: ""
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

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke1 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1ProvisioningTestSuite/TestProvisioningRKE1Cluster"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke1 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1ProvisioningTestSuite/TestProvisioningRKE1ClusterDynamicInput"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.

## Cloud Provider
Cloud Provider enables additional options such as load-balancers and storage devices to be provisioned through your cluster
available options:
### AWS
* `aws` uses the in-tree provider for aws -- **Deprecated on kubernetes 1.26 and below**
* `external-aws` uses the out-of-tree provider for aws. Built in logic to the automation will be applied to the cluster that applies the correct configuration for the out-of-tree charts to be installed. Supported on kubernetes 1.22+. An AWS provided LB will be attached to a workload in order to test that the cloud provider is working as expected. 

### Vsphere
* `rancher-vsphere` is out-of-tree since 1.22. A workload using vsphere's cloud provider storage class will be created on the cluster to test that the provider is working as expected. 

## Custom Cluster
For custom clusters, no nodeTemplateConfig or credentials are required. Currently only supported for ec2.

Dependencies:
* **Ensure you have nodeDrivers set in provisioningInput**
* make sure that all roles are entered at least once in nodePools.nodeRoles
* ensure nodeProviders is set
* use AMIs that already have Docker installed and the service is enabled on boot

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
```

These tests utilize Go build tags. Due to this, see the below examples on how to run the custom cluster tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke1 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCustomClusterRKE1ProvisioningTestSuite/TestProvisioningRKE1CustomCluster"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke1 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCustomClusterRKE1ProvisioningTestSuite/TestProvisioningRKE1CustomClusterDynamicInput"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.

## RKE1 Support Matrix Checks - K8s Components & Architecture
Custom clusters have the ability to perform RKE1 support matrix checks for K8s components & architectures. 

To do this, ensure that your `provisioningInput` has `flannel` and `canal` both defined, along with your desired K8s versions as shown below:

```yaml
provisioningInput:
  nodePools:
  - nodeRoles:
      etcd: true
      controlplane: true
      worker: true
      quantity: 1
  - nodeRoles:
      worker: true
      quantity: 2
  rke1KubernetesVersion: ["v1.26.9-rancher1-1", "v1.27.6-rancher1-1"]
  cni: ["flannel", "calico"]
  nodeProviders: ["ec2"]
  psact: ""
```

## Static Test Cases
In an effort to have uniform testing across our internal QA test case reporter, there are specific test cases that are put into their respective test files. This section highlights those test cases.

### PSACT
These test cases cover the following PSACT values as both an admin and standard user:
1. `rancher-privileged`
2. `rancher-restricted`
3. `rancher-baseline`

See an example YAML below:

```yaml
rancher:
  host: "<rancher server url>"
  adminToken: "<rancher admin bearer token>"
  cleanup: false
  clusterName: "<provided cluster name>"
  insecure: true
provisioningInput:
  rke1KubernetesVersion: ["v1.27.10-rancher1-1"]
  cni: ["calico"]
  providers: ["linode"]
  nodeProviders: ["ec2"]
nodeTemplate:
  engineInstallURL: "https://releases.rancher.com/install-docker/23.0.sh"
linodeNodeConfig:
  authorizedUsers: ""
  createPrivateIp: true
  dockerPort: "2376"
  image: "linode/ubuntu22.04"
  instanceType: "g6-dedicated-8"
  label: ""
  region: "us-west"
  rootPass: ""
  sshPort: "22"
  sshUser: "root"
  stackscript: ""
  stackscriptData: ""
  swapSize: "512"
  tags: ""
  token: ""
  type: "linodeConfig"
  uaPrefix: "Rancher"
```

These tests utilize Go build tags. Due to this, see the below examples on how to run the tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke1 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1PSACTTestSuite$"`

### Hardened Custom Cluster
This will provision a hardened custom cluster that runs across the following CIS scan profiles:
1. `rke-profile-hardened-1.8`
2. `rke-profile-permissive-1.8`

You would use the same config that you setup for a custom cluster to run this test. Plese reference this [section](#custom-cluster). It also important to note that the machines that you select has `sudo` capabilities. The tests utilize `sudo`, so this can cause issues if there is no `sudo` present on the machine.

These tests utilize Go build tags. Due to this, see the below examples on how to run the tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/rke1 --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestHardenedRKE1ClusterProvisioningTestSuite$"`

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