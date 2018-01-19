package dialer

import (
	"crypto/tls"
	"net"

	"strings"

	"fmt"

	"github.com/rancher/machine-controller/store"
	"github.com/rancher/machine-controller/store/config"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
)

type TLSDialerFactory struct {
	Store           *store.GenericEncryptedStore
	MachineClient   v3.MachineInterface
	ConfigMapGetter v1.ConfigMapsGetter
}

func (t *TLSDialerFactory) Build(h *hosts.Host) (func(network, address string) (net.Conn, error), error) {
	if h.MachineName == "" {
		return hosts.SSHFactory(h)
	}

	parts := strings.SplitN(h.MachineName, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid name reference %s", h.MachineName)
	}

	machine, err := t.MachineClient.GetNamespaced(parts[0], parts[1], metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	config, err := config.NewMachineConfig(t.Store, machine)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		return nil, err
	}

	realTLSConfig, err := tlsConfig.ToConfig()
	if err != nil {
		return nil, err
	}

	d := &TLSDialer{
		Config:  realTLSConfig,
		Address: tlsConfig.Address,
	}

	return d.Dial, nil
}

type TLSDialer struct {
	Config  *tls.Config
	Address string
}

func (t *TLSDialer) Dial(network, address string) (net.Conn, error) {
	return tls.Dial("tcp", t.Address, t.Config)
}
