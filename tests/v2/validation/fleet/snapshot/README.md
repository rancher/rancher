# Snapshot

For the snapshot + fleet tests, these tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running these tests. It is recommended to have a cluster configuration of 3 etcd, 2 controlplane, 3 workers.

Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).
Upgrade strategy and fleet's gitRepo are hard coded for this test, so no outside input is necessary. 

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

Additionally, S3 is a supported restore option. If you choose to use S3, then you must have it already enabled on the downstream cluster.

These tests utilize Go build tags. Due to this, see the below example on how to run the tests:

### Snapshot restore
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreWithFleetTestSuite/TestFleetThenSnapshotRestore"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/snapshot --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestSnapshotRestoreWithFleetTestSuite/TestSnapshotThenFleetRestore"`
