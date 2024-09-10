package pipeline

import (
	"os"

	"github.com/rancher/rancher/tests/v2/actions/upgradeinput"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// ReleaseUpgradeConfigKey is the key name of ReleaseUpgradeConfig values in the cattle config
const ReleaseUpgradeConfigKey = "releaseUpgrade"

// ReleaseUpgradeConfig is a struct that contains:
//   - MainConfig, which is the embedded yaml fields, and built on top of it.
//   - HA and Clusters inputs.
//   - Provisioning and upgrade test cases and packages.
type ReleaseUpgradeConfig struct {
	//metada configs
	HAConfig HAConfig `yaml:"ha"`
	Clusters Clusters `yaml:"clusters"`

	//test case configs
	TestCases TestCases `yaml:"testCases"`
}

// TestCasesConfigKey is the key name of TestCases values in the cattle config
const TestCasesConfigKey = "testCases"

// TestCases is a struct that contains related information about the required package and run test strings to run go test commands.
type TestCases struct {
	//provisioning test cases
	ProvisioningTestPackage string `yaml:"provisioningTestPackage" json:"provisioningTestPackage"`
	ProvisioningTestCase    string `yaml:"provisioningTestCase" json:"provisioningTestCase"`

	//upgrade test cases
	UpgradeTestPackage        string `yaml:"upgradeTestCase" json:"upgradeTestCase"`
	UpgradeKubernetesTestCase string `yaml:"upgradeKubernetesTestCase" json:"upgradeKubernetesTestCase"`
	PreUpgradeTestCase        string `yaml:"preUpgradeTestCase" json:"preUpgradeTestCase"`
	PostUpgradeTestCase       string `yaml:"postUpgradeTestCase" json:"postUpgradeTestCase"`

	//validation tag
	Tags string `yaml:"tags" json:"tags"`

	//validation test run flag
	RunFlag string `yaml:"runFlag" json:"runFlag"`
}

// HAConfigKey is the key name of HAConfig values in the cattle config
const HAConfigKey = "ha"

// HAConfig is a struct that contains related information about the HA that's going to be created and upgraded.
type HAConfig struct {
	Host                       string `yaml:"host" json:"host"`
	ChartVersion               string `yaml:"chartVersion" json:"chartVersion"`
	ChartVersionToUpgrade      string `yaml:"chartVersionToUpgrade" json:"chartVersionToUpgrade"`
	ImageTag                   string `yaml:"imageTag" json:"imageTag"`
	ImageTagToUpgrade          string `yaml:"imageTagToUpgrade" json:"imageTagToUpgrade"`
	CertOption                 string `yaml:"certOption" json:"certOption"`
	Insecure                   *bool  `yaml:"insecure" json:"insecure" default:"true"`
	Cleanup                    *bool  `yaml:"cleanup" json:"cleanup" default:"true"`
	HelmExtraSettings          string `yaml:"helmExtraSettings" json:"helmExtraSettings"`
	HelmExtraSettingsToUpgrade string `yaml:"helmExtraSettingsToUpgrade" json:"helmExtraSettingsToUpgrade"`
	HelmRepoURL                string `yaml:"helmURL" json:"helmURL"`
	HelmRepoURLToUpgrade       string `yaml:"helmURLToUpgrade" json:"helmURLToUpgrade"`
}

// ClustersConfigKey is the key name of Clusters values in the cattle config
const ClustersConfigKey = "clusters"

// Clusters is a struct that contains cluster types.
type Clusters struct {
	Local  *RancherCluster `yaml:"local" json:"local"`
	RKE1   RancherClusters `yaml:"rke1" json:"rke1"`
	RKE2   RancherClusters `yaml:"rke2" json:"rke2"`
	K3s    RancherClusters `yaml:"k3s" json:"k3s"`
	Hosted []HostedCluster `yaml:"hosted" json:"hosted"`
}

// RancherClusters is a struct that contains slice of custom and node providers as ProviderCluster type.
type RancherClusters struct {
	Custom       []RancherCluster `yaml:"custom" json:"custom"`
	NodeProvider []RancherCluster `yaml:"nodeProvider" json:"nodeProvider"`
}

// RancherCluster is a struct that contains related information about the downstream cluster that's going to be created and upgraded.
type RancherCluster struct {
	Provider                   string                `yaml:"provider" json:"provider"`
	KubernetesVersion          string                `yaml:"kubernetesVersion" json:"kubernetesVersion"`
	KubernetesVersionToUpgrade string                `yaml:"kubernetesVersionToUpgrade" json:"kubernetesVersionToUpgrade"`
	Image                      string                `yaml:"image" json:"image"`
	CNIs                       []string              `yaml:"cni" json:"cni"`
	FeaturesToTest             upgradeinput.Features `yaml:"enabledFeatures" json:"enabledFeatures" default:""`
	Tags                       string                `yaml:"tags" json:"tags" default:""`
	RunFlag                    string                `yaml:"runFlag" json:"runFlag" default:""`
	SSHUser                    string                `yaml:"sshUser" json:"sshUser" default:""`
	VolumeType                 string                `yaml:"volumeType" json:"volumeType" default:""`
}

// HostedCluster is a struct that contains related information about the downstream cluster that's going to be created and upgraded.
type HostedCluster struct {
	Provider                   string                `yaml:"provider" json:"provider"`
	KubernetesVersion          string                `yaml:"kubernetesVersion" json:"kubernetesVersion"`
	KubernetesVersionToUpgrade string                `yaml:"kubernetesVersionToUpgrade" json:"kubernetesVersionToUpgrade"`
	FeaturesToTest             upgradeinput.Features `yaml:"enabledFeatures" json:"enabledFeatures" default:""`
}

// GenerateDefaultReleaseUpgradeConfig is a function that creates the ReleaseUpgradeConfig with its default values.
func GenerateDefaultReleaseUpgradeConfig() {
	configFileName := "default-release-upgrade.yaml"

	config := new(ReleaseUpgradeConfig)

	configData, err := yaml.Marshal(&config)
	if err != nil {
		logrus.Fatalf("error marshaling: %v", err)
	}
	err = os.WriteFile(configFileName, configData, 0644)
	if err != nil {
		logrus.Fatalf("error writing yaml: %v", err)
	}
}
