package dialer

import (
	"fmt"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func (f *factory) sshDialer(machine *v3.Node) (Dialer, error) {
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

func (f *factory) sshLocalDialer(machine *v3.Node) (Dialer, error) {
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
