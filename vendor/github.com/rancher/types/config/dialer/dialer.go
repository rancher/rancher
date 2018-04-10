package dialer

import "net"

type Dialer func(network, address string) (net.Conn, error)

type Factory interface {
	LocalClusterDialer() Dialer
	ClusterDialer(clusterName string,disableKeepAlives bool) (Dialer, error)
	DockerDialer(clusterName, machineName string) (Dialer, error)
	NodeDialer(clusterName, machineName string) (Dialer, error)
}
