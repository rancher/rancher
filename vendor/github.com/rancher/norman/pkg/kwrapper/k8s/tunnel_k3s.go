// +build k8s

package k8s

import (
	"context"
	"net"
	"time"

	"github.com/rancher/norman/pkg/kv"
	"github.com/rancher/norman/pkg/remotedialer"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
)

func setupK3s(tunnelServer *remotedialer.Server) {
	app.DefaultProxyDialerFn = utilnet.DialFunc(func(_ context.Context, network, address string) (net.Conn, error) {
		_, port, _ := net.SplitHostPort(address)
		addr := "127.0.0.1"
		if port != "" {
			addr += ":" + port
		}
		nodeName, _ := kv.Split(address, ":")
		return tunnelServer.Dial(nodeName, 15*time.Second, "tcp", addr)
	})
}
