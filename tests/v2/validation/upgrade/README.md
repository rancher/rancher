# Upgrade Configs

## Table of Contents
1. [Getting Started](#Getting-Started)
2. [Cloud Provider Migration](#cloud-provider-migration)

## Getting Started
Kubernetes and pre/post upgrade workload tests use the same single shared configuration as shown below. You can find the correct suite name below by checking the test file you plan to run.
In your config file, set the following, this will run each test in parallel both for Post/Pre and Kubernetes tests:

```yaml
upgradeInput:
  clusters:
    - name: ""                      # Cluster name that is already provisioned in Rancher
      psact: ""                     # Values are rancher-privileged, rancher-restricted or rancher-baseline
      enabledFeatures:
        chart: false                # Boolean, pre/post upgrade checks, default is false
        ingress: false              # Boolean, pre/post upgrade checks, default is false
      provisioningInput:            # See the [Hosted Provider Provisioning](hosted/README.md)
        rke1KubernetesVersion: [""]
        rke2KubernetesVersion: [""]
        k3sKubernetesVersion: [""]              
```
Note: To see the `provisioningInput` in further detail, please review over the [Provisioning README](../provisioning/README.md).
See below how to run the test:

### Kubernetes Upgrade
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/upgrade --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestKubernetesUpgradeTestSuite/TestUpgradeKubernetes"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/upgrade --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestWindowsKubernetesUpgradeTestSuite/TestUpgradeWindowsKubernetes"`

## Cloud Provider Migration
Migrates a cluster's cloud provider from in-tree to out-of-tree

### Current Support:
* AWS
  * RKE1
  * RKE2

### Pre-Requisites in the provided cluster
* in-tree provider is enabled
* out-of-tree provider is supported with your selected kubernetes version

### Running the test
```yaml
rancher:
  host: <your_host>
  adminToken: <your_token>
  insecure: true/false
  cleanup: false/true
  clusterName: "<your_cluster_name>"
```

**note** that no `upgradeInput` is required. See below how to run each of the tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/upgrade --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCloudProviderMigrationTestSuite/TestAWS"`


## Cloud Provider Upgrade
Upgrades the chart version of cloud provider (CPI/CSI)

### Current Support:
* Vsphere
  * RKE1

### Pre-Requisites on the cluster
* cluster should have upgradeable CPI/CSI charts installed. You can do this via automation in provisioning/rke1 with the following option, chartUpgrade, which will install a version of the chart (latest - 1) that can later be upgraded to the latest version. 
```yaml
chartUpgrade:
  isUpgradable: true
```

### Running the test
```yaml
rancher:
  host: <your_host>
  adminToken: <your_token>
  insecure: true/false
  cleanup: false/true
  clusterName: "<your_cluster_name>"
vmwarevsphereCredentials:
  ...
vmwarevsphereConfig: 
  ...
```
See below how to run each of the tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/upgrade --junitfile results.xml -- -timeout=60m -tags=validation -v -run ^TestCloudProviderVersionUpgradeSuite$"`