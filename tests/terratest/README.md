# rancher-terratest

Automated tests for Terraform rancher2 provider using Terratest

Provisioning and Scaling:
- AWS Node driver
  - RKE1
  - RKE2
  - K3s
- Linode Node driver
  - RKE1
  - RKE2
  - K3s
- Hosted
  - AKS
  - EKS

Kubernetes Upgrade:
- AWS Node driver
  - RKE2
  - K3s
- Linode Node driver
  - RKE2
  - K3s


Functions:
- **CleanupConfigTF**:
  - parameters - (`module string`);
  - description - cleans main.tf of desired module
- **SetConfigTF**: 
  - parameters - (`module string`, `k8sVersion string`, `nodepools []models.Nodepool`; returns `bool`
  - description - sets config of desired module and overwrites existing main.tf
- **WaitForActiveCluster**:
  - parameters - (`t *testing.T`, `client *rancher.Client`, `clusterID string`, `module string`)
  - description - waits until cluster is in an active state


Note: 
- Tests that timeout will not have cleaned up resources
- Extending the test timeout is a best practice; default is 10m
- To extend timeout, add `-timeout <int>m` when running tests
  - e.g. `go test <testfile>.go -timeout 45m` || `go test <testfile>.go -timeout 1h`