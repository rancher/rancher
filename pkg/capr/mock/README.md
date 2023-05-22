# mock

These are the mock controllers/clients/caches that are used to allow unit testing of the Planner.

### Maintenance

Make sure you have `mockgen`, you can get it through

```
go install github.com/golang/mock/mockgen@v1.6.0
```

Make sure your `$GOPATH/bin` is in your path, i.e.
```
export PATH=$PATH:/root/go/bin
```

They were generated through the following commands:

```
mockgen --build_flags=--mod=mod -package mockrkecontrollers -destination ./pkg/capr/mock/mockrkecontrollers/mock_rkecontrollers.go github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1 RKEBootstrapClient,RKEBootstrapCache,RKEControlPlaneController,ETCDSnapshotCache
mockgen --build_flags=--mod=mod -package mockcorecontrollers -destination ./pkg/capr/mock/mockcorecontrollers/mock_corecontrollers.go github.com/rancher/wrangler/pkg/generated/controllers/core/v1 SecretClient,SecretCache,ConfigMapCache
mockgen --build_flags=--mod=mod -package mockmgmtcontrollers -destination ./pkg/capr/mock/mockmgmtcontrollers/mock_mgmtcontrollers.go github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3 ClusterRegistrationTokenCache,ClusterCache
mockgen --build_flags=--mod=mod -package mockcapicontrollers -destination ./pkg/capr/mock/mockcapicontrollers/mock_capicontrollers.go github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1 MachineClient,MachineCache,ClusterClient,ClusterCache
mockgen --build_flags=--mod=mod -package mockranchercontrollers -destination ./pkg/capr/mock/mockranchercontrollers/mock_ranchercontrollers.go github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1 ClusterCache
mockgen --build_flags=--mod=mod -package mockfleetcontrollers -destination ./pkg/capr/mock/mockfleetcontrollers/mock_fleetcontrollers.go github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1 BundleController
```

Eventually, when Wrangler is updated to generate mock clients, we should use that instead of generating our own mock clients/controllers/caches.

### Usage

Most information on using mock can be found by looking at the godoc for mock, but the gist is you define your mock interfaces and "set them up" by using `.Expect()` calls where you pre-define expected calls to the mock interfaces (and define the returns). 

You can instantiate a "mockPlanner" using the function `newMockPlanner()` that is in `planner_test.go`.

### Example

Look at `Test_rotateCertificatesPlan` in `pkg/capr/planner/certificaterotation_test.go`. 