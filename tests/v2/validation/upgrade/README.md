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
 - name: "" # String, cluster name
    kubernetesVersionToUpgrade: "" # String, kubernetes version to upgrade
    enabledFeatures:
        chart: false # Boolean, pre/post upgrade checks, default is false
        ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
```
 - If you want to run Post/Pre Upgrade tests against all the clusters except local, you can add **WorkloadUpgradeAllClusters** environment flag instead above. 
 - If you want to run Kubernetes Upgrade tests against all the clusters except local, you can add **KubernetesUpgradeAllClusters** environment flag instead above.

For Kubernetes upgrade test, *"latest"* string would pick the latest possible Kubernetes version from the version pool. Empty string value *""* for the version to upgrade field, skips the Kubernetes Upgrade Test.

Please use one of the following links to check upgrade tests:

1. [Kubernetes Upgrade](kubernetes_test.go)
2. [Pre/Post Upgrade Workload](workload_test.go)


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

**note** that no upgradeInput is required

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/upgrade --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCloudProviderMigrationTestSuite/TestAWS"`