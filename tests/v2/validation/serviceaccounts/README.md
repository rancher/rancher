For the serviceaccount tests, these tests are designed to accept an existing RKE1 cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running these tests. It is recommended to have a cluster configuration of 3 etcd, 2 controlplane, 3 workers.

For the sa_test.go run, an RKE1 cluster is required.

In your config file, set the following:
```yaml
rancher:
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  clusterName: "cluster_to_run_tests_on"
  insecure: true/optional
  cleanup: true/optional
```