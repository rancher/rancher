# ClusterRoleTemplateBinding(v1)
This repository contains Golang automation tests for the ClusterRoleTemplateBinding functionality by SteveAPI(V1) and kubectl. This is part of Public API functionality to generate Custom Resource Definitions (CRD's) for ClusterRoleTemplateBinding. ClusterRoleTemplateBindings is allowing users to perform all the CRUD operations by using SteveAPI(V1) OR kubectl. These automation tests contains both success and error test cases to make sure the functionality is working as expected.

## Pre-requisites
- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.

## Test Setup
Your GO suite should be set to `-run ^TestCRTBGenTestSuite$`. You can find specific tests by checking the test file you plan to run.

In your config file, set the following:
```
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True
  cleanup: True
  clusterName: "downstream_cluster_name"
```