# CLI

For CLI tests, the local cluster is used to perform each of the tests. It is important to note that you must have the Rancher CLI already installed and configured on your client machine before running this test.

Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Getting Started](#Getting-Started)

## Getting Started
In your config file, set the following:
```yaml
rancher:
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: true/optional
  rancherCLI: true
```

These tests utilize Go build tags. Due to this, see the below examples on how to run the tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/cli --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestCLITestSuite$"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.