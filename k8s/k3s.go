// +build k3s

package k8s

import (
	"context"
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/wrapper/server"
)

func getEmbedded(ctx context.Context) (bool, context.Context, *rest.Config, error) {
	sc, ok := ctx.Value(serverConfig).(*server.ServerConfig)
	if !ok {
		return false, ctx, nil, fmt.Errorf("failed to find k3s config")
	}

	err := server.Server(ctx, sc)
	if err != nil {
		return false, ctx, nil, err
	}

	os.Setenv("KUBECONFIG", sc.KubeConfig)
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: sc.KubeConfig}, &clientcmd.ConfigOverrides{}).ClientConfig()

	return true, ctx, restConfig, err
}
