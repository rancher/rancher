# PSA

## Getting Started
Your GO suite should be set to `-run ^TestRBACPSATestSuite$`.
In your config file, set the following:

```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token",
  "clusterName": "cluster_to_run_tests_on",
  "insecure": true/optional,
  "cleanup": false/optional,
}
```

