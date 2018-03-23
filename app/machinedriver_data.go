package app

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addMachineDrivers(management *config.ManagementContext) error {
	if err := addMachineDriver("amazonec2", "local://", "", []string{"*.amazonaws.com", "*.amazonaws.com.cn"}, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("digitalocean", "local://", "", []string{"api.digitalocean.com"}, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("exoscale", "local://", "", []string{"api.exoscale.ch"}, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("openstack", "local://", "", nil, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("otc", "https://obs.otc.t-systems.com/dockermachinedriver/docker-machine-driver-otc",
		"e98f246f625ca46f5e037dc29bdf00fe", []string{"*.otc.t-systems.com"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("packet", "https://github.com/packethost/docker-machine-driver-packet/releases/download/v0.1.4/docker-machine-driver-packet_linux-amd64.zip",
		"2cd0b9614ab448b61b1bf73ef4738ab5", []string{"api.packet.net"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("rackspace", "local://", "", nil, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("softlayer", "local://", "", nil, false, true, management); err != nil {
		return err
	}

	return addMachineDriver("vmwarevsphere", "local://", "", nil, true, true, management)
}

func addMachineDriver(name, url, checksum string, whitelist []string, active, builtin bool, management *config.ManagementContext) error {
	lister := management.Management.NodeDrivers("").Controller().Lister()
	cli := management.Management.NodeDrivers("")
	m, _ := lister.Get("", name)
	if m != nil {
		if m.Spec.Builtin != builtin || m.Spec.URL != url || m.Spec.Checksum != checksum || m.Spec.DisplayName != name {
			logrus.Infof("Updating node driver %v", name)
			m.Spec.Builtin = builtin
			m.Spec.URL = url
			m.Spec.Checksum = checksum
			m.Spec.DisplayName = name
			m.Spec.WhitelistDomains = whitelist
			_, err := cli.Update(m)
			return err
		}
		return nil
	}

	logrus.Infof("Creating node driver %v", name)
	_, err := cli.Create(&v3.NodeDriver{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Spec: v3.NodeDriverSpec{
			Active:           active,
			Builtin:          builtin,
			URL:              url,
			DisplayName:      name,
			Checksum:         checksum,
			WhitelistDomains: whitelist,
		},
	})

	return err
}
