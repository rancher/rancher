## Cluster template Provisioning Config

For your config, you will need everything in the with and at least one [Cloud Credential](../../provisioning/rke1/README.md#cloud-credentials). 

Your GO test_package should be set to `provisioning/clustertemplates`.
Your GO suite should be set to `-run ^TestClusterTemplateRKE1ProvisioningTestSuite$`.
Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Note: 
Test uses node drivers provisioning. This is an example config. we can make use of other node drivers like, Linode, DO etc. For reference, please check [RKE1 Node Drivers Config](../../provisioning/rke1/README.md#nodetemplateconfigs)

## Table of Contents
1. [Getting Started](#Getting-Started)

## Getting Started
See below a sample config file to run this test:
```yaml
rancher:                                   
  host: ""                                  #required
  adminToken: ""                            #required
  clusterName: ""                           

provisioningInput:
  RKE1KubernetesVersions: ["<Your preferred Version>"]    #optional. if empty, latest version is considered.
  cni: ["calico"]                                  #optional, if empty, calico is considered.
  providers: ["aws"]                           #required (at least 1)                    

amazonec2Config:
    accessKey: ""
    secretKey: ""
    ami: "ami-0e6577a75723c81f8"
    blockDurationMinutes: "0"
    encryptEbsVolume: false
    endpoint: ""
    httpEndpoint: "enabled"
    httpTokens: "optional"
    iamInstanceProfile: ""
    insecureTransport: false
    instanceType: "t3.xlarge"
    keypairName: ""
    kmsKey: ""
    monitoring: false
    privateAddressOnly: false
    region: "us-east-2"
    requestSpotInstance: false
    retries: "5"
    rootSize: "100"
    securityGroup: []
    securityGroupReadonly: false
    sessionToken: ""
    spotPrice: "0.50"
    sshKeyContents: ""
    sshUser: "ubuntu"
    subnetId: ""
    tags: ""
    type: "amazonec2Config"
    useEbsOptimizedInstance: false
    usePrivateAddress: false
    userdata: ""
    volumeType: "gp2"
    vpcId: "vpc-bfccf4d7"
    zone: "a"                      
```

These tests utilize Go build tags. Due to this, see the below example on how to run the test:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/clustertemplates --junitfile results.xml -- -timeout=120m -tags=validation -v -run "TestClusterTemplateRKE1ProvisioningTestSuite/TestProvisioningRKE1ClusterWithClusterTemplate"`