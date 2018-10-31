// +build k8s

package k8s

import (
	"context"
	"os"

	"github.com/rancher/norman/pkg/kwrapper/etcd"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/wrapper/server"
)

func getEmbedded(ctx context.Context) (bool, context.Context, *rest.Config, error) {
	var (
		err error
	)

	sc, ok := ctx.Value(serverConfig).(*server.ServerConfig)
	if !ok {
		ctx, obj, _, err := NewK3sConfig(ctx, "./management-state", nil)
		if err != nil {
			return false, ctx, nil, err
		}
		sc = obj.(*server.ServerConfig)
		sc.NoScheduler = false
	}

	if len(sc.ETCDEndpoints) == 0 {
		etcdEndpoints, err := etcd.RunETCD(ctx, sc.DataDir)
		if err != nil {
			return false, ctx, nil, err
		}
		sc.ETCDEndpoints = etcdEndpoints
	}

	if err = server.Server(ctx, sc); err != nil {
		return false, ctx, nil, err
	}

	os.Setenv("KUBECONFIG", sc.KubeConfig)
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: sc.KubeConfig}, &clientcmd.ConfigOverrides{}).ClientConfig()

	return true, ctx, restConfig, err
}
