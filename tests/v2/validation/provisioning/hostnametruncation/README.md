# Hostname Truncation

For your config, you can directly reference the [RKE2 README](../rke2/README.md), specifically, for the prequisites, provisioning input, cloud credentials and machine RKE2 configuration.

Your GO test_package should be set to `hostnametruncation`.
Your GO suite should be set to `-run ^TestProvisioningRKE2ClusterTruncation$`.
Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

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
  machinePools:                             
  - nodeRoles:                              #required (at least 1)
      etcd: true                            #required (at least 1 controlplane & etcd & worker)                            
      quantity: 1
  - nodeRoles:
      controlplane: true
      quantity: 1
  - nodeRoles:
      worker: true
      quantity: 1
  rke2KubernetesVersion: ["v1.27.6+rke2r1"] #required (at least 1)
  cni: ["calico"]                           #required (at least 1)
  providers: ["aws"]                        #required (at least 1)

awsCredentials:                            
  secretKey: "",                            #required                                               
  accessKey: "",                            #required                          
  defaultRegion: ""                         #required                      

awsMachineConfig:                                   
  region: "us-east-2"                       
  ami: ""                                   #required                      
  instanceType: "t3a.medium"                #required                
  sshUser: "ubuntu"                         #required                        
  vpcId: ""                                 #required                               
  volumeType: "gp2"                         #required                        
  zone: "a"                                 #required                          
  retries: "5"                              #required
  rootSize: "60"                            #required
  securityGroup: [""]                       #required                         
```

These tests utilize Go build tags. Due to this, see the below example on how to run the test:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/hostnametruncation --junitfile results.xml -- -timeout=120m -tags=validation -v -run "TestProvisioningHostnameTruncationTestSuite/TestProvisioningRKE2ClusterTruncation"`