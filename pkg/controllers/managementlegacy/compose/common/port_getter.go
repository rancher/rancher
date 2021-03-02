package common

import clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

type KubeConfigGetter interface {
	KubeConfig(clusterName, token string) *clientcmdapi.Config
	GetHTTPSPort() int
}
