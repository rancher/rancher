package upgradeinput

import "github.com/rancher/rancher/tests/v2/actions/provisioninginput"

type PSACT string

const (
	ConfigurationFileKey = "upgradeInput" // ConfigurationFileKey is used to parse the configuration of upgrade tests.
	localClusterID       = "local"        // localClusterID is a string to used ignore this cluster in comparisons
	LatestKey            = "latest"       // latestKey is a string to determine automatically version pooling to the latest possible
)

// Config is a struct that stores multiple clusters and their testing options to load from the configuration file
type Config struct {
	Clusters []Cluster `json:"clusters" yaml:"clusters" default:"[]"`
}

// Cluster is a struct that's used to configure a single cluster to be used in an upgrade test
type Cluster struct {
	Name              string                   `json:"name" yaml:"name" default:""`
	VersionToUpgrade  string                   `json:"versionToUpgrade" yaml:"versionToUpgrade" default:""`
	FeaturesToTest    Features                 `json:"enabledFeatures" yaml:"enabledFeatures" default:""`
	ProvisioningInput provisioninginput.Config `json:"provisioningInput" yaml:"provisioningInput"`
}

// Features is a struct that stores test case options for a single cluster
type Features struct {
	Chart   *bool `json:"chart" yaml:"chart" default:"false"`
	Ingress *bool `json:"ingress" yaml:"ingress" default:"false"`
}
