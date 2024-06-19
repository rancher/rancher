# LastLogin
Last login introduces a new label in user resource and LastLogin field in user attribute resource. Last login is now updated based on the user login time.

## Pre-requisites
- This test runs all the validations on a local cluster. This can be run on an rancher server without any downstream clusters.
- We need default admin password to be updated in the config below. Tests use the admin password to login to rancher as default admin.

## Getting Started
Your GO suite should be set to `-run ^Test<>TestSuite$`. To run the last_login_test.go, set the GO suite to `-run ^TestLastLoginTestSuite$` You can find specific tests by checking the test file you plan to run.
In your config file, set the following:

```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token",
  "insecure": true/optional,
  "cleanup": false/optional,
  "adminPassword": "<adminPassword>"
}
```

