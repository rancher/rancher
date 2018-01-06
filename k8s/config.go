// +build k8s

package k8s

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/k8s/service"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetConfig(ctx context.Context, k8sMode string, addLocal bool, kubeConfig string, internalAPIPort int) (*rest.Config, bool, error) {
	if kubeConfig == "" {
		if config, err := rest.InClusterConfig(); err == nil {
			return config, true, nil
		}
		switch k8sMode {
		case "internal":
			return runServices(ctx, true, internalAPIPort), addLocal, nil
		case "exec":
			return runServices(ctx, false, internalAPIPort), addLocal, nil
		default:
			return nil, false, fmt.Errorf("invalid k8s-mode: %s", k8sMode)
		}
	} else {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		return cfg, addLocal, err
	}

}

func runServices(ctx context.Context, internal bool, internalAPIPort int) *rest.Config {
	go service.Service(ctx, internal, "etcd")
	go service.Service(ctx, internal, "api-server")

	return &rest.Config{
		Host: fmt.Sprintf("http://localhost:%d", internalAPIPort),
	}
}
