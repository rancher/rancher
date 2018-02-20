package dialer

import (
	"fmt"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

func (f *Factory) sshDialer(machine *v3.Node) (dialer.Dialer, error) {
	if machine.Status.NodeConfig == nil {
		return nil, fmt.Errorf("waiting to provision %s", machine.Name)
	}
	host := &hosts.Host{
		RKEConfigNode: *machine.Status.NodeConfig,
	}

	sshFactory, err := hosts.SSHFactory(host)
	if err != nil {
		return nil, err
	}
	return sshFactory, nil
}

func (f *Factory) sshLocalDialer(machine *v3.Node) (dialer.Dialer, error) {
	if machine.Status.NodeConfig == nil {
		return nil, fmt.Errorf("waiting to provision %s", machine.Name)
	}
	host := &hosts.Host{
		RKEConfigNode: *machine.Status.NodeConfig,
	}

	sshFactory, err := hosts.LocalConnFactory(host)
	if err != nil {
		return nil, err
	}
	return sshFactory, nil
}
