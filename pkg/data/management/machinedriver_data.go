package management

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Amazonec2driver    = "amazonec2"
	Azuredriver        = "azure"
	DigitalOceandriver = "digitalocean"
	ExoscaleDriver     = "exoscale"
	HarvesterDriver    = "harvester"
	Linodedriver       = "linode"
	NutanixDriver      = "nutanix"
	OCIDriver          = "oci"
	OTCDriver          = "otc"
	OpenstackDriver    = "openstack"
	PacketDriver       = "packet"
	PhoenixNAPDriver   = "pnap"
	RackspaceDriver    = "rackspace"
	SoftLayerDriver    = "softlayer"
	Vmwaredriver       = "vmwarevsphere"
	GoogleDriver       = "google"
	OutscaleDriver     = "outscale"
)

var DriverData = map[string]map[string][]string{
	Amazonec2driver:    {"publicCredentialFields": []string{"accessKey"}, "privateCredentialFields": []string{"secretKey"}},
	Azuredriver:        {"publicCredentialFields": []string{"clientId", "subscriptionId", "tenantId", "environment"}, "privateCredentialFields": []string{"clientSecret"}, "optionalCredentialFields": []string{"tenantId"}},
	DigitalOceandriver: {"privateCredentialFields": []string{"accessToken"}},
	ExoscaleDriver:     {"privateCredentialFields": []string{"apiSecretKey"}},
	HarvesterDriver:    {"publicCredentialFields": []string{"clusterType", "clusterId"}, "privateCredentialFields": []string{"kubeconfigContent"}, "optionalCredentialFields": []string{"clusterId"}},
	Linodedriver:       {"privateCredentialFields": []string{"token"}, "passwordFields": []string{"rootPass"}},
	NutanixDriver:      {"publicCredentialFields": []string{"endpoint", "username", "port"}, "privateCredentialFields": []string{"password"}},
	OCIDriver:          {"publicCredentialFields": []string{"tenancyId", "userId", "fingerprint"}, "privateCredentialFields": []string{"privateKeyContents"}, "passwordFields": []string{"privateKeyPassphrase"}},
	OTCDriver:          {"privateCredentialFields": []string{"accessKeySecret"}},
	OpenstackDriver:    {"privateCredentialFields": []string{"password"}},
	PacketDriver:       {"privateCredentialFields": []string{"apiKey"}},
	PhoenixNAPDriver:   {"publicCredentialFields": []string{"clientIdentifier"}, "privateCredentialFields": []string{"clientSecret"}},
	RackspaceDriver:    {"privateCredentialFields": []string{"apiKey"}},
	SoftLayerDriver:    {"privateCredentialFields": []string{"apiKey"}},
	Vmwaredriver:       {"publicCredentialFields": []string{"username", "vcenter", "vcenterPort"}, "privateCredentialFields": []string{"password"}},
	GoogleDriver:       {"privateCredentialFields": []string{"authEncodedJson"}},
	OutscaleDriver:     {"publicCredentialFields": []string{"accessKey", "region"}, "privateCredentialFields": []string{"secretKey"}},
}

var driverDefaults = map[string]map[string]string{
	HarvesterDriver: {"clusterType": "imported"},
	Vmwaredriver:    {"vcenterPort": "443"},
}

type machineDriverCompare struct {
	builtin            bool
	addCloudCredential bool
	url                string
	uiURL              string
	checksum           string
	name               string
	whitelist          []string
	annotations        map[string]string
}

func addMachineDrivers(management *config.ManagementContext) error {
	if err := addMachineDriver("pinganyunecs", "https://drivers.rancher.cn/node-driver-pinganyun/0.3.0/docker-machine-driver-pinganyunecs-linux.tgz", "https://drivers.rancher.cn/node-driver-pinganyun/0.3.0/component.js", "f84ccec11c2c1970d76d30150916933efe8ca49fe4c422c8954fc37f71273bb5", []string{"drivers.rancher.cn"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("aliyunecs", "https://drivers.rancher.cn/node-driver-aliyun/1.0.4/docker-machine-driver-aliyunecs.tgz", "", "5990d40d71c421a85563df9caf069466f300cd75723effe4581751b0de9a6a0e", []string{"ecs.aliyuncs.com"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(Amazonec2driver, "local://", "", "",
		[]string{"iam.amazonaws.com", "iam.us-gov.amazonaws.com", "iam.%.amazonaws.com.cn", "ec2.%.amazonaws.com", "ec2.%.amazonaws.com.cn", "eks.%.amazonaws.com", "eks.%.amazonaws.com.cn", "kms.%.amazonaws.com", "kms.%.amazonaws.com.cn"},
		true, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver(Azuredriver, "local://", "", "", nil, true, true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudca", "https://github.com/cloud-ca/docker-machine-driver-cloudca/files/2446837/docker-machine-driver-cloudca_v2.0.0_linux-amd64.zip", "https://objects-east.cloud.ca/v1/5ef827605f884961b94881e928e7a250/ui-driver-cloudca/v2.1.2/component.js", "2a55efd6d62d5f7fd27ce877d49596f4", []string{"objects-east.cloud.ca"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudscale", "https://github.com/cloudscale-ch/docker-machine-driver-cloudscale/releases/download/v1.2.0/docker-machine-driver-cloudscale_v1.2.0_linux_amd64.tar.gz", "https://objects.rma.cloudscale.ch/cloudscale-rancher-v2-ui-driver/component.js", "e33fbd6c2f87b1c470bcb653cc8aa50baf914a9d641a2f18f86a07c398cfb544", []string{"objects.rma.cloudscale.ch"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(DigitalOceandriver, "local://", "", "", []string{"api.digitalocean.com"}, true, true, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(ExoscaleDriver, "local://", "", "", []string{"api.exoscale.ch"}, false, true, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(GoogleDriver, "local://", "", "", nil, false, true, true, management); err != nil {
		return err
	}
	harvesterEnabled := features.GetFeatureByName(HarvesterDriver).Enabled()
	// make sure the version number is consistent with the one at Line 40 of package/Dockerfile
	if err := addMachineDriver(HarvesterDriver, "https://releases.rancher.com/harvester-node-driver/v0.6.7/docker-machine-driver-harvester-amd64.tar.gz", "", "a913f22a7cad5d0df18e0dfa8a8ae19703c36eb8ed4f6a34ae471a06af0890c3", []string{"releases.rancher.com"}, harvesterEnabled, harvesterEnabled, false, management); err != nil {
		return err
	}
	linodeBuiltin := true
	if dl := os.Getenv("CATTLE_DEV_MODE"); dl != "" {
		linodeBuiltin = isCommandAvailable("docker-machine-driver-linode")
	}
	if err := addMachineDriver(Linodedriver, "https://github.com/linode/docker-machine-driver-linode/releases/download/v0.1.11/docker-machine-driver-linode_linux-amd64.zip", "/assets/rancher-ui-driver-linode/component.js", "b31b6a504c59ee758d2dda83029fe4a85b3f5601e22dfa58700a5e6c8f450dc7", []string{"api.linode.com"}, linodeBuiltin, linodeBuiltin, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OCIDriver, "https://github.com/rancher-plugins/rancher-machine-driver-oci/releases/download/v1.3.0/docker-machine-driver-oci-linux", "", "0a1afa6a0af85ecf3d77cc554960e36e1be5fd12b22b0155717b9289669e4021", []string{"*.oraclecloud.com"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OpenstackDriver, "local://", "", "", nil, false, true, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OTCDriver, "https://github.com/rancher-plugins/docker-machine-driver-otc/releases/download/v2019.5.7/docker-machine-driver-otc", "", "3f793ebb0ebd9477b9166ec542f77e25", nil, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(PacketDriver, "https://github.com/equinix/docker-machine-driver-metal/releases/download/v0.6.0/docker-machine-driver-metal_linux-amd64.zip", "https://rancher-drivers.equinixmetal.net/1.0.2/component.js", "fad5e551a35d2ef2db742b07ca6d61bb9c9b574d322d3000f0c557d5fb90a734", []string{"api.packet.net", "api.equinix.com", "rancher-drivers.equinixmetal.net"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(PhoenixNAPDriver, "https://github.com/phoenixnap/docker-machine-driver-pnap/releases/download/v0.4.0/docker-machine-driver-pnap_0.4.0_linux_amd64.zip", "", "0bc81bdc80ab258fa0db67918f3b04435ed2c81f84c942c9123a0729f884190b", []string{"api.securedservers.com", "api.phoenixnap.com", "auth.phoenixnap.com"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(RackspaceDriver, "local://", "", "", nil, false, true, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(SoftLayerDriver, "local://", "", "", nil, false, true, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(NutanixDriver, "https://github.com/nutanix/docker-machine/releases/download/v3.6.0/docker-machine-driver-nutanix", "https://nutanix.github.io/rancher-ui-driver/v3.6.0/component.js", "d9710fe31a1357d1bbd57539a4b0b00e3ab3550fcaeffea18cbc145cb4e9b22f", []string{"nutanix.github.io"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OutscaleDriver, "https://github.com/outscale/docker-machine-driver-outscale/releases/download/v0.2.0/docker-machine-driver-outscale_0.2.0_linux_amd64.zip", "https://oos.eu-west-2.outscale.com/rancher-ui-driver-outscale/v0.2.0/component.js", "bb539ed4e2b0f1a1083b29cbdbab59bde3efed0a3145fefc0b2f47026c48bfe0", []string{"oos.eu-west-2.outscale.com"}, false, false, false, management); err != nil {
		return err
	}
	return addMachineDriver(Vmwaredriver, "local://", "", "", nil, true, true, false, management)
}

func addMachineDriver(name, url, uiURL, checksum string, whitelist []string, active, builtin, addCloudCredential bool, management *config.ManagementContext) error {
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
	for key, fields := range DriverData[name] {
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
			builtin:            m.Spec.Builtin,
			addCloudCredential: m.Spec.AddCloudCredential,
			url:                m.Spec.URL,
			uiURL:              m.Spec.UIURL,
			checksum:           m.Spec.Checksum,
			name:               m.Spec.DisplayName,
			whitelist:          m.Spec.WhitelistDomains,
			annotations:        m.Annotations,
		}
		new := machineDriverCompare{
			builtin:            builtin,
			addCloudCredential: addCloudCredential,
			url:                url,
			uiURL:              uiURL,
			checksum:           checksum,
			name:               name,
			whitelist:          whitelist,
			annotations:        annotations,
		}
		if !reflect.DeepEqual(new, old) {
			logrus.Infof("Updating node driver %v", name)
			m.Spec.Builtin = builtin
			m.Spec.AddCloudCredential = addCloudCredential
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
		Spec: v32.NodeDriverSpec{
			Active:             active,
			Builtin:            builtin,
			AddCloudCredential: addCloudCredential,
			URL:                url,
			UIURL:              uiURL,
			DisplayName:        name,
			Checksum:           checksum,
			WhitelistDomains:   whitelist,
		},
	})

	return err
}

func isCommandAvailable(name string) bool {
	return exec.Command("command", "-v", name).Run() == nil
}
