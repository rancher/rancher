package clusterprovisioner

import (
	"fmt"
	"net"
	"strings"

	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rke/hosts"
)

type RKEDialerFactory struct {
	Factory dialer.Factory
	Docker  bool
}

func (t *RKEDialerFactory) Build(h *hosts.Host) (func(network, address string) (net.Conn, error), error) {
	if h.NodeName == "" {
		return hosts.SSHFactory(h)
	}

	parts := strings.SplitN(h.NodeName, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid name reference %s", h.NodeName)
	}

	if t.Docker {
		return t.Factory.DockerDialer(parts[0], parts[1])
	}
	return t.Factory.NodeDialer(parts[0], parts[1])
}
