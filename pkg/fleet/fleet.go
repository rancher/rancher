package fleet

import (
	"errors"
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClusterHost returns the API server host for the specified client configuration.
func GetClusterHost(clientCfg clientcmd.ClientConfig) (string, error) {
	icc, err := rest.InClusterConfig()
	if err == nil {
		return icc.Host, nil
	}

	fail := func(err error) (string, error) {
		return "", fmt.Errorf("fleet.GetClusterHost: %w", err)
	}

	if clientCfg == nil {
		return fail(errors.New("client config not set"))
	}

	rawConfig, err := clientCfg.RawConfig()
	if err != nil {
		return fail(errors.New("could not convert client config into raw config"))
	}

	cluster, ok := rawConfig.Clusters[rawConfig.CurrentContext]
	if ok {
		return cluster.Server, nil
	}

	for _, v := range rawConfig.Clusters {
		return v.Server, nil
	}

	return "", errors.New("failed to find cluster server parameter")
}
