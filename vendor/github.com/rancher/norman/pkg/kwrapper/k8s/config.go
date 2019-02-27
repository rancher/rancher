package k8s

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Auto(ctx context.Context) (bool, context.Context, *rest.Config, error) {
	return GetConfig(ctx, "auto", os.Getenv("KUBECONFIG"))
}

func GetConfig(ctx context.Context, k8sMode string, kubeConfig string) (bool, context.Context, *rest.Config, error) {
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
		return false, nil, nil, fmt.Errorf("invalid k8s-mode %s", k8sMode)
	}

	return false, ctx, cfg, err
}

func getAuto(ctx context.Context, kubeConfig string) (bool, context.Context, *rest.Config, error) {
	if kubeConfig != "" {
		cfg, err := getExternal(kubeConfig)
		return false, ctx, cfg, err
	}

	if config, err := rest.InClusterConfig(); err == nil {
		if config.BearerToken == "" {
			tokenBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
			if err == nil {
				config.BearerToken = string(tokenBytes)
			}
		}
		return false, ctx, config, nil
	}

	return getEmbedded(ctx)
}

func getExternal(kubeConfig string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", kubeConfig)
}
