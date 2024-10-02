package dialer

import (
	"context"
	"net"

	"k8s.io/client-go/transport"
)

type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

type Factory interface {
	// ClusterDialer returns a dialer that can be used to connect to a cluster's Kubernetes API
	// Note that the dialer may or may not use a remotedialer tunnel
	// If retryOnError is true, the dialer will retry for ~30s in case it cannot connect,
	// otherwise return immediately
	// NOTE: ClusterDialer must not be used for Kubernetes clients; use ClusterDialHolder instead.
	ClusterDialer(clusterName string, retryOnError bool) (Dialer, error)
	// ClusterDialHolder returns a ClusterDialer, wrapped inside a Kubernetes' transport struct
	// used to allow caching of transports.
	// Using a custom Dial in rest.Config will cause this cache to grow indefinitely.
	// see: https://github.com/kubernetes/kubernetes/issues/125818
	ClusterDialHolder(clusterName string, retryOnError bool) (*transport.DialHolder, error)
	DockerDialer(clusterName, machineName string) (Dialer, error)
	NodeDialer(clusterName, machineName string) (Dialer, error)
}
