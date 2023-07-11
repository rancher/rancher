# v2prov Integration Tests

These are a set of provisioning-v2 integration tests.

These tests are designed to be run on a single-node cluster (due to the usage of HostPath volumes), and perform e2e validations of v2prov + CAPR using systemd-node.

## Test Categories

There are three major categories of tests that are contained within this test suite.

### General

General tests, for example, ensuring system-agent version is as expected.

### Provisioning_MP

Provisioning tests encompass creation and deletion of v2prov clusters. MP stands for machine-provisioned, and will use nodepools to manage the test infrastructure.

### Provisioning_Custom

Provisioning tests encompass creation and deletion of v2prov clusters. Custom will use a "Custom" cluster and the framework manually creates systemd-node pods.

### Operation_MP

Operation tests are day 2 operation tests like encryption key rotation, certificate rotation, and etcd snapshot creation/restore. MP stands for machine-provisioned, and will use nodepools to manage the test infrastructure.

### Operation_Custom

Operation tests are day 2 operation tests like encryption key rotation, certificate rotation, and etcd snapshot creation/restore. Custom will use a "Custom" cluster and the framework manually creates systemd-node pods.

## Test Naming Format

Within the `tests` folder, there are mutliple types of tests. They notably all have a format like to ensure we do not clog up individual pipeline stages as they are resource intensive.

`Test_<Category>_<TestName>`

`<Category>` corresponds to the categories listed above.

## Running Locally

You can run these tests locally on a machine that can support running systemd within containers. 

If you invoke `make provisioning-tests`, it will run all of the provisioning/general tests.

You can customize the tests you run through the use of the environment variables: `V2PROV_TEST_RUN_REGEX` and `V2PROV_TEST_DIST`. 

`V2PROV_TEST_DIST` can be either `k3s` (default) or `rke2`
`V2PROV_TEST_RUN_REGEX` is a regex string indicating the pattern of tests to match. You can selectively run tests by setting this, for example:

```
V2PROV_TEST_RUN_REGEX=Test_Operation_Custom_EncryptionKeyRotation V2PROV_TEST_DIST=rke2 make provisioning-tests
```

would specifically run the `Test_Operation_Custom_EncryptionKeyRotation` test with RKE2.
