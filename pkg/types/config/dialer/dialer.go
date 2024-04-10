package dialer

import (
	"context"
	"net"
)

type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

type Factory interface {
	// ClusterDialer returns a dialer that can be used to connect to a cluster's Kubernetes API
	// Note that the dialer may or may not use a remotedialer tunnel
	// If retryOnError is true, the dialer will retry for ~30s in case it cannot connect,
	// otherwise return immediately
	ClusterDialer(clusterName string, retryOnError bool) (Dialer, error)
	DockerDialer(clusterName, machineName string) (Dialer, error)
	NodeDialer(clusterName, machineName string) (Dialer, error)
}
