package dialer

import "net"

type Dialer func(network, address string) (net.Conn, error)

type Factory interface {
	ClusterDialer(clusterName string) (Dialer, error)
	DockerDialer(clusterName, machineName string) (Dialer, error)
	NodeDialer(clusterName, machineName string) (Dialer, error)
}
