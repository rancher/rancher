package app

import (
	"reflect"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

var driverData = map[string]map[string][]string{
	"amazonec2":     {"cred": []string{"accessKey"}},
	"azure":         {"cred": []string{"clientId", "subscriptionId"}},
	"vmwarevsphere": {"cred": []string{"username", "vcenter", "vcenterPort"}},
}

type machineDriverCompare struct {
	builtin     bool
	url         string
	uiURL       string
	checksum    string
	name        string
	whitelist   []string
	annotations map[string]string
}

func addMachineDrivers(management *config.ManagementContext) error {
	if err := addMachineDriver("aliyunecs", "http://machine-driver.oss-cn-shanghai.aliyuncs.com/aliyun/1.0.2/linux/amd64/docker-machine-driver-aliyunecs.tgz",
		"", "c31b9da2c977e70c2eeee5279123a95d", []string{"ecs.aliyuncs.com"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("amazonec2", "local://", "", "", []string{"*.amazonaws.com", "*.amazonaws.com.cn"}, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("azure", "local://", "", "", nil, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudca", "https://github.com/cloud-ca/docker-machine-driver-cloudca/files/2446837/docker-machine-driver-cloudca_v2.0.0_linux-amd64.zip",
		"https://objects-east.cloud.ca/v1/5ef827605f884961b94881e928e7a250/ui-driver-cloudca/v2.1.2/component.js", "2a55efd6d62d5f7fd27ce877d49596f4",
		[]string{"objects-east.cloud.ca"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("digitalocean", "local://", "", "", []string{"api.digitalocean.com"}, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("exoscale", "local://", "", "", []string{"api.exoscale.ch"}, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("linode", "https://github.com/linode/docker-machine-driver-linode/releases/download/v0.1.6/docker-machine-driver-linode_linux-amd64.zip",
		"https://linode.github.io/rancher-ui-driver-linode/releases/v0.2.0/component.js", "4d53a20a6ee3090a713c48c2d3f5ed45",
		[]string{"linode.github.io"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("openstack", "local://", "", "", nil, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("otc", "https://dockermachinedriver.obs.eu-de.otc.t-systems.com/docker-machine-driver-otc",
		"", "b4c05f6598dcfac7a8f10899aaac3d42", []string{"*.otc.t-systems.com"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("packet", "https://github.com/packethost/docker-machine-driver-packet/releases/download/v0.1.4/docker-machine-driver-packet_linux-amd64.zip",
		"", "2cd0b9614ab448b61b1bf73ef4738ab5", []string{"api.packet.net"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("rackspace", "local://", "", "", nil, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("softlayer", "local://", "", "", nil, false, true, management); err != nil {
		return err
	}
	return addMachineDriver("vmwarevsphere", "local://", "", "", nil, true, true, management)
}

func addMachineDriver(name, url, uiURL, checksum string, whitelist []string, active, builtin bool, management *config.ManagementContext) error {
	lister := management.Management.NodeDrivers("").Controller().Lister()
	cli := management.Management.NodeDrivers("")
	m, _ := lister.Get("", name)
	// annotations can have keys cred and password, values []string to be considered as a part of cloud credential
	annotations := map[string]string{}
	if m != nil {
		for k, v := range m.Annotations {
			annotations[k] = v
		}
	}
	for key, fields := range driverData[name] {
		annotations[key] = strings.Join(fields, ",")
	}
	if m != nil {
		old := machineDriverCompare{
			builtin:     m.Spec.Builtin,
			url:         m.Spec.URL,
			uiURL:       m.Spec.UIURL,
			checksum:    m.Spec.Checksum,
			name:        m.Spec.DisplayName,
			whitelist:   m.Spec.WhitelistDomains,
			annotations: m.Annotations,
		}
		new := machineDriverCompare{
			builtin:     builtin,
			url:         url,
			uiURL:       uiURL,
			checksum:    checksum,
			name:        name,
			whitelist:   whitelist,
			annotations: annotations,
		}
		if !reflect.DeepEqual(new, old) {
			logrus.Infof("Updating node driver %v", name)
			m.Spec.Builtin = builtin
			m.Spec.URL = url
			m.Spec.UIURL = uiURL
			m.Spec.Checksum = checksum
			m.Spec.DisplayName = name
			m.Spec.WhitelistDomains = whitelist
			m.Annotations = annotations
			_, err := cli.Update(m)
			return err
		}
		return nil
	}

	logrus.Infof("Creating node driver %v", name)
	_, err := cli.Create(&v3.NodeDriver{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: v3.NodeDriverSpec{
			Active:           active,
			Builtin:          builtin,
			URL:              url,
			UIURL:            uiURL,
			DisplayName:      name,
			Checksum:         checksum,
			WhitelistDomains: whitelist,
		},
	})

	return err
}
