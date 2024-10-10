# Snapshot

For the snapshot tests, these tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running these tests. It is recommended to have a cluster configuration of 3 etcd, 2 controlplane, 3 workers.

Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Getting Started](#Getting-Started)
2. [Release Testing](#Release-Testing)
3. [Local Qase Reporting](#Local-Qase-Reporting)

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

Additionally, S3 is a supported restore option. If you choose to use S3, then you must have it already enabled on the downstream cluster.

These tests utilize Go build tags. Due to this, see the below example on how to run the tests:

### Snapshot restore
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreETCDOnlyTestSuite/TestSnapshotRestoreETCDOnly"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreETCDOnlyTestSuite/TestSnapshotRestoreETCDOnlyDynamicInput"`

### Snapshot restore with K8s upgrade
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreK8sUpgradeTestSuite/TestSnapshotRestoreK8sUpgrade"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreK8sUpgradeTestSuite/TestSnapshotRestoreK8sUpgradeDynamicInput"`

### Sanpshot restore with upgrade strategy
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreUpgradeStrategyTestSuite/TestSnapshotRestoreUpgradeStrategy"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreUpgradeStrategyTestSuite/TestSnapshotRestoreUpgradeStrategyDynamicInput"`

### Sanpshot restore - Windows clusters
Note: This test will only work with Windows nodes existing in the cluster. Run this test with a vSphere Windows node driver cluster or a custom cluster with a Windows node present.

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreWindowsTestSuite/TestSnapshotRestoreWindows"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreWindowsTestSuite/TestSnapshotRestoreWindowsDynamicInput"`

### Sanpshot additional tests
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotAdditionalTestsTestSuite$"`

## Release Testing
The release testing includes all of the tests that are ran during release testing time. Each test will first provision a cluster and then run the specific test. See an example config below:

```yaml
rancher:
  host: ""
  adminToken: ""
  cleanup: false
  clusterName: ""
  insecure: true
provisioningInput:
  rke1KubernetesVersion: [""]
  rke2KubernetesVersion: [""]
  k3sKubernetesVersion: [""]
  cni: ["calico"]
  providers: ["linode"]
linodeCredentials:
   token: ""
linodeConfig:
  authorizedUsers: ""
  createPrivateIp: true
  dockerPort: "2376"
  image: "linode/ubuntu22.04"
  instanceType: "g6-dedicated-8"
  label: ""
  region: "us-west"
  rootPass: ""
  sshPort: "22"
  sshUser: "root"
  stackscript: ""
  stackscriptData: ""
  swapSize: "512"
  tags: ""
  token: ""
  type: "linodeConfig"
  uaPrefix: "Rancher"
linodeMachineConfigs:
  linodeMachineConfig:
  - roles: ["etcd", "controlplane", "worker"]
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

To run, use the following command:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=240m -tags=validation -v -run "TestSnapshotRestoreReleaseTestingTestSuite$"`

## Local Qase Reporting
If you are planning to report to Qase locally, then you will need to have the following done:
1. The `rancher` block in your config file must have `localQaseReporting: true`.
2. The working shell session must have the following two environmental variables set:
     - `QASE_AUTOMATION_TOKEN=""`
     - `QASE_TEST_RUN_ID=""`
3. Append `./reporter` to the end of the `gotestsum` command. See an example below::
     - `gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml --jsonfile results.json -- -timeout=300m -v -run "TestSnapshotRestoreReleaseTestingTestSuite$";/path/to/rancher/rancher/reporter`