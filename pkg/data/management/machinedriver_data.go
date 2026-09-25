package management

import (
	"fmt"
	"maps"
	"reflect"
	"runtime"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/drivers/nodedriver"
	"github.com/rancher/rancher/pkg/data/management/driverdata"
	"github.com/rancher/rancher/pkg/features"
	normanv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AlibabaDriver      = driverdata.AlibabaDriver
	Amazonec2driver    = driverdata.Amazonec2driver
	Azuredriver        = driverdata.Azuredriver
	DigitalOceandriver = driverdata.DigitalOceandriver
	ExoscaleDriver     = driverdata.ExoscaleDriver
	HarvesterDriver    = driverdata.HarvesterDriver
	Linodedriver       = driverdata.Linodedriver
	NutanixDriver      = driverdata.NutanixDriver
	OCIDriver          = driverdata.OCIDriver
	OTCDriver          = driverdata.OTCDriver
	OpenstackDriver    = driverdata.OpenstackDriver
	PacketDriver       = driverdata.PacketDriver
	PhoenixNAPDriver   = driverdata.PhoenixNAPDriver
	PodDriver          = driverdata.PodDriver
	RackspaceDriver    = driverdata.RackspaceDriver
	SoftLayerDriver    = driverdata.SoftLayerDriver
	Vmwaredriver       = driverdata.Vmwaredriver
	GoogleDriver       = driverdata.GoogleDriver
	OutscaleDriver     = driverdata.OutscaleDriver
)

type DriverDataConfig = driverdata.DriverDataConfig

var DriverData = driverdata.DriverData

func addMachineDrivers(management *config.ManagementContext) error {
	if err := addMachineDriver(Amazonec2driver, "local://", "", "",
		[]string{
			// https://docs.aws.amazon.com/general/latest/gr/iam-service.html
			"iam.amazonaws.com",
			"iam.us-gov.amazonaws.com",
			"iam.%.amazonaws.com.cn",
			// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_dual-stack_endpoint_support.html
			"iam.global.api.aws",
			// https://docs.aws.amazon.com/general/latest/gr/ec2-service.html
			"ec2.%.amazonaws.com",
			"ec2.%.amazonaws.com.cn",
			"ec2.%.api.aws",
			// https://docs.aws.amazon.com/general/latest/gr/eks.html
			"eks.%.amazonaws.com",
			"eks.%.amazonaws.com.cn",
			"eks.%.api.aws",
			// https://docs.aws.amazon.com/general/latest/gr/kms.html
			"kms.%.amazonaws.com",
			"kms.%.amazonaws.com.cn",
			"kms.%.api.aws",
		},
		true, true, true, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(Azuredriver, "local://", "", "", nil, true, true, true, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudca", "https://github.com/cloud-ca/docker-machine-driver-cloudca/files/2446837/docker-machine-driver-cloudca_v2.0.0_linux-amd64.zip", "", "1757e2b807b0fea4a38eade4e961bcc609853986a44a99a4e86a37ccea1f7679", []string{"objects-east.cloud.ca"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver("cloudscale", "https://github.com/cloudscale-ch/docker-machine-driver-cloudscale/releases/download/v1.2.0/docker-machine-driver-cloudscale_v1.2.0_linux_amd64.tar.gz", "", "e33fbd6c2f87b1c470bcb653cc8aa50baf914a9d641a2f18f86a07c398cfb544", []string{"objects.rma.cloudscale.ch"}, false, false, false, nil, management); err != nil {
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

	linodeDriverURL := fmt.Sprintf(
		"https://github.com/linode/docker-machine-driver-linode/releases/download/v0.1.16/docker-machine-driver-linode_linux-%s.zip",
		runtime.GOARCH,
	)
	linodeDriverChecksum := "d9c3b8c389a022b0e5c5e8912496c18a673fa74bb52ec6ab51c0a93e0f4de06d"
	if runtime.GOARCH == "arm64" {
		linodeDriverChecksum = "1a8336f66bffc2186f5fb77642f509b9370f177397ee71f1a8cb522e5979dbec"
	}
	// The linode docker-machine driver binary is no longer shipped inside the
	// rancher image; it is downloaded from upstream on first activation.
	//
	// createActive=false: fresh installations land with the driver inactive.
	// updateActive=nil:   the "disable if unused" decision is made once per
	//   Rancher installation by disableUnusedLinodeNodeDriver, which runs early
	//   in rancher.New() before informers start. After that the admin's choice
	//   is preserved across restarts.
	if err := addMachineDriver(Linodedriver, linodeDriverURL, "", linodeDriverChecksum, []string{"api.linode.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OCIDriver, "https://github.com/rancher-plugins/rancher-machine-driver-oci/releases/download/v1.3.0/docker-machine-driver-oci-linux", "", "0a1afa6a0af85ecf3d77cc554960e36e1be5fd12b22b0155717b9289669e4021", []string{"*.oraclecloud.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OpenstackDriver, "local://", "", "", nil, false, true, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OTCDriver, "https://otc-rancher.obs.eu-de.otc.t-systems.com/node/driver/0.3.3/docker-machine-driver-otc_0.3.3_linux_amd64.tar.gz", "", "2151670a96e3ee71aedcfb5a1b73804a4772c3f9ca7f714f2a572d3868a648d1", []string{"*.otc.t-systems.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(PacketDriver, "https://github.com/equinix/docker-machine-driver-metal/releases/download/v0.6.0/docker-machine-driver-metal_linux-amd64.zip", "", "fad5e551a35d2ef2db742b07ca6d61bb9c9b574d322d3000f0c557d5fb90a734", []string{"api.packet.net", "api.equinix.com", "rancher-drivers.equinixmetal.net"}, false, false, false, nil, management); err != nil {
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
	if err := addMachineDriver(NutanixDriver, "https://github.com/nutanix/docker-machine/releases/download/v3.8.0/docker-machine-driver-nutanix", "", "ad18a0150f37e6cf0aef99a529f9541be72daa9e6dc3dad835d095d44e600f6a", []string{"nutanix.github.io"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(OutscaleDriver, "https://github.com/outscale/docker-machine-driver-outscale/releases/download/v0.2.0/docker-machine-driver-outscale_0.2.0_linux_amd64.zip", "", "bb539ed4e2b0f1a1083b29cbdbab59bde3efed0a3145fefc0b2f47026c48bfe0", []string{"oos.eu-west-2.outscale.com"}, false, false, false, nil, management); err != nil {
		return err
	}
	if err := addMachineDriver(Vmwaredriver, "local://", "", "", nil, true, true, false, nil, management); err != nil {
		return err
	}

	deleteMachineDriver("pinganyunecs", "https://drivers.rancher.cn", management.Management.NodeDrivers(""))
	deleteMachineDriver("aliyunecs", "https://drivers.rancher.cn", management.Management.NodeDrivers(""))

	return nil
}

func AddHarvesterMachineDriver(mgmt *config.ManagementContext) error {
	// make sure the version number is consistent with the one at Line 40 of package/Dockerfile
	harvesterDriverVersion := "v1.0.9"
	harvesterDriverChecksums := map[string]string{
		"amd64": "159ec4d8c0d9dae8e322e7ddff64c0a5837c3dc209ae2e345128d4d5cb32279d",
		"arm64": "32a2d906bae047884c30dfd5370f83458edfc77e016b631dcdd46c9f98610951",
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
	annotations := getAnnotations(m, name)

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

// getAnnotations returns a merged set of annotations combining the given NodeDriver's annotations
// with driver-specific credential metadata derived from DriverData.
func getAnnotations(nodeDriver *v3.NodeDriver, driverName string) map[string]string {
	annotations := map[string]string{}
	if nodeDriver != nil {
		maps.Copy(annotations, nodeDriver.Annotations)
	}

	fields, exists := DriverData[driverName]
	if !exists {
		return annotations
	}

	// Helper: formats map[string]string to key:value comma-separated string
	formatMap := func(mp map[string]string) string {
		pairs := make([]string, 0, len(mp))
		for k, v := range mp {
			pairs = append(pairs, fmt.Sprintf("%s:%s", k, v))
		}
		return strings.Join(pairs, ",")
	}

	for key, val := range map[string][]string{
		"publicCredentialFields":   fields.PublicCredentialFields,
		"privateCredentialFields":  fields.PrivateCredentialFields,
		"optionalCredentialFields": fields.OptionalCredentialFields,
		"passwordFields":           fields.PasswordFields,
	} {
		if len(val) > 0 {
			annotations[key] = strings.Join(val, ",")
		}
	}

	if len(fields.Defaults) > 0 {
		annotations["defaults"] = formatMap(fields.Defaults)
	}
	if len(fields.FileToFieldAliases) > 0 {
		annotations[nodedriver.FileToFieldAliasesAnno] = formatMap(fields.FileToFieldAliases)
	}

	return annotations
}

// Delete a deprecated or invalid node driver. Don't return errors to avoid affecting
// Rancher's startup, as the driver will be removed on the next restart.
func deleteMachineDriver(name, urlPrefix string, ndClient normanv3.NodeDriverInterface) {
	driver, err := ndClient.Get(name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			logrus.Warnf("Error getting node driver %s for deletion: %v", name, err)
		}
		return
	}

	// Don't delete if the driver is active or if the url is not the expected invalid one,
	// as it was likely modified.
	if driver.Spec.Active || !strings.HasPrefix(driver.Spec.URL, urlPrefix) {
		logrus.Infof("Not deleting active or modified node driver %s", name)
		return
	}

	logrus.Infof("Deleting node driver %s", name)
	if err := ndClient.Delete(name, &v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		logrus.Warnf("Error deleting node driver %s: %v", name, err)
	}
}
