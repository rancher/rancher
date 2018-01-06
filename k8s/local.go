// +build !k8s

package k8s

import (
	"context"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetConfig(ctx context.Context, k8sMode string, addLocal bool, kubeConfig string, internalAPIPort int) (*rest.Config, bool, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	return cfg, true, err
}
