package rkedialerfactory

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/rancher/norman/types/slice"
	dialer2 "github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rke/hosts"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	"k8s.io/client-go/transport"
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

func (t *RKEDialerFactory) WrapTransport(config *v3.RancherKubernetesEngineConfig) transport.WrapperFunc {
	translateAddress := map[string]string{}

	for _, node := range config.Nodes {
		if !slice.ContainsString(node.Role, "controlplane") {
			continue
		}
		if node.InternalAddress != "" && node.Address != "" {
			translateAddress[node.Address] = node.InternalAddress
		}
	}

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
				ht.Dial = func(network, address string) (net.Conn, error) {
					ip, _, _ := net.SplitHostPort(address)
					if privateIP, ok := translateAddress[ip]; ok {
						address = strings.Replace(address, ip, privateIP, 1)
					}
					conn, err := dialer(network, address)
					if dialer2.IsNodeNotFound(err) {
						clusterDialer, dialerErr := t.Factory.ClusterDialer(ns)
						if dialerErr == nil {
							return clusterDialer(network, address)
						}
					}
					return conn, err
				}
			}
			return rt
		}
	}

	return nil
}
