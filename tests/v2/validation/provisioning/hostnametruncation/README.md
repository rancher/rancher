# Hostname Truncation

For your config, you can directly reference the [RKE2 README](../rke2/README.md), specifically, for the prequisites, provisioning input, cloud credentials and machine RKE2 configuration.

Your GO test_package should be set to `hostnametruncation`.
Your GO suite should be set to `-run ^TestHostnameTruncationTestSuite$`.
Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Getting Started](#Getting-Started)

## Getting Started
See below a sample config file to run this test:
```yaml
rancher:
  host: ""
  adminToken: ""
  clusterName: ""
provisioningInput:
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
  rke2KubernetesVersion: ["v1.27.6+rke2r1"]
  cni: ["calico"]
  providers: ["linode"]
linodeCredentials:
   token: ""
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

These tests utilize Go build tags. Due to this, see the below example on how to run the test:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/provisioning/hostnametruncation --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestHostnameTruncationTestSuite/TestProvisioningRKE2ClusterTruncation"`