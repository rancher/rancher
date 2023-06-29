# Scale

## Getting Started
Your GO suite should be set to `-run ^Test<RKE1 if applicable>ScaleDownAndUp$`. You can find specific tests by checking the test file you plan to run.
In your config file, set the following:
```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token",
  "userToken": "your_rancher_user_token",
  "clusterName": "cluster_to_run_tests_on",
  "insecure": true/optional,
  "cleanup": false/optional,
}
```

Typically, a cluster with the following 3 pools is used for testing:
```yaml
{
  {
    ControlPlane: true,
    Quantity:     2,
  },
  {
    Etcd:     true,
    Quantity: 3,
  },
  {
    Worker:   true,
    Quantity: 2,
  },
}
  ```
These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in rancher, you should create one first before running this test. 

Scaling tests require that the given pools have unique, distinct roles and more than 1 node per pool. You can run a subset of the tests, but still need more than 1 node for the role you would like to run the test for. i.e. for `-run ^TestScaleDownAndUp/TestWorkerScaleDownAndUp$` you would need at least 1 pool with 2 or more dedicaated workers in it. The last node in the pool will be replaced. 


----

# RKE2 Custom Cluster Add Nodes

## Getting Started
Your GO suite should be set to `-run ^TestRKE2CustomClusterAddNodes$`. You can find specific tests by checking the test file you plan to run.
In your config file, set the following:
```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token",
  "clusterName": "cluster_to_run_tests_on",
}
"provisioningInput": {
        "nodeProviders": [
            "ec2"
        ]
    }
"awsEC2Configs": {
      "region": "us-east-2",
      "awsSecretAccessKey": "",
      "awsAccessKeyID": "",
      "awsEC2Config": [
          {
              "instanceType": "",
              "awsRegionAZ": "",
              "awsAMI": "",
              "awsAMI": "",
              "awsSecurityGroups": [
                  ""
              ],
              "awsSSHKeyName": "",
              "awsCICDInstanceTag": "rancher-validation",
              "awsIAMProfile": "",
              "awsUser": "",
              "volumeSize": ,
              "roles": [
                  "etcd",
                  "controlplane",
                  "worker"
              ]
          }
      ]
  }
"sshPath": {
        "sshPath": ""
    }
```
Typically, a custom cluster with 1-etcd, 1-controlplane and 1-worker node is used for testing this. These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in rancher, you should create one first before running this test. 
Also note that the cluster needs to be in fleet-default namespace and this test adds 1 controlplane node, 1 worker node and 2 etcd nodes.

