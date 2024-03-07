package fleet

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// GetClusterHost returns the API server host and CA for the specified client configuration.
func GetClusterHost(clientCfg clientcmd.ClientConfig) (string, []byte, error) {
	icc, err := rest.InClusterConfig()
	if err == nil {
		ca, err := os.ReadFile(icc.CAFile)
		return icc.Host, ca, err
	}

	fail := func(err error) (string, []byte, error) {
		return "", []byte{}, fmt.Errorf("fleet.GetClusterHost: unable to determine cluster host: %w", err)
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
		ca, err := getCA(cluster)
		return cluster.Server, ca, err
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
		ca, err := getCA(v)
		return v.Server, ca, err
	}

	return "", []byte{}, errors.New("failed to find cluster server parameter")
}

// getCA retrieves certificate authority information for the specified cluster.
func getCA(cluster *api.Cluster) ([]byte, error) {
	if len(cluster.CertificateAuthorityData) > 0 {
		return cluster.CertificateAuthorityData, nil
	}
	return os.ReadFile(cluster.CertificateAuthority)
}
