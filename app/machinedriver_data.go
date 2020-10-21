package app

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Amazonec2driver    = "amazonec2"
	Azuredriver        = "azure"
	DigitalOceandriver = "digitalocean"
	ExoscaleDriver     = "exoscale"
	Linodedriver       = "linode"
	OCIDriver          = "oci"
	OTCDriver          = "otc"
	OpenstackDriver    = "openstack"
	PacketDriver       = "packet"
	RackspaceDriver    = "rackspace"
	SoftLayerDriver    = "softlayer"
	Vmwaredriver       = "vmwarevsphere"
)

var driverData = map[string]map[string][]string{
	Amazonec2driver:    {"publicCredentialFields": []string{"accessKey"}, "privateCredentialFields": []string{"secretKey"}},
	Azuredriver:        {"publicCredentialFields": []string{"clientId", "subscriptionId"}, "privateCredentialFields": []string{"clientSecret"}},
	DigitalOceandriver: {"privateCredentialFields": []string{"accessToken"}},
	ExoscaleDriver:     {"privateCredentialFields": []string{"apiSecretKey"}},
	Linodedriver:       {"privateCredentialFields": []string{"token"}, "passwordFields": []string{"rootPass"}},
	OCIDriver:          {"publicCredentialFields": []string{"tenancyId", "userId", "fingerprint"}, "privateCredentialFields": []string{"privateKeyContents"}, "passwordFields": []string{"privateKeyPassphrase"}},
	OTCDriver:          {"privateCredentialFields": []string{"accessKeySecret"}},
	OpenstackDriver:    {"privateCredentialFields": []string{"password"}},
	PacketDriver:       {"privateCredentialFields": []string{"apiKey"}},
	RackspaceDriver:    {"privateCredentialFields": []string{"apiKey"}},
	SoftLayerDriver:    {"privateCredentialFields": []string{"apiKey"}},
	Vmwaredriver:       {"publicCredentialFields": []string{"username", "vcenter", "vcenterPort"}, "privateCredentialFields": []string{"password"}},
}

var driverDefaults = map[string]map[string]string{
	Vmwaredriver: {"vcenterPort": "443"},
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
	if err := addMachineDriver("pinganyunecs", "https://drivers.rancher.cn/node-driver-pinganyun/0.3.0/docker-machine-driver-pinganyunecs-linux.tgz",
		"https://drivers.rancher.cn/node-driver-pinganyun/0.3.0/component.js", "f84ccec11c2c1970d76d30150916933efe8ca49fe4c422c8954fc37f71273bb5",
		[]string{"drivers.rancher.cn"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("aliyunecs", "https://drivers.rancher.cn/node-driver-aliyun/1.0.4/docker-machine-driver-aliyunecs.tgz",
		"", "5990d40d71c421a85563df9caf069466f300cd75723effe4581751b0de9a6a0e", []string{"ecs.aliyuncs.com"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(Amazonec2driver, "local://", "", "",
		[]string{"iam.amazonaws.com", "iam.%.amazonaws.com.cn", "ec2.%.amazonaws.com", "ec2.%.amazonaws.com.cn", "kms.%.amazonaws.com", "kms.%.amazonaws.com.cn"},
		true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver(Azuredriver, "local://", "", "", nil, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudca", "https://github.com/cloud-ca/docker-machine-driver-cloudca/files/2446837/docker-machine-driver-cloudca_v2.0.0_linux-amd64.zip",
		"https://objects-east.cloud.ca/v1/5ef827605f884961b94881e928e7a250/ui-driver-cloudca/v2.1.2/component.js", "2a55efd6d62d5f7fd27ce877d49596f4",
		[]string{"objects-east.cloud.ca"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(DigitalOceandriver, "local://", "", "", []string{"api.digitalocean.com"}, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver(ExoscaleDriver, "local://", "", "", []string{"api.exoscale.ch"}, false, true, management); err != nil {
		return err
	}
	linodeBuiltin := true
	if dl := os.Getenv("CATTLE_DEV_MODE"); dl != "" {
		linodeBuiltin = false
	}
	if err := addMachineDriver(Linodedriver, "https://github.com/linode/docker-machine-driver-linode/releases/download/v0.1.8/docker-machine-driver-linode_linux-amd64.zip",
		"/assets/rancher-ui-driver-linode/component.js", "b31b6a504c59ee758d2dda83029fe4a85b3f5601e22dfa58700a5e6c8f450dc7", []string{"api.linode.com"}, linodeBuiltin, linodeBuiltin, management); err != nil {
		return err
	}
	if err := addMachineDriver(OCIDriver, "https://github.com/rancher-plugins/rancher-machine-driver-oci/releases/download/v1.0.1/docker-machine-driver-oci-linux",
		"", "6867f59e9f33bdbce34b5bf9476c48d2edc2ef4bca8a2ef82ccaa1de57af811c", []string{"*.oraclecloud.com"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OpenstackDriver, "local://", "", "", nil, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver(OTCDriver, "https://github.com/rancher-plugins/docker-machine-driver-otc/releases/download/v2019.5.7/docker-machine-driver-otc",
		"", "3f793ebb0ebd9477b9166ec542f77e25", nil, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(PacketDriver, "https://github.com/packethost/docker-machine-driver-packet/releases/download/v0.2.2/docker-machine-driver-packet_linux-amd64.zip",
		"https://packethost.github.io/ui-driver-packet/1.0.2/component.js", "e03c6bc9406c811e03e9bc2c39f43e6cc8c02d1615bd0e0b8ee1b38be6fe201c", []string{"api.packet.net", "packethost.github.io"}, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(RackspaceDriver, "local://", "", "", nil, false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver(SoftLayerDriver, "local://", "", "", nil, false, true, management); err != nil {
		return err
	}
	return addMachineDriver(Vmwaredriver, "local://", "", "", nil, true, true, management)
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
	defaults := []string{}
	for key, val := range driverDefaults[name] {
		defaults = append(defaults, fmt.Sprintf("%s:%s", key, val))
	}
	if len(defaults) > 0 {
		annotations["defaults"] = strings.Join(defaults, ",")
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
