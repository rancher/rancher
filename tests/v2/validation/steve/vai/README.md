# Steve / Vai

## Pre-requisites

## Test Setup

Your GO suite should be set to `-run ^TestVaiTestSuite$`.

In your config file, set the following:

```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  insecure: True # optional
  cleanup: True # optional
  clusterName: "local" # can just be checked against local
```
