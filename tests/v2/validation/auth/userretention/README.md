# Userretention

The User Retention feature in Rancher allows administrators to manage inactive users by automatically disabling and deleting them based on predefined settings and a user retention cron job. This functionality is implemented based on the user's last login timestamp. The tests included in this package focus on validating the behavior of the User Retention feature, including updating retention settings and verifying the proper disabling and deletion of inactive users.

## Pre-requisites

- The tests are designed to be executed on a local cluster. They can be run on a Rancher server without any downstream clusters.
- Update the default admin password in the configuration file. The tests rely on the admin password to authenticate and perform actions as the default admin user in Rancher.

## Getting Started

Your GO suite should be set to `-run ^Test<>TestSuite$`. 

To run the userretention_test.go, set the GO suite to `-run ^TestUserRetentionSettingsSuite$` You can find specific tests by checking the test file you plan to run.
To run the userretention_disable_user_test.go, set the GO suite to `-run ^TestURDisableUserSuite$` You can find specific tests by checking the test file you plan to run.
To run the userretention_delete_user_test.go, set the GO suite to `-run ^TestURDeleteUserSuite$` You can find specific tests by checking the test file you plan to run.

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