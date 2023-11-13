# Snapshot

For the snapshot tests, these tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running these tests.

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
snapshotInput:
  upgradeKubernetesVersion: "v1.26.9+rke2r1" # If left blank, the default version in Rancher will be used.
  snapshotRestore: "all" # Options include none, kubernetesVersion, all. Option 'none' means that only the etcd will be restored.
  controlPlaneConcurrencyValue: "15%"
  workerConcurrencyValue: "20%"
  controlPlaneUnavailableValue: "1"
  workerUnavailableValue: "10%"
  recurringRestores: 1  # By default, this is set to 1 if this field is not included in the config.
```

Additionally, S3 is a supported restore option. If you choose to use S3, then you must have it already enabled on the downstream cluster.

These tests utilize Go build tags. Due to this, see the below example on how to run the tests:

### Snapshot restore
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreTestSuite/TestSnapshotRestore"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreTestSuite/TestSnapshotRestoreDynamicInput"`

### Snapshot restore with K8s upgrade
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreK8sUpgradeTestSuite/TestSnapshotRestoreK8sUpgrade"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreK8sUpgradeTestSuite/TestSnapshotRestoreK8sUpgradeDynamicInput"`

### Sanpshot restore with upgrade strategy
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreUpgradeStrategyTestSuite/TestSnapshotRestoreUpgradeStrategy"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreUpgradeStrategyTestSuite/TestSnapshotRestoreUpgradeStrategyDynamicInput"`