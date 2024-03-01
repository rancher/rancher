package fleet

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
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
		return "", fmt.Errorf("fleet.GetClusterHost: unable to determine cluster host: %w", err)
	}

	if clientCfg == nil {
		return fail(errors.New("client config not set"))
	}

	rawConfig, err := clientCfg.RawConfig()
	if err != nil {
		return fail(fmt.Errorf("no configuration available: %w", err))
	}

	cluster, ok := rawConfig.Clusters[rawConfig.CurrentContext]
	if ok {
		return cluster.Server, nil
	}

	logrus.Warnf(
		"API server host retrieval: no cluster found for the current context (%s)",
		rawConfig.CurrentContext,
	)

	for k, v := range rawConfig.Clusters {
		logrus.Warnf(
			"API server host retrieval: picking server %s "+
				"with reference %s randomly from set of configured clusters",
			v.Server,
			k,
		)
		return v.Server, nil
	}

	return "", errors.New("failed to find cluster server parameter")
}
