# Deleting

For deleting clusters, the following cluster types are supported for both node drivers and custom clusters: RKE1, RKE2, K3S. These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running these tests.

Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Getting Started](#Getting-Started)

## Getting Started
In your config file, set the following:
```yaml
rancher:
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  clusterName: "cluster_to_run_tests_on"
  insecure: true/optional
  cleanup: false/optional
```

These tests utilize Go build tags. Due to this, see the below examples on how to run the tests:

### RKE1 | RKE2 | K3S
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/deleting --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestClusterDeleteTestSuite/TestDeletingCluster"`

### Delete Init Machine Suite (rke2/k3s)
Automated check to validate [this issue](https://github.com/rancher/rancher/issues/42709), where the "init node" machine for v2 prov clusters would hang in deletion state and fail to be removed.

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/deleting --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestDeleteInitMachineTestSuite/TestDeleteInitMachine"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.