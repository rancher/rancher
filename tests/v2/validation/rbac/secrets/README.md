# RBAC Secrets

## Pre-requisites

- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.

## Test Setup

Your GO suite should be set to `-run ^Test<TestSuite>$`

- To run the rbac_opaque_secrets_test.go, set the GO suite to `-run ^TestRbacOpaqueSecretTestSuite$`
- To run the rbac_registry_secrets_test.go, set the GO suite to `-run ^TestRbacRegistrySecretTestSuite$`

In your config file for **TestRbacOpaqueSecretTestSuite**, set the following:

```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True #optional
  cleanup: True #optional
  clusterName: "cluster_name"
```

In your config file for **TestRbacRegistrySecretTestSuite**, set the following:

```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True #optional
  cleanup: True #optional
  clusterName: "cluster_name"
registryInput: 
  name: "registry_name"
  registryUsername: "registry_username" 
  registryPassword: "registry_password"
```
