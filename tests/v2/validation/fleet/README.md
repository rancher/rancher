# Fleet Integrations

## Table of Contents
1. [Getting Started](#Getting-Started)
2. [Public Repo Tests](#Public-Repo)

## Getting Started
Your GO suite should be set to `-run ^TestFleet<enter_test_name_here>TestSuite$`. You can find the correct suite name in the below README links, or by checking the test file you plan to run.
In your config file, set the following:
```yaml
rancher:
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  cleanup: false
  insecure: true/optional
  cleanup: false/optional
  clusterName: "your-cluster-name"
```

## Public Repo

TestFleetPublicRepo/TestGitRepoDeployment

There is no additional config for the static test. Simply input the clusterName you'd like the fleet GitRepo + resources deployed to. 
The test will report the fleetVersion being used, and fail if gitRepo deployment fails or if resources via Steve don't come up properly. 

## Dynamic Test

TestFleetPublicRepo/TestDynamicGitRepoDeployment

Add the following to the root level of the go config with your own parameters to run a test using your own GitRepo:
```yaml
gitRepo:
  metadata:
    name: "dynamic-test-1"
    namespace: "fleet-default"
  spec:
    repo: https://github.com/rancher/fleet-examples
    branch: master
    paths:
    - simple
    imageScanCommit:
      authorName: ""
      authorEmail: ""
```
