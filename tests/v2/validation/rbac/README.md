# Rbac

## Getting Started
Your GO suite should be set to `-run ^Test<>TestSuite$`. For example to run the rbac_additional_test.go, set the GO suite to `-run ^TestRBACAdditionalTestSuite$` You can find specific tests by checking the test file you plan to run.
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

For the rbac_additional_test.go run, we need the following paramters to be passed in the config file as we create an rke1 cluster
Please use the following links to continue adding to your config for provisioning tests:
 [Define your test](../provisioning/rke1/README.md#provisioning-input)
(#Provisioning-Input)
 [Configure providers to use for Node Driver Clusters](../provisioning/rke1/README.md#NodeTemplateConfigs)