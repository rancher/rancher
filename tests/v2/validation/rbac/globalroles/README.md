# Global Roles

## Pre-requisites

- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.

## Test Setup

Your GO suite should be set to `-run ^Test<TestSuite>$`

- To run the global_roles_test.go, set the GO suite to `-run ^TestGlobalRolesTestSuite$`
- To run the rbac_global_roles_test.go, set the GO suite to `-run ^TestRbacGlobalRolesTestSuite$`

In your config file, set the following:

```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True #optional
  cleanup: True #optional
  clusterName: "cluster_name"
```
