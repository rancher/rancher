# Verify rolebindings for users when RKE1 cluster is moved to a new workspace

## Pre-requisites

- Ensure you have an existing RKE1 cluster that the user has access to. If you do not have a downstream RKE1 cluster in Rancher, create one first before running this test.

## Test Setup

Your GO suite should be set to `-run ^TestMoveClusterToFleetWorkspaceTestSuite$`.

In your config file, set the following:

```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True #optional
  cleanup: True #optional
  clusterName: "RKE1_cluster_name"
```
