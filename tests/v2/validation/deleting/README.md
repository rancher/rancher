# Deleting

## Getting Started
You can find specific tests by checking the test file you plan to run. An example is `-run ^ TestClusterDeleteTestSuite/TestDeletingRKE2K3SCluster$`
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
