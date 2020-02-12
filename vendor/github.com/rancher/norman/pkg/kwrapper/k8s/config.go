package k8s

import (
	"context"
	"fmt"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetConfig(ctx context.Context, k8sMode string, kubeConfig string) (bool, clientcmd.ClientConfig, error) {
	var (
		cfg clientcmd.ClientConfig
		err error
	)

	switch k8sMode {
	case "auto":
		return getAuto(ctx, kubeConfig)
	case "embedded":
		return getEmbedded(ctx)
	case "external":
		cfg = getExternal(kubeConfig)
	default:
		return false, nil, fmt.Errorf("invalid k8s-mode %s", k8sMode)
	}

	return false, cfg, err
}

func getAuto(ctx context.Context, kubeConfig string) (bool, clientcmd.ClientConfig, error) {
	if isManual(kubeConfig) {
		return false, kubeconfig.GetNonInteractiveClientConfig(kubeConfig), nil
	}

	return getEmbedded(ctx)
}

func isManual(kubeConfig string) bool {
	if kubeConfig != "" {
		return true
	}
	_, inClusterErr := rest.InClusterConfig()
	return inClusterErr == nil
}

func getExternal(kubeConfig string) clientcmd.ClientConfig {
	return kubeconfig.GetNonInteractiveClientConfig(kubeConfig)
}
