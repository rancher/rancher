package wrangler_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerschemes "github.com/rancher/wrangler/v3/pkg/schemes"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	_ func(*wrangler.Context) (*config.UserOnlyContext, error)                             = config.NewUserOnlyContext
	_ func(context.Context, clientcmd.ClientConfig, *rest.Config) (*config.Context, error) = wrangler.NewContext
	_ func(context.Context, clientcmd.ClientConfig, *rest.Config) (*config.Context, error) = wrangler.NewPrimaryContext
	_ func(*config.Context) *config.DeferredCAPIInitializer                                = wrangler.NewCAPIInitializer
	_ func(*config.Context) *config.DeferredEXTAPIInitializer                              = wrangler.NewEXTAPIInitializer
)

func TestFacadeTypeIdentity(t *testing.T) {
	require.Equal(t, reflect.TypeFor[config.Context](), reflect.TypeFor[wrangler.Context]())
	require.Equal(t, reflect.TypeFor[config.MultiClusterManager](), reflect.TypeFor[wrangler.MultiClusterManager]())
	require.Equal(t, reflect.TypeFor[config.SimpleRESTClientGetter](), reflect.TypeFor[wrangler.SimpleRESTClientGetter]())
	require.Equal(t, reflect.TypeFor[config.CAPIContext](), reflect.TypeFor[wrangler.CAPIContext]())
	require.Equal(t, reflect.TypeFor[config.EXTAPIContext](), reflect.TypeFor[wrangler.EXTAPIContext]())
	require.Equal(t, reflect.TypeFor[config.DeferredCAPIInitializer](), reflect.TypeFor[wrangler.DeferredCAPIInitializer]())
	require.Equal(t, reflect.TypeFor[config.DeferredEXTAPIInitializer](), reflect.TypeFor[wrangler.DeferredEXTAPIInitializer]())

	clients := &config.Context{}
	scaled := config.ScaledContext{Wrangler: clients}
	management := config.ManagementContext{Wrangler: clients}
	var scaledWrangler, managementWrangler *wrangler.Context = scaled.Wrangler, management.Wrangler
	require.Same(t, clients, scaledWrangler)
	require.Same(t, clients, managementWrangler)
	require.Same(t, clients, (&wrangler.CAPIContext{Context: clients}).Context)
	require.Same(t, clients, (&wrangler.EXTAPIContext{Context: clients}).Context)
}

type initializer struct {
	value string
}

func (i *initializer) WaitForClient(context.Context) (string, error) {
	return i.value, nil
}

func TestGenericFacadeCompatibility(t *testing.T) {
	require.Equal(t, reflect.TypeFor[config.DeferredInitializer[string]](), reflect.TypeFor[wrangler.DeferredInitializer[string]]())
	require.Equal(t, reflect.TypeFor[config.DeferredRegistration[string, *initializer]](), reflect.TypeFor[wrangler.DeferredRegistration[string, *initializer]]())

	var registration *config.DeferredRegistration[string, *initializer] = wrangler.NewDeferredRegistration(
		&config.Context{}, &initializer{value: "ready"}, "compatibility")
	var facade *wrangler.DeferredRegistration[string, *initializer] = registration
	require.Same(t, registration, facade)
	require.Equal(t, "compatibility", facade.Name)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	values := make(chan string, 2)
	facade.DeferFunc(func(value string) { values <- value + "-first" })
	errs := registration.DeferFuncWithError(func(value string) error {
		values <- value + "-second"
		return nil
	})
	facade.Manage(ctx)
	for _, expected := range []string{"ready-first", "ready-second"} {
		select {
		case value := <-values:
			require.Equal(t, expected, value)
		case <-time.After(5 * time.Second):
			t.Fatal("deferred function was not executed")
		}
	}
	select {
	case err, ok := <-errs:
		require.NoError(t, err)
		require.False(t, ok, "successful deferred function should close its error channel")
	case <-time.After(5 * time.Second):
		t.Fatal("deferred function did not close its error channel")
	}
}

func TestSharedScheme(t *testing.T) {
	require.Same(t, config.Scheme, wrangler.Scheme)
	require.Equal(t, reflect.ValueOf(config.AddToScheme).Pointer(), reflect.ValueOf(wrangler.AddToScheme).Pointer())
	schemes := []*runtime.Scheme{config.Scheme, runtime.NewScheme(), runtime.NewScheme()}
	require.NoError(t, config.AddToScheme(schemes[1]))
	require.NoError(t, wrangler.AddToScheme(schemes[2]))
	for _, s := range schemes[1:] {
		metav1.AddToGroupVersion(s, schema.GroupVersion{Version: "v1"})
		require.NoError(t, wranglerschemes.AddToScheme(s))
	}

	resources := []schema.GroupVersionKind{
		{Group: "provisioning.cattle.io", Version: "v1", Kind: "Cluster"},
		{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster"},
		{Group: "fleet.cattle.io", Version: "v1alpha1", Kind: "Bundle"},
		{Group: "management.cattle.io", Version: "v3", Kind: "Cluster"},
		{Group: "project.cattle.io", Version: "v3", Kind: "Workload"},
		{Group: "cluster.cattle.io", Version: "v3", Kind: "ClusterAuthToken"},
		{Group: "rke.cattle.io", Version: "v1", Kind: "RKEControlPlane"},
		{Group: "", Version: "v1", Kind: "Namespace"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"},
		{Group: "apiregistration.k8s.io", Version: "v1", Kind: "APIService"},
		{Group: "catalog.cattle.io", Version: "v1", Kind: "ClusterRepo"},
		{Group: "ext.cattle.io", Version: "v1", Kind: "Token"},
	}
	for _, s := range schemes {
		for _, gvk := range resources {
			_, err := s.New(gvk)
			require.NoError(t, err, "resource %s must be registered", gvk)
		}
	}
	require.Equal(t, schemes[1].AllKnownTypes(), schemes[2].AllKnownTypes())
	require.Equal(t, config.Scheme.AllKnownTypes(), schemes[1].AllKnownTypes())
}

func TestNewContextRetainsNamespaceWrappers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clients, err := wrangler.NewContext(ctx, nil, &rest.Config{Host: "https://127.0.0.1"})
	require.NoError(t, err)
	require.IsType(t, &namespace.Clientset{}, clients.K8s)
	require.Equal(t, reflect.TypeFor[namespace.Clientset]().PkgPath(), reflect.TypeOf(clients.Core.Namespace()).Elem().PkgPath())
	require.NotNil(t, clients.DeferredCAPIRegistration)
	require.NotNil(t, clients.DeferredEXTAPIRegistration)
	_, err = clients.MultiClusterManager.UserContext("missing")
	require.EqualError(t, err, "no cluster manager")
}
