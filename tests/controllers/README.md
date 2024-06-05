# Controller Integration Tests

## Summary

The controller integration tests make use of [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest). It starts a local control plane that controllers can be registered to it and tested against. It creates an instance of etcd and the Kubernetes API server, but without other components, making it lightweight and fast.
The library is maintained by kubebuilder. For more information see: https://book.kubebuilder.io/reference/envtest.html

## Dependencies

To run the tests, [`setup-envtest`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/tools/setup-envtest) needs to be installed. That can be done by doing:
```
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
```

## Running Integration Tests

### Makefile
All tests can be run with `make controller-test`. That runs the bash script [`run_controller_tests.sh`](./run_controller_tests.sh).

### Manually

After installing `setup-envtest`, export the environment variable `KUBEBUILDER_ASSETS` to point to where setup-envtest was installed using the following:
```
export KUBEBUILDER_ASSETS=$(setup-envtest use -p path)
```

Note: You can add a k8s version to the export command if you want to test a specific version. Otherwise it will use the latest version installed.

After setting the environment variable, running the tests is the same as any other go test and can be run with `go test`.

## Developing Tests

All integration tests have the following initial steps in common:

1. Start a local kubernetes server

```
testEnv = &envtest.Environment{}
restCfg, err := testEnv.Start()
```

2. Create the CRDs

```
import "github.com/rancher/wrangler/v3/pkg/crd"

factory, err := crd.NewFactoryFromClient(restCfg)
err = factory.BatchCreateCRDs(ctx,
crd.CRD{
    SchemaObject: <CRD struct>,
    NonNamespace: <true/false>,
})
```

After those two initial steps, the process depends on the kind of controller being used.

### Wrangler Controllers

The [feature controller](feature/feature_test.go) is an example of a wrangler controller. It is used for the examples below.

1. Create a wrangler context

```
import "github.com/rancher/rancher/pkg/wrangler"

wranglerContext, err := wrangler.NewContext(ctx, nil, restCfg)
```

2. Register the controller

```
import "github.com/rancher/rancher/pkg/controllers/management/feature"

feature.Register(ctx, wranglerContext)
```

3. Create and start the controller factory

```
import "k8s.io/apimachinery/pkg/runtime/schema"

controllerFactory := wranglerContext.ControllerFactory.ForResourceKind(schema.GroupVersionResource{
    Group: "management.cattle.io",
    Version: "v3",
    Resource: "features",
}, "Feature", false)
err := controllerFactory.Start(ctx, 1)
```

4. (Optional) Start any needed resource caches

For controllers that have caches for other resources, those need to be created and started as well.

```
_, err := wranglerContext.ControllerFactory.SharedCacheFactory().ForKind(schema.GroupVersionKind{
	Group:   "management.cattle.io",
	Version: "v3",
	Kind:    "NodeDriver",
})
err = wranglerContext.ControllerFactory.SharedCacheFactory().StartGVK(s.ctx, schema.GroupVersionKind{
	Group:   "management.cattle.io",
	Version: "v3",
	Kind:    "NodeDriver",
})
```

Now the controller has been registered and started, it can be used for any operations like `wranglerContext.Mgmt.Feature().Get("feature-name", metav1.GetOptions{})`

### Norman Controller
TODO