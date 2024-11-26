# Rbac

## Getting Started
Your GO suite should be set to `-run ^Test<>TestSuite$`. For example to run the cluster_role_test.go, set the GO suite to `-run ^TestClusterRoleTestSuite$` You can find specific tests by checking the test file you plan to run.
Config needed for each of the suites cluster_role_test.go and project_role_test.go require the following config:

```yaml
rancher:
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True #optional
  cleanup: True #optional
  clusterName: "downstream_cluster_name"
```

For more info, please use the following links to continue adding to your config for provisioning tests:
 [Define your test](../provisioning/rke1/README.md#provisioning-input)
(#Provisioning-Input)


