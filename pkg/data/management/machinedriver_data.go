package management

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
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

func addMachineDrivers(management *config.ManagementContext) error {
	if err := addMachineDriver("pinganyunecs", "https://drivers.rancher.cn/node-driver-pinganyun/0.3.0/docker-machine-driver-pinganyunecs-linux.tgz", "https://drivers.rancher.cn/node-driver-pinganyun/0.3.0/component.js", "f84ccec11c2c1970d76d30150916933efe8ca49fe4c422c8954fc37f71273bb5", []string{"drivers.rancher.cn"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver("aliyunecs", "https://drivers.rancher.cn/node-driver-aliyun/1.0.4/docker-machine-driver-aliyunecs.tgz", "", "5990d40d71c421a85563df9caf069466f300cd75723effe4581751b0de9a6a0e", []string{"ecs.aliyuncs.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(Amazonec2driver, "local://", "", "",
		[]string{"iam.amazonaws.com", "iam.us-gov.amazonaws.com", "iam.%.amazonaws.com.cn", "ec2.%.amazonaws.com", "ec2.%.amazonaws.com.cn", "eks.%.amazonaws.com", "eks.%.amazonaws.com.cn", "kms.%.amazonaws.com", "kms.%.amazonaws.com.cn"},
		true, true, true, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(Azuredriver, "local://", "", "", nil, true, true, true, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudca", "https://github.com/cloud-ca/docker-machine-driver-cloudca/files/2446837/docker-machine-driver-cloudca_v2.0.0_linux-amd64.zip", "https://objects-east.cloud.ca/v1/5ef827605f884961b94881e928e7a250/ui-driver-cloudca/v2.1.2/component.js", "1757e2b807b0fea4a38eade4e961bcc609853986a44a99a4e86a37ccea1f7679", []string{"objects-east.cloud.ca"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudscale", "https://github.com/cloudscale-ch/docker-machine-driver-cloudscale/releases/download/v1.2.0/docker-machine-driver-cloudscale_v1.2.0_linux_amd64.tar.gz", "https://objects.rma.cloudscale.ch/cloudscale-rancher-v2-ui-driver/component.js", "e33fbd6c2f87b1c470bcb653cc8aa50baf914a9d641a2f18f86a07c398cfb544", []string{"objects.rma.cloudscale.ch"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(DigitalOceandriver, "local://", "", "", []string{"api.digitalocean.com"}, true, true, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(ExoscaleDriver, "local://", "", "", []string{"api.exoscale.ch"}, false, true, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(GoogleDriver, "local://", "", "", nil, false, true, true, nil, management); err != nil {
		return err
	}
	if err := AddHarvesterMachineDriver(management); err != nil {
		return err
	}
	linodeBuiltin := true
	if dl := os.Getenv("CATTLE_DEV_MODE"); dl != "" {
		linodeBuiltin = isCommandAvailable("docker-machine-driver-linode")
	}
	linodeDriverURL := fmt.Sprintf("https://github.com/linode/docker-machine-driver-linode/releases/download/v0.1.15/docker-machine-driver-linode_linux-%s.zip", runtime.GOARCH)
	linodeDriverChecksum := "26a71dbbc2f5249a66716cad586c3b4048d3cd9e67c0527442c374dd5dcf1c41"
	if runtime.GOARCH == "arm64" {
		//overwrite arm driver version here
		linodeDriverChecksum = "3b1ed74291cbf581c0f8a63d878d79e1fe3b443387c1c0bb8b1d078a78db8bc4"
	}
	if err := addMachineDriver(Linodedriver, linodeDriverURL, "/assets/rancher-ui-driver-linode/component.js", linodeDriverChecksum, []string{"api.linode.com"}, linodeBuiltin, linodeBuiltin, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OCIDriver, "https://github.com/rancher-plugins/rancher-machine-driver-oci/releases/download/v1.3.0/docker-machine-driver-oci-linux", "", "0a1afa6a0af85ecf3d77cc554960e36e1be5fd12b22b0155717b9289669e4021", []string{"*.oraclecloud.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OpenstackDriver, "local://", "", "", nil, false, true, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OTCDriver, "https://otc-rancher.obs.eu-de.otc.t-systems.com/node/driver/0.3.3/docker-machine-driver-otc_0.3.3_linux_amd64.tar.gz", "https://otc-rancher.obs.eu-de.otc.t-systems.com/node/ui/1.0.2/component.js", "2151670a96e3ee71aedcfb5a1b73804a4772c3f9ca7f714f2a572d3868a648d1", []string{"*.otc.t-systems.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(PacketDriver, "https://github.com/equinix/docker-machine-driver-metal/releases/download/v0.6.0/docker-machine-driver-metal_linux-amd64.zip", "https://rancher-drivers.equinixmetal.net/1.0.2/component.js", "fad5e551a35d2ef2db742b07ca6d61bb9c9b574d322d3000f0c557d5fb90a734", []string{"api.packet.net", "api.equinix.com", "rancher-drivers.equinixmetal.net"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(PhoenixNAPDriver, "https://github.com/phoenixnap/docker-machine-driver-pnap/releases/download/v0.5.1/docker-machine-driver-pnap_0.5.1_linux_amd64.zip", "", "5847599c24c137975fd190747a15ad4db9529ae35059b60e5f8bc9a470d55229", []string{"api.securedservers.com", "api.phoenixnap.com", "auth.phoenixnap.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(RackspaceDriver, "local://", "", "", nil, false, true, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(SoftLayerDriver, "local://", "", "", nil, false, true, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(NutanixDriver, "https://github.com/nutanix/docker-machine/releases/download/v3.6.0/docker-machine-driver-nutanix", "https://nutanix.github.io/rancher-ui-driver/v3.6.0/component.js", "d9710fe31a1357d1bbd57539a4b0b00e3ab3550fcaeffea18cbc145cb4e9b22f", []string{"nutanix.github.io"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OutscaleDriver, "https://github.com/outscale/docker-machine-driver-outscale/releases/download/v0.2.0/docker-machine-driver-outscale_0.2.0_linux_amd64.zip", "https://oos.eu-west-2.outscale.com/rancher-ui-driver-outscale/v0.2.0/component.js", "bb539ed4e2b0f1a1083b29cbdbab59bde3efed0a3145fefc0b2f47026c48bfe0", []string{"oos.eu-west-2.outscale.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	return addMachineDriver(Vmwaredriver, "local://", "", "", nil, true, true, false, nil, management)
}

func AddHarvesterMachineDriver(mgmt *config.ManagementContext) error {
	// make sure the version number is consistent with the one at Line 40 of package/Dockerfile
	harvesterDriverVersion := "v1.0.2"
	harvesterDriverChecksums := map[string]string{
		"amd64": "a238768f343e9f6fa2403219a02311f9d52a5b72d39bf74a6ecd99e90d6f5c4c",
		"arm64": "4c18edc9e9fcaa4870481efc4d218a6be69e35a4f9243896ca3a1e3b7dcdd789",
	}

	harvesterDriverURL := fmt.Sprintf("https://github.com/harvester/docker-machine-driver-harvester/releases/download/%s/docker-machine-driver-harvester-%s.tar.gz",
		harvesterDriverVersion,
		runtime.GOARCH,
	)

	harvesterEnabled := features.GetFeatureByName(HarvesterDriver).Enabled()

	harvesterDriverChecksum, ok := harvesterDriverChecksums[runtime.GOARCH]
	if !ok {
		logrus.Warnf("machine driver %v does not support GOARCH %v", HarvesterDriver, runtime.GOARCH)
		harvesterEnabled = false
	}

	return addMachineDriver(
		HarvesterDriver,
		harvesterDriverURL,
		"",
		harvesterDriverChecksum,
		[]string{"github.com"},
		harvesterEnabled,
		harvesterEnabled,
		false,
		&harvesterEnabled,
		mgmt,
	)
}

func addMachineDriver(name, url, uiURL, checksum string, whitelist []string, createActive, builtin,
	addCloudCredential bool, updateActive *bool, management *config.ManagementContext) error {
	cli := management.Management.NodeDrivers("")
	lister := cli.Controller().Lister()
	m, err := lister.Get("", name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
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
		n := m.DeepCopy()
		if updateActive != nil {
			n.Spec.Active = *updateActive
		}
		n.Annotations = annotations
		n.Spec.Builtin = builtin
		n.Spec.AddCloudCredential = addCloudCredential
		n.Spec.URL = url
		n.Spec.UIURL = uiURL
		n.Spec.Checksum = checksum
		n.Spec.DisplayName = name
		n.Spec.WhitelistDomains = whitelist
		if !reflect.DeepEqual(m, n) {
			logrus.Infof("Updating node driver %v", name)
			_, err := cli.Update(n)
			return err
		}
		return nil
	}

	logrus.Infof("Creating node driver %v", name)
	_, err = cli.Create(&v3.NodeDriver{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: v3.NodeDriverSpec{
			Active:             createActive,
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
