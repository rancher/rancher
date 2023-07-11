package kubeconfig

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type RestGetter struct {
	genericclioptions.RESTClientGetter
	restConfig   *rest.Config
	clientConfig clientcmd.ClientConfig
	cache        noCacheDiscoveryClient
}
type noCacheDiscoveryClient struct {
	discovery.DiscoveryInterface
}

// Fresh is a no-op implementation of the corresponding method of the CachedDiscoveryInterface.
// No need to re-try search in the cache, return true.
func (noCacheDiscoveryClient) Fresh() bool { return true }
func (noCacheDiscoveryClient) Invalidate() {}

func NewRestGetter(restConfig *rest.Config, clientConfig clientcmd.ClientConfig) (*RestGetter, error) {
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return &RestGetter{
		restConfig:   restConfig,
		clientConfig: clientConfig,
		cache:        noCacheDiscoveryClient{clientSet.Discovery()},
	}, nil
}

// ToRESTConfig returns restconfig
func (r *RestGetter) ToRESTConfig() (*rest.Config, error) {
	return r.restConfig, nil
}

// ToDiscoveryClient returns a cached discovery client.
func (r *RestGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return r.cache, nil
}

// ToRESTMapper returns a RESTMapper.
func (r *RestGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return restmapper.NewDeferredDiscoveryRESTMapper(r.cache), nil
}

// ToRawKubeConfigLoader return kubeconfig loader as-is
func (r *RestGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return r.clientConfig
}
