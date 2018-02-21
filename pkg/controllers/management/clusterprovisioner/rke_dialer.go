package clusterprovisioner

import (
	"fmt"
	"net"
	"strings"

	"net/http"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
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

func (t *RKEDialerFactory) WrapTransport(config *v3.RancherKubernetesEngineConfig) k8s.WrapTransport {
	for _, node := range config.Nodes {
		if !slice.ContainsString(node.Role, "controlplane") {
			continue
		}

		ns, n := ref.Parse(node.NodeName)
		dialer, err := t.Factory.NodeDialer(ns, n)
		if dialer == nil || err != nil {
			continue
		}

		return func(rt http.RoundTripper) http.RoundTripper {
			if ht, ok := rt.(*http.Transport); ok {
				ht.DialContext = nil
				ht.DialTLS = nil
				ht.Dial = dialer
			}
			return rt
		}
	}

	return nil
}
