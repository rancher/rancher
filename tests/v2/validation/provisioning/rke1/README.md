# RKE1 Provisioning Configs

For your config, you will need everything in the [Prerequisites](../README.md) section on the previous readme, [Define your test](#Provisioning-Input), and at least one [Node Driver Cluster Template](#NodeTemplateConfigs) or [Custom Cluster Template](#Custom-Cluster), which should match what you have specified in `provisioningInput`. 

Your GO test_package should be set to `provisioning/rke1`.
Your GO suite should be set to `-run ^TestRKE1ProvisioningTestSuite$`.
Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Prerequisites](../README.md)
2. [Define your test](#Provisioning-Input)
3. [Configure providers to use for Node Driver Clusters](#NodeTemplateConfigs)
4. [Configuring Custom Clusters](#Custom-Cluster)
5. [Advanced Cluster Settings](#advanced-settings)
6. [Back to general provisioning](../README.md)

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
      quantity: 2
  flags:
    desiredflags: "Long" #These flags are for running TestProvisioningRKE1Cluster or TestProvisioningRKE1CustomCluster it is not needed for the dynamic tests. Long will run the full table, where as short will run the short version of this test.
  rke1KubernetesVersion: ["v1.26.8-rancher1-1"]
  cni: ["calico"]
  providers: ["linode", "aws", "do", "harvester"]
  nodeProviders: ["ec2"]
  psact: ""
```

## NodeTemplateConfigs
RKE1 specifically needs a node template config to run properly. These are the inputs needed for the different node providers.

### AWS
```yaml
  awsNodeTemplate:
    accessKey: ""
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
    secretKey: ""
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
azureNodeTemplate:
  availabilitySet: "docker-machine"
  clientId: ""
  clientSecret: ""
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
  tenantId: ""
  type: "azureConfig"
  updateDomainCount: "5"
  vnet: "docker-machine-vnet"
```

### Harvester
```yaml
harvesterNodeTemplate":
  cloudConfig: ""
  clusterId: ""
  clusterType: ""
  cpuCount: "2"
  diskBus: "virtio"
  diskSize: "40"
  imageName: "default/image-gchq8"
  keyPairName: ""
  kubeconfigContent: ""
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
linodeNodeTemplate:
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

### Vsphere
```yaml
vmwarevsphereNodeTemplate:
  cfgparam: ["disk.enableUUID=TRUE"]
  cloneFrom: ""
  cloudinit: "#cloud-config\n\n"
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