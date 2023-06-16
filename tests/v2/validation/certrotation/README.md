# Certificate Rotation

## Getting Started
Your GO suite should be set to `-run ^TestCertRotation$`. You can find specific tests by checking the test file you plan to run.
In your config file, set the following:
```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token",
  "userToken": "your_rancher_user_token",
  "clusterName": "cluster_to_run_tests_on",
  "insecure": true/optional,
  "cleanup": false/optional,
}
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
These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in rancher, you should create one first before running this test. 
Untested on k3s nor rke1.