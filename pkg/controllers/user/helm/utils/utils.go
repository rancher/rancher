package utils

import (
	"encoding/base64"

	"k8s.io/client-go/rest"
	"strings"
)

func RestToRaw(config rest.Config) KubeConfig {
	rawConfig := KubeConfig{}
	host := config.Host
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	rawConfig.CurrentContext = "default"
	rawConfig.APIVersion = "v1"
	rawConfig.Kind = "Config"
	rawConfig.Clusters = []configCluster{
		{
			Name: "default",
			Cluster: dataCluster{
				Server: host,
				CertificateAuthorityData: base64.StdEncoding.EncodeToString(config.TLSClientConfig.CAData),
			},
		},
	}
	rawConfig.Contexts = []configContext{
		{
			Name: "default",
			Context: contextData{
				User:    "kube-admin",
				Cluster: "default",
			},
		},
	}
	rawConfig.Users = []configUser{
		{
			Name: "kube-admin",
			User: userData{
				Token: config.BearerToken,
			},
		},
	}
	return rawConfig
}
