# Charts Configs

You can find the correct suite name in the below by checking the test file you plan to run.
In your config file, set the following:

```json
"rancher": { 
  "host": "<rancher-server-host>",
  "adminToken": "<rancher-admin-token>",
  "insecure": true/optional,
  "cleanup": false/optional,
  "clusterName": "<cluster-to-run-test>"
}
```

From there, please use one of the following links to check charts tests:

1. [Monitoring Chart](monitoring_test.go)
2. [Gatekeeper Chart](gatekeeper_test.go)
3. [Istio Chart](istio_test.go)
