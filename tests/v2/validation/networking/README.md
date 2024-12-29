# Networking

## Getting Started

Your GO suite should be set to `-run ^Test<>TestSuite$`. For example to run the network_policy_test.go, set the GO suite
to `-run ^TestNetworkPolicyTestSuite$` You can find specific tests by checking the test file you plan to run.
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

to `-run ^TestPortTestSuite/TestNodePort$` the Project Network Isolation should be enable on the cluster.
to `-run ^TestPortTestSuite/TestHostPort$` the Project Network Isolation should be enable on the cluster.

**NOTE** These tests most run on a server with private networking setup as they rely on being able to SSH into servers
