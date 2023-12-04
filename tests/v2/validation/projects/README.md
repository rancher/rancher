# Projects

## Pre-requisites

- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.

## Test Setup

Your GO suite should be set to `-run ^Test<TestSuite>$`. For example to run the rbac_terminating_project_test.go, set the GO suite to `-run ^TestRbacTerminatingProjectTestSuite$` You can find specific tests by checking the test file you plan to run.

In your config file, set the following:

```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True
  cleanup: True
  clusterName: "downstream_cluster_name"
```
