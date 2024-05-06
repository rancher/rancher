## Hosted Provider Provisioning Configs

For your config, you will need everything in the [Prerequisites](../README.md) section on the previous readme along with and at least one [Cloud Credential](#cloud-credentials) and [Hosted Provider Config](#hosted-provider-configs). 

Your GO test_package should be set to `provisioning/hosted`.
Your GO suite should be set to `-run ^TestHostedClusterProvisioningTestSuite$`.
Please see below for more details for your config. Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Prerequisites](../README.md)
2. [Cloud Credential](#cloud-credentials)
3. [Hosted Provider Config](#hosted-provider-configs)
4. [Back to general provisioning](../README.md)

Below are example configs needed for the different hosted providers including GKE, AKS, and EKS. In order to run these tests, the [cloud credentials](#cloud-credentials) are also needed. GKE (googleCredentials), AKS(azureCredentials), and EKS(awsCredentials)

## Cloud Credentials

### AWS
```yaml
awsCredentials:
  secretKey: "",
  accessKey: "",
  defaultRegion: ""
```
### Azure
```yaml
azureCredentials:
  clientId: "",
  clientSecret: "",
  subscriptionId: "",
  environment: "AzurePublicCloud"
```
### Google
```yaml
googleCredentials:
  authEncodedJson: |-
    {
      "type": "",
      "project_id": "",
      "private_key_id": "",
      "private_key": "",
      "client_email": "",
      "client_id": "",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": ""
    }
```

## Hosted Provider Configs

### EKS Cluster Config
```yaml
eksClusterConfig:
  imported: false
  kmsKey: ""
  kubernetesVersion: "1.26"
  loggingTypes: []
  nodeGroups:
  - desiredSize: 3
    diskSize: 50
    ec2SshKey: ""
    gpu: false
    imageId: ""
    instanceType: t3.medium
    labels: {}
    maxSize: 10
    minSize: 1
    nodeRole: ""
    nodegroupName: ""
    requestSpotInstances: false
    resourceTags: {}
    spotInstanceTypes: []
    subnets: []
    tags: {}
    userData: ""
    version: "1.26"
  privateAccess: false
  publicAccess: true
  publicAccessSources: []
  region: us-east-2
  secretsEncryption: false
  securityGroups:
  -
  serviceRole: ""
  subnets:
  -
  -
  tags: {}
```

These tests utilize Go build tags. Due to this, see the below example on how to run the test: 

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/hosted --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestHostedEKSClusterProvisioningTestSuite/TestProvisioningHostedEKS"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.

### AKS Cluster Config
```yaml
aksClusterConfig:
  dnsPrefix: ""
  kubernetesVersion: "1.26.6"
  linuxAdminUsername: "azureuser"
  loadBalancerSku: "Standard"
  networkPlugin: "kubenet"
  nodePools:
  - availabilityZones:
    - "1"
    - "2"
    - "3"
    enableAutoScaling: false
    maxPods: 110
    maxCount: 10
    minCount: 3
    mode: "System"
    name: "agentpool"
    nodeCount: 3
    osDiskSizeGB: 128
    osDiskType: "Managed"
    osType: "Linux"
    vmSize: "Standard_DS2_v2"
  resourceGroup: ""
  resourceLocation: ""
  tags: {}
```

These tests utilize Go build tags. Due to this, see the below example on how to run the test: 

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/hosted --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestHostedAKSClusterProvisioningTestSuite/TestProvisioningHostedAKS"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.

### GKE Cluster Config
Note that the following are required and should be updated:
* kubernetesVersion
* location
* locations
* zone
* labels
* nodePools->name
* nodePools->labels
* nodePools->config->imageType (currently set to COS_CONTAINERD for use with 1.23+)

```yaml
gkeClusterConfig:
  clusterAddons:
    horizontalPodAutoscaling: true
    httpLoadBalancing: true
    networkPolicyConfig: false
  clusterIpv4Cidr: ""
  enableKubernetesAlpha: false
  horizontalPodAutoscaling: true
  httpLoadBalancing: true
  ipAllocationPolicy:
    clusterIpv4Cidr: ""
    clusterIpv4CidrBlock: null
    clusterSecondaryRangeName: null
    createSubnetwork: false
    nodeIpv4CidrBlock: null
    servicesIpv4CidrBlock: null
    servicesSecondaryRangeName: null
    subnetworkName: null
    useIpAliases: true
  kubernetesVersion: 1.26.8-gke.200
  labels: {}
  locations: []
  loggingService: logging.googleapis.com/kubernetes
  maintenanceWindow: ""
  masterAuthorizedNetworks:
    enabled: false
  monitoringService: monitoring.googleapis.com/kubernetes
  network: default
  networkPolicyConfig: false
  networkPolicyEnabled: false
  nodePools:
  - autoscaling:
      enabled: false
      maxNodeCount: null
      minNodeCount: null
    config:
      diskSizeGb: 100
      diskType: pd-standard
      imageType: COS_CONTAINERD
      labels: {}
      localSsdCount: 0
      machineType: n1-standard-2
      oauthScopes:
      - https://www.googleapis.com/auth/devstorage.read_only
      - https://www.googleapis.com/auth/logging.write
      - https://www.googleapis.com/auth/monitoring
      - https://www.googleapis.com/auth/servicecontrol
      - https://www.googleapis.com/auth/service.management.readonly
      - https://www.googleapis.com/auth/trace.append
      preemptible: false
      tags: null
      taints: null
    initialNodeCount: 3
    isNew: true
    management:
      autoRepair: true
      autoUpgrade: true
    maxPodsConstraint: 110
    name: markus-automation
  privateClusterConfig:
    enablePrivateEndpoint: false
    enablePrivateNodes: false
    masterIpv4CidrBlock: null
  projectID: ""
  region: ""
  subnetwork: default
  zone: us-central1-c
```

These tests utilize Go build tags. Due to this, see the below example on how to run the test: 

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/hosted --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestHostedGKEClusterProvisioningTestSuite/TestProvisioningHostedGKE"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.