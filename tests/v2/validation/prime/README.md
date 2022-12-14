# Prime Configs

## Getting Started
Your GO suite should be set to `-run ^TestPrimeVersionTestSuite$`. You can find any additional suite name(s) by checking the test file you plan to run.

In your config file, set the following:
```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token"
  ...,
}
"prime": {
  "rancherVersion": "<version_or_commit_of_rancher>",
  "isPrime": false, //boolean, default is false
}
```

if isPrime is `true`, we will also check that the ui-brand is correctly set. 