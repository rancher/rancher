# Certificate Rotation

For certificate rotation, the test has only been tested on RKE2. That does not mean RKE1 or K3S are unsupported, they just have not been tested. These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running these tests.

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

Typically, a cluster with the following 3 pools is used for testing:
```yaml
{
  {
    ControlPlane: true,
    Quantity:     1,
  },
  {
    Etcd:     true,
    Quantity: 1,
  },
  {
    Worker:   true,
    Quantity: 1,
  },
}
```

These tests utilize Go build tags. Due to this, see the below example on how to run the tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/certrotation --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCertRotation/TestCertRotation"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.