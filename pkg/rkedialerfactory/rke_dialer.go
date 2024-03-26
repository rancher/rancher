package rkedialerfactory

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/rancher/rke/hosts"
	rketypes "github.com/rancher/rke/types"
	"k8s.io/client-go/transport"
)

type RKEDialerFactory struct {
	Factory dialer.Factory
	Docker  bool
	Ctx     context.Context
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
		return func(network, address string) (net.Conn, error) {
			d, err := t.Factory.DockerDialer(parts[0], parts[1])
			if err != nil {
				return nil, err
			}
			return d(t.Ctx, network, address)
		}, nil
	}
	return func(network, address string) (net.Conn, error) {
		d, err := t.Factory.NodeDialer(parts[0], parts[1])
		if err != nil {
			return nil, err
		}
		return d(t.Ctx, network, address)
	}, nil
}

func (t *RKEDialerFactory) WrapTransport(config *rketypes.RancherKubernetesEngineConfig) transport.WrapperFunc {
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
				ht.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
					ip, _, _ := net.SplitHostPort(address)
					if privateIP, ok := translateAddress[ip]; ok {
						address = strings.Replace(address, ip, privateIP, 1)
					}
					conn, err := dialer(ctx, network, address)
					if ref.IsNodeNotFound(err) {
						clusterDialer, dialerErr := t.Factory.ClusterDialer(ns, true)
						if dialerErr == nil {
							return clusterDialer(ctx, network, address)
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
