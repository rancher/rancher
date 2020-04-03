package dialer

import (
	"context"
	"net"
)

type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

type Factory interface {
	ClusterDialer(clusterName string) (Dialer, error)
	DockerDialer(clusterName, machineName string) (Dialer, error)
	NodeDialer(clusterName, machineName string) (Dialer, error)
}
