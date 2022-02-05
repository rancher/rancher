package kubeconfig

import (
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// GetContexts is a helper function the lists the contexts of a kubeconfig
func GetContexts(clientConfig *clientcmd.ClientConfig) (map[string]*api.Context, error) {
	rawConfig, err := (*clientConfig).RawConfig()
	if err != nil {
		return nil, err
	}

	return rawConfig.Contexts, nil
}
