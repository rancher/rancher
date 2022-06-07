# rancher-terratest

Automated tests for Rancher using Terraform + Terratest

Provisioning:
- AWS Node driver
  - RKE1
  - RKE2
  - K3s
- Hosted
  - AKS


Functions:
- **CleanupConfigTF**:
  - parameters - (`module string`);
  - description - cleans main.tf of desired module
- **GetClusterID**: 
  - parameters - (`url string`, `clusterName string`, `bearer token string`); returns `string`
  - description - returns the cluster's id
- **GetClusterName**:
  - parameters - (`url string`, `clusterID string`, `bearer token string`); returns `string`
  - description - returns the cluster's name
- **GetClusterNodeCount**:
  - parameters - (`url string`, `clusterID string`, `bearer token string`); returns `int`
  - description - returns the cluster's node count
- **GetClusterProvider**:
  - parameters - (`url string`, `clusterID string`, `bearer token string`); returns `string`
  - description - returns the cluster's provider
- **GetClusterState**:
  - parameters - (`url string`, `clusterID string`, `bearer token string`); returns `string`
  - description - returns the cluster's current state
- **GetKubernetesVersion**:
  - parameters - (`url string`, `clusterID string`, `bearer token string`); returns `string`
  - description - returns the cluster's kubernetes version
- **GetRancherServerVersion**:
  - parameters - (`url string`, `bearer token string`); returns `string`
  - description - returns rancher's server version
- **GetUserID**:
  - parameters - (`url string`, `bearer token string`); returns `string`
  - description - returns admin user id
- **OutputToInt**:
  - parameters - (`output string`); returns `int`
  - description - returns tf output as type int
  - note - tf outputs values as type string;
- **SetConfigTF**: 
  - parameters - (`module string`, `k8sVersion string`, `nodepools []models.Nodepool`; returns `bool`
  - description - sets config of desired module and overwrites exiting main.tf
- **WaitForActiveCluster**:
  - parameters - (`url string`, `clusterName string`, `bearer token string`)
  - description - waits until cluster is in an active state

Testing:
- Create and export configuration specs in config.go, to later reference in tests
- Create a new _test.go file in the `tests` folder and begin writing a test
- Most functions take in a url, token, name, or id; it is recommended to grab these values before writing tests
  ```
  // Grab variables for reference w/ testing functions below
	url := terraform.Output(t, terraformOptions, "host_url")
	token := terraform.Output(t, terraformOptions, "token_prefix") + terraform.Output(t, terraformOptions, "token")
	name := terraform.Output(t, terraformOptions, "cluster_name")
	id := functions.GetClusterID(url, name, token)
  ```


Note: 
- Extending the test timeout is a best practice
- The default timeout when testing with Go is 10 mins
- To extend timeout, add `-timeout <int>m` when running tests
  - e.g. `go test <testfile>.go -timeout 45m` || `go test <testfile>.go -timeout 1h`
- Tests that timeout will likely not have cleaned up resources properly
