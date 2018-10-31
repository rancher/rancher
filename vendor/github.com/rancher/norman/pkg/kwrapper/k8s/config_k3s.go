// +build k8s

package k8s

import (
	"context"
	"net"
	"net/http"

	"github.com/rancher/norman/pkg/remotedialer"
	"github.com/rancher/norman/pkg/resolvehome"
	"k8s.io/kubernetes/pkg/wrapper/server"
)

func NewK3sConfig(ctx context.Context, dataDir string, authorizer remotedialer.Authorizer) (context.Context, interface{}, http.Handler, error) {
	dataDir, err := resolvehome.Resolve(dataDir)
	if err != nil {
		return ctx, nil, nil, err
	}

	listenIP := net.ParseIP("127.0.0.1")
	_, clusterIPNet, _ := net.ParseCIDR("10.42.0.0/16")
	_, serviceIPNet, _ := net.ParseCIDR("10.43.0.0/16")

	sc := &server.ServerConfig{
		AdvertiseIP:    &listenIP,
		AdvertisePort:  6444,
		PublicHostname: "localhost",
		ListenAddr:     listenIP,
		ListenPort:     6443,
		ClusterIPRange: *clusterIPNet,
		ServiceIPRange: *serviceIPNet,
		UseTokenCA:     true,
		DataDir:        dataDir,
	}

	ctx = SetK3sConfig(ctx, sc)
	return ctx, sc, newTunnel(authorizer), nil
}
