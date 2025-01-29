# RBAC Workloads

## Pre-requisites

- Ensure you have an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, create one first before running this test.

## Test Setup

Your GO suite should be set to `-run ^Test<TestSuite>$`

- To run the rbac_daemonset_test.go, set the GO suite to `-run ^TestRbacDaemonsetTestSuite$`
- To run the rbac_deployment_test.go, set the GO suite to `-run ^TestRbacDeploymentTestSuite$`
- To run the rbac_statefulset_test.go, set the GO suite to `-run ^TestRbacStatefulSetTestSuite$`
- To run the rbac_cronjob_test.go, set the GO suite to `-run ^TestRbacCronJobTestSuite$`
- To run the rbac_job_test.go, set the GO suite to `-run ^TestRbacJobTestSuite$`

In your config file, set the following:

```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True #optional
  cleanup: True #optional
  clusterName: "cluster_name"
```
