package management

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
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
	OTCDriver:          {"publicCredentialFields": []string{"accessKey", "username"}, "privateCredentialFields": []string{"secretKey", "password", "token"}},
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
	if err := addMachineDriver("cloudca", "https://github.com/cloud-ca/docker-machine-driver-cloudca/files/2446837/docker-machine-driver-cloudca_v2.0.0_linux-amd64.zip", "https://objects-east.cloud.ca/v1/5ef827605f884961b94881e928e7a250/ui-driver-cloudca/v2.1.2/component.js", "1757e2b807b0fea4a38eade4e961bcc609853986a44a99a4e86a37ccea1f7679", []string{"objects-east.cloud.ca"}, false, false, false, management); err != nil {
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
	harvesterDriverVersion := "v0.7.2"
	harvesterDriverURL := fmt.Sprintf("https://github.com/harvester/docker-machine-driver-harvester/releases/download/%s/docker-machine-driver-harvester-%s.tar.gz", harvesterDriverVersion, runtime.GOARCH)
	harvesterDriverCheckSum := "25515420d8205067d8e27b0eb9440e70374d413b2830842ed2b531b48a4d14c0"
	if runtime.GOARCH == "arm64" {
		//overwrite arm driver version here
		harvesterDriverCheckSum = "ad21f6f94695d60b095d4e69afaeb9b59ae649b5d788106dc8d9580f86c2c870"
	}
	if err := addMachineDriver(HarvesterDriver, harvesterDriverURL, "", harvesterDriverCheckSum, []string{"github.com"}, harvesterEnabled, harvesterEnabled, false, management); err != nil {
		return err
	}
	linodeBuiltin := true
	if dl := os.Getenv("CATTLE_DEV_MODE"); dl != "" {
		linodeBuiltin = isCommandAvailable("docker-machine-driver-linode")
	}
	linodeDriverURL := fmt.Sprintf("https://github.com/linode/docker-machine-driver-linode/releases/download/v0.1.12/docker-machine-driver-linode_linux-%s.zip", runtime.GOARCH)
	linodeDriverChecksum := "5fab97320e3965607340567b11857e76a2d9d87fe6dbb3571cc3df04db432c14"
	if runtime.GOARCH == "arm64" {
		//overwrite arm driver version here
		linodeDriverChecksum = "1d4cc22b5ffc9cb47446905e8e9303ad2043dea471f07f1ef16f255e4b738044"
	}
	if err := addMachineDriver(Linodedriver, linodeDriverURL, "/assets/rancher-ui-driver-linode/component.js", linodeDriverChecksum, []string{"api.linode.com"}, linodeBuiltin, linodeBuiltin, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OCIDriver, "https://github.com/rancher-plugins/rancher-machine-driver-oci/releases/download/v1.3.0/docker-machine-driver-oci-linux", "", "0a1afa6a0af85ecf3d77cc554960e36e1be5fd12b22b0155717b9289669e4021", []string{"*.oraclecloud.com"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OpenstackDriver, "local://", "", "", nil, false, true, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(OTCDriver, "https://otc-rancher.obs.eu-de.otc.t-systems.com/node/driver/0.3.3/docker-machine-driver-otc_0.3.3_linux_amd64.tar.gz", "https://otc-rancher.obs.eu-de.otc.t-systems.com/node/ui/1.0.2/component.js", "2151670a96e3ee71aedcfb5a1b73804a4772c3f9ca7f714f2a572d3868a648d1", []string{"*.otc.t-systems.com"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(PacketDriver, "https://github.com/equinix/docker-machine-driver-metal/releases/download/v0.6.0/docker-machine-driver-metal_linux-amd64.zip", "https://rancher-drivers.equinixmetal.net/1.0.2/component.js", "fad5e551a35d2ef2db742b07ca6d61bb9c9b574d322d3000f0c557d5fb90a734", []string{"api.packet.net", "api.equinix.com", "rancher-drivers.equinixmetal.net"}, false, false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver(PhoenixNAPDriver, "https://github.com/phoenixnap/docker-machine-driver-pnap/releases/download/v0.5.1/docker-machine-driver-pnap_0.5.1_linux_amd64.zip", "", "5847599c24c137975fd190747a15ad4db9529ae35059b60e5f8bc9a470d55229", []string{"api.securedservers.com", "api.phoenixnap.com", "auth.phoenixnap.com"}, false, false, false, management); err != nil {
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
