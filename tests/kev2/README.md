# KEv2 Unit Tests

KEv2 integration (unit) tests tests code functionality in the rancher/rancher handler for each KEv2 cloud provider (AKS EKS and GKE). The tests are written using the [Go Test Framework](https://pkg.go.dev/testing) and [MockCompose](https://github.com/kelveny/mockcompose). They mock the cluster state and `AKSClusterConfig` objects using test files to simulate reactions to a real cluster, and mock any mockable Interfaces used by the OperatorController. Test files are located in each handler folder and data used by the tests are located in a `/test` sub folder. The unit test structure is different from `rancher/rancher/tests` because [Go Test Framework](https://pkg.go.dev/testing) requires test files to be in the same package as the source code.

## File structure

## Requirements

---

To run kev2 unit tests you will need:
- [golang 1.17](https://go.dev/doc/install)

## File Structure

---

```
rancher/rancher
-> pkg/controllers/management/aks
     -> aks_cluster_handler.go
     -> aks_cluster_handler_test.go
     -> tests/
          -> test1.yaml
          -> test2.json
```

## How to Run Unit Tests

Clone the repo

```
Git clone rancher/rancher
cd rancher
```

If you want to test each provider separately

```
go test -v ./pkg/controllers/management/aks
go test -v ./pkg/controllers/management/eks
go test -v ./pkg/controllers/management/gke
```

If you want to run all tests

```
go test -v ./pkg/controllers/management/aks ./pkg/controllers/management/eks ./pkg/controllers/management/gke
```

If you want to check test coverage (example: 72.5% of statements)

```
go test (or alias command) -coverprofile=coverage.out
go tool cover -html=coverage.out
```