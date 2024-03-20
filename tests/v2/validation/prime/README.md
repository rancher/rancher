# Prime Configs

## Table of Contents
1. [Getting Started](#Getting-Started)

## Getting Started
Your GO suite should be set to `-run ^TestPrimeTestSuite$`. You can find any additional suite name(s) by checking the test file you plan to run.

In your config file, set the following:
```yaml
rancher: 
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  ...
prime:
  brand: "<name of brand>"
  isPrime: false  #boolean, default is false
  rancherVersion: "<version_or_commit_of_rancher>"
  registry: "<name of registry>
```

if isPrime is `true`, we will also check that the ui-brand is correctly set. For the `TestPrimeVersion` test case, your Rancher URL that is passed must use a secure certificate. If an insecure certificate is recognized, then the test will fail; this is expected.