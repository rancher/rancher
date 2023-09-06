# Upgrade Configs

## Table of Contents
1. [Getting Started](#Getting-Started)

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