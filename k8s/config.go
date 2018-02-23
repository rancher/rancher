package k8s

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetConfig(ctx context.Context, k8sMode string, kubeConfig string) (context.Context, *rest.Config, error) {
	var (
		cfg *rest.Config
		err error
	)

	switch k8sMode {
	case "auto":
		return getAuto(ctx, kubeConfig)
	case "embedded":
		return getEmbedded(ctx)
	case "external":
		cfg, err = getExternal(kubeConfig)
	default:
		return nil, nil, fmt.Errorf("invalid k8s-mode %s", k8sMode)
	}

	return ctx, cfg, err
}

func getAuto(ctx context.Context, kubeConfig string) (context.Context, *rest.Config, error) {
	if kubeConfig != "" {
		cfg, err := getExternal(kubeConfig)
		return ctx, cfg, err
	}

	if config, err := rest.InClusterConfig(); err == nil {
		return ctx, config, nil
	}

	return getEmbedded(ctx)
}

func getExternal(kubeConfig string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", kubeConfig)
}
