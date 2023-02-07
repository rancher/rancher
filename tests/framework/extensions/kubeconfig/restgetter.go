package kubeconfig

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type RestGetter struct {
	genericclioptions.RESTClientGetter
	restConfig   *rest.Config
	clientConfig clientcmd.ClientConfig
}

func NewRestGetter(restConfig *rest.Config, clientConfig clientcmd.ClientConfig) *RestGetter {
	return &RestGetter{
		restConfig:   restConfig,
		clientConfig: clientConfig,
	}
}

// ToRESTConfig returns restconfig
func (r *RestGetter) ToRESTConfig() (*rest.Config, error) {
	return r.restConfig, nil
}

// ToDiscoveryClient returns discovery client
func (r *RestGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return nil, nil
}

// ToRESTMapper returns a restmapper
func (r *RestGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return nil, nil
}

// ToRawKubeConfigLoader return kubeconfig loader as-is
func (r *RestGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return r.clientConfig
}
