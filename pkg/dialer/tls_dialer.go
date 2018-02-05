package dialer

import (
	"crypto/tls"
	"errors"
	"net"
	"strings"

	"github.com/rancher/rancher/pkg/machine/store/config"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func (f *factory) tlsDialer(machine *v3.Machine) (Dialer, error) {
	config, err := config.NewMachineConfig(f.store, machine)
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

	d := &tlsDialer{
		Config:  realTLSConfig,
		Address: tlsConfig.Address,
	}

	return d.Dial, nil
}

type tlsDialer struct {
	Config  *tls.Config
	Address string
}

func (t *tlsDialer) Dial(network, address string) (net.Conn, error) {
	if !strings.Contains(address, "docker.sock") {
		return nil, errors.New("only docker.sock connections are supported for this node")
	}
	return tls.Dial("tcp", t.Address, t.Config)
}
