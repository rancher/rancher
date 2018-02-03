package dialer

import (
	"fmt"
	"net"
	"time"

	"github.com/rancher/rancher/pkg/machine/store"
	machineconfig "github.com/rancher/rancher/pkg/machine/store/config"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/tunnel"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

func NewFactory(management *config.ManagementContext, tunneler *remotedialer.Server) (Factory, error) {
	if tunneler == nil {
		tunneler = tunnel.NewTunneler(management)
	}

	secretStore, err := machineconfig.NewStore(management)
	if err != nil {
		return nil, err
	}

	return &factory{
		machineLister: management.Management.Machines("").Controller().Lister(),
		tunneler:      tunneler,
		store:         secretStore,
	}, nil
}

type factory struct {
	machineLister v3.MachineLister
	tunneler      *remotedialer.Server
	store         *store.GenericEncryptedStore
}

func (f *factory) ClusterDialer(clusterName string) (Dialer, error) {
	return nil, nil
}

func (f *factory) DockerDialer(clusterName, machineName string) (Dialer, error) {
	machine, err := f.machineLister.Get(clusterName, machineName)
	if err != nil {
		return nil, err
	}

	if machine.Spec.Imported {
		d := f.tunneler.Dialer(machine.Name, 15*time.Second)
		return func(string, string) (net.Conn, error) {
			return d("unix", "/var/run/docker.sock")
		}, nil
	}

	if machine.Spec.CustomConfig != nil && machine.Spec.CustomConfig.Address != "" && machine.Spec.CustomConfig.SSHKey != "" {
		return f.sshDialer(machine)
	}

	if machine.Spec.MachineTemplateName != "" {
		return f.tlsDialer(machine)
	}

	return nil, fmt.Errorf("can not build dailer to %s:%s", clusterName, machineName)

}

func (f *factory) NodeDialer(clusterName, machineName string) (Dialer, error) {
	machine, err := f.machineLister.Get(clusterName, machineName)
	if err != nil {
		return nil, err
	}

	if machine.Spec.Imported {
		d := f.tunneler.Dialer(machine.Name, 15*time.Second)
		return Dialer(d), nil
	}

	if machine.Spec.CustomConfig != nil && machine.Spec.CustomConfig.Address != "" && machine.Spec.CustomConfig.SSHKey != "" {
		return f.sshLocalDialer(machine)
	}

	if machine.Spec.MachineTemplateName != "" {
		return f.sshLocalDialer(machine)
	}

	return nil, fmt.Errorf("can not build dailer to %s:%s", clusterName, machineName)
}
