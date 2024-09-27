# Configmaps RBAC Suite(public API)
This repository contains Golang automation tests for the ConfigmapsRBACSuite functionality by Public API(Wrangler). These automation tests contains both success and error test cases to make sure the functionality is working as expected.
CRUD operations are performed by the users and a deployment is created with a configmap as env var and volume.

## Pre-requisites
- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.

## Test Setup
Your GO suite should be set to `-run ^TestConfigmapsRBACTestSuite$`. You can find specific tests by checking the test file you plan to run.

In your config file, set the following:
```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True  #optional
  cleanup: True #optional
  clusterName: "cluster_name"
```