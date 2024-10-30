# KDM

For the `kdm` test `TestChangeKDMurl`, any cluster with rancher installed works because the test just needs access to `settings`
object and the API endpoint `/v1-rke2-release/releases`

Example `config.yaml`

```yaml
rancher:
  host: "rancher_server_url"
  adminToken: "bearer_api_token"
  clusterName: "local"
  insecure: true
```
