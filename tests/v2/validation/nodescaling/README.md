# Node Scaling

These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running this test. 

## Table of Contents
1. [Getting Started](#Getting-Started)
2. [Replacing Nodes](#Node-Replacing)
3. [Scaling Existing Node Pools](#Scaling-Existing-Node-Pools)

## Getting Started
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

## Node Replacing
Node replacement tests require that the given pools have unique, distinct roles and more than 1 node per pool. You can run a subset of the tests, but still need more than 1 node for the role you would like to run the test for. i.e. for `-run ^TestScaleDownAndUp/TestWorkerScaleDownAndUp$` you would need at least 1 pool with 2 or more dedicaated workers in it. The last node in the pool will be replaced. 

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

See below some examples on how to run the node replacment tests:

### RKE1
`-run ^TestRKE1NodeScaleDownAndUp/TestEtcdScaleDownAndUp$`
`-run ^TestRKE1NodeScaleDownAndUp/TestControlPlaneScaleDownAndUp$`
`-run ^TestRKE1NodeScaleDownAndUp/TestWorkerScaleDownAndUp$`

### RKE2 | K3S
`-run ^TestNodeScaleDownAndUp/TestEtcdScaleDownAndUp$`
`-run ^TestNodeScaleDownAndUp/TestControlPlaneScaleDownAndUp$`
`-run ^TestNodeScaleDownAndUp/TestWorkerScaleDownAndUp$`

## Scaling Existing Node Pools
Similar to the `provisioning` tests, the node scaling tests have static test cases as well as dynamicInput tests you can specify. In order to run the dynamicInput tests, you will need to define the `scalingInput` block in your config file. This block defines the quantity you would like the pool to be scaled up/down to. See an example below:
```json
"scalingInput": {
    "nodePools": [ 
      {
        "nodeRoles": {
          "worker": true,
          "quantity": 2
        }
      },
    ],
    "machinePools": [
      {
        "nodeRoles": {
          "etcd": true,
          "quantity": 1
        }
      }
    ]
    "aksNodePool": [
      {
        "nodeCount": 1,
      }
    ],
    "eksNodePool": [
      {
        "desiredSize": 1,
      }
    ],
    "gkeNodePool": [
      {
        "initialNodeCount": 1,
      }
    ]
  }
```
NOTE: When scaling AKS and EKS, you will need to make sure that the `maxCount` and `maxSize` parameter is greater than the desired scale amount, respectively. For example, if you wish to have 6 total EKS nodes, then the `maxSize` parameter needs to be at least 7. This is not a limitation of the automation, but rather how EkS specifically handles nodegroups.

See below some examples on how to run the node scaling tests:

### RKE1
`-run ^TestRKE1NodeScalingTestSuite/TestScalingRKE1NodePools$`
`-run ^TestRKE1NodeScalingTestSuite/TestScalingRKE1NodePoolsDynamicInput$`

### RKE2
`-run ^TestRKE2NodeScalingTestSuite/TestScalingRKE2NodePools$`
`-run ^TestRKE2NodeScalingTestSuite/TestScalingRKE2NodePoolsDynamicInput$`

### K3S
`-run ^TestK3SNodeScalingTestSuite/TestScalingK3SNodePools$`
`-run ^TestK3SNodeScalingTestSuite/TestScalingK3SNodePoolsDynamicInput$`

### AKS
`-run ^TestAKSNodeScalingTestSuite/TestScalingAKSNodePools$`
`-run ^TestAKSNodeScalingTestSuite/TestScalingAKSNodePoolsDynamicInput$`

### EKS
`-run ^TestEKSNodeScalingTestSuite/TestScalingEKSNodePools$`
`-run ^TestEKSNodeScalingTestSuite/TestScalingEKSNodePoolsDynamicInput$`

### GKE
`-run ^TestGKENodeScalingTestSuite/TestScalingGKENodePools$`
`-run ^TestGKENodeScalingTestSuite/TestScalingGKENodePoolsDynamicInput$`