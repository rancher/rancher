package mapper

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

func New(config *rest.Config) (meta.RESTMapper, error) {
	d, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	cached := memory.NewMemCacheClient(d)
	return restmapper.NewDeferredDiscoveryRESTMapper(cached), nil
}
