# Snapshot

For the snapshot tests, these tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running these tests. It is recommended to have a cluster configuration of 3 etcd, 2 controlplane, 3 workers.

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
  upgradeKubernetesVersion: ""        # If left blank, the default version in Rancher will be used.
  snapshotRestore: "all"              # Options include none, kubernetesVersion, all. Option 'none' means that only the etcd will be restored.
  controlPlaneConcurrencyValue: "15%"
  workerConcurrencyValue: "20%"
  controlPlaneUnavailableValue: "1"
  workerUnavailableValue: "10%"
  recurringRestores: 1                # By default, this is set to 1 if this field is not included in the config.
  replaceRoles:                       # If selected, S3 must be properly configured on the cluster. This test is specific to S3 etcd snapshots.
    etcd: false
    controlplane: false
    worker: false
```

Additionally, S3 is a supported restore option. If you choose to use S3, then you must have it already enabled on the downstream cluster. Please note that the `TestSnapshotReplaceNodes` test is specifically for S3-enabled clusters.

These tests utilize Go build tags. Due to this, see the below example on how to run the tests:

### Snapshot restore
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreETCDOnlyTestSuite/TestSnapshotRestoreETCDOnly"`

### Snapshot restore with K8s upgrade
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreK8sUpgradeTestSuite/TestSnapshotRestoreK8sUpgrade"`

### Snapshot restore with upgrade strategy
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreUpgradeStrategyTestSuite/TestSnapshotRestoreUpgradeStrategy"`

### S3 snapshot restore
Note: This test is meant to be ran only with downstream clusters that have S3 backups enabled. If you are looking to utilize local backups, use one of the other tests instead.

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestS3SnapshotTestSuite/TestS3SnapshotRestore"`

### Snapshot restore - Windows clusters
Note: This test will only work with Windows nodes existing in the cluster. Run this test with a vSphere Windows node driver cluster or a custom cluster with a Windows node present.

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreWindowsTestSuite/TestSnapshotRestoreWindows"`

### Snapshot additional tests
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotAdditionalTestsTestSuite$"`