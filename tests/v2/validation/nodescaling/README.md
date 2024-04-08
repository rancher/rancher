# Node Scaling

These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in Rancher, you should create one first before running this test.

Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Getting Started](#Getting-Started)
2. [Replacing Nodes](#Node-Replacing)
3. [Scaling Existing Node Pools](#Scaling-Existing-Node-Pools)

## Getting Started
In your config file, set the following:
```yaml
rancher:
  host: "rancher_server_address"
  adminToken: "rancher_admin_token"
  clusterName: "cluster_to_run_tests_on"
  insecure: true/optional
  cleanup: false/optional
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

These tests utilize Go build tags. Due to this, see the below examples on how to run the tests:

### RKE1
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1NodeScaleDownAndUp/TestEtcdScaleDownAndUp"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1NodeScaleDownAndUp/TestControlPlaneScaleDownAndUp"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1NodeScaleDownAndUp/TestWorkerScaleDownAndUp"`

### RKE2 | K3S
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestNodeScaleDownAndUp/TestEtcdScaleDownAndUp"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestNodeScaleDownAndUp/TestControlPlaneScaleDownAndUp"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestNodeScaleDownAndUp/TestWorkerScaleDownAndUp"`

## Scaling Existing Node Pools
Similar to the `provisioning` tests, the node scaling tests have static test cases as well as dynamicInput tests you can specify. In order to run the dynamicInput tests, you will need to define the `scalingInput` block in your config file. This block defines the quantity you would like the pool to be scaled up/down to. See an example below:
```yaml
scalingInput:
  nodePools:
    nodeRoles:
      worker: true
      quantity: 2
  machinePools:
    nodeRoles:
      etcd: true
      quantity: 1
scalingInput:
  nodeProvider: "ec2"
  nodePools:
    nodeRoles:
      worker: true
      quantity: 2
  machinePools:
    nodeRoles:
      etcd: true
      quantity: 1
  aksNodePool:
    nodeCount: 1
  eksNodePool:
    desiredSize: 1
  gkeNodePool:
    initialNodeCount: 1
```
NOTE: When scaling AKS and EKS, you will need to make sure that the `maxCount` and `maxSize` parameter is greater than the desired scale amount, respectively. For example, if you wish to have 6 total EKS nodes, then the `maxSize` parameter needs to be at least 7. This is not a limitation of the automation, but rather how EkS specifically handles nodegroups.

These tests utilize Go build tags. Due to this, see the below examples on how to run the tests:

### RKE1
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1NodeScalingTestSuite/TestScalingRKE1NodePools"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE1NodeScalingTestSuite/TestScalingRKE1NodePoolsDynamicInput"`

### RKE2 | K3S
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE2NodeScalingTestSuite/TestRKE2NodeScalingTestSuite"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestRKE2NodeScalingTestSuite/TestScalingRKE2NodePoolsDynamicInput"`

### AKS
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestAKSNodeScalingTestSuite/TestScalingAKSNodePools"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestAKSNodeScalingTestSuite/TestScalingAKSNodePoolsDynamicInput"`

### EKS
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestEKSNodeScalingTestSuite/TestScalingEKSNodePools"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestEKSNodeScalingTestSuite/TestScalingEKSNodePoolsDynamicInput"`

### GKE
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestGKENodeScalingTestSuite/TestScalingGKENodePools"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestGKENodeScalingTestSuite/TestScalingGKENodePoolsDynamicInput"`

If the specified test passes immediately without warning, try adding the `-count=1` flag to get around this issue. This will avoid previous results from interfering with the new test run.


## Auto Replacing Nodes
If UnhealthyNodeTimeout is set on your machinepools, auto_replace_test.go will replace a single node with the given role. There are static tests for Etcd, ControlPlane and Worker roles.

If UnhealthyNodeTimeout is not set, the test(s) in this suite will wait for the cluster upgrade default timeout to be reached (30 mins), expecting an error on the node to remain as a negative test. 

Each test requires 2 or more nodes in the specified role's pool. i.e. if you're running the entire suite, you would need 3etcd, 2controlplane, 2worker, minimum. 

### RKE2 | K3S
`gotestsum --format standard-verbose --packages=github.com/rancher/rancher/tests/v2/validation/nodescaling --junitfile results.xml -- -timeout=60m -tags=validation -v -run "TestEtcdAutoReplaceRKE2K3S/TestEtcdAutoReplaceRKE2K3S"`