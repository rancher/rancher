package observability

const (
	project                 = "management.cattle.io.project"
	rancherPartnerCharts    = "rancher-partner-charts"
	systemProject           = "System"
	localCluster            = "local"
	stackStateConfigFileKey = "stackstateConfigs"
	uiExtensionsRepo        = "https://github.com/rancher/ui-plugin-charts"
	uiGitBranch             = "main"
	rancherUIPlugins        = "rancher-ui-plugins"
)

// BaseConfig represents the base configuration values
type BaseConfig struct {
	Global struct {
		ImageRegistry string `yaml:"imageRegistry"`
	} `yaml:"global"`
	Stackstate struct {
		BaseUrl        string `yaml:"baseUrl"`
		Authentication struct {
			AdminPassword string `yaml:"adminPassword"`
		} `yaml:"authentication"`
		ApiKey struct {
			Key string `yaml:"key"`
		} `yaml:"apiKey"`
		License struct {
			Key string `yaml:"key"`
		} `yaml:"license"`
	} `yaml:"stackstate"`
}

// SizingConfig represents the sizing configuration values
type SizingConfig struct {
	Clickhouse struct {
		ReplicaCount int `yaml:"replicaCount"`
		Persistence  struct {
			Size string `yaml:"size"`
		} `yaml:"persistence"`
	} `yaml:"clickhouse"`
	Elasticsearch struct {
		PrometheusExporter struct {
			Resources Resources `yaml:"resources"`
		} `yaml:"prometheus-elasticsearch-exporter"`
		MinimumMasterNodes  int       `yaml:"minimumMasterNodes"`
		Replicas            int       `yaml:"replicas"`
		EsJavaOpts          string    `yaml:"esJavaOpts"`
		Resources           Resources `yaml:"resources"`
		VolumeClaimTemplate struct {
			Resources struct {
				Requests struct {
					Storage string `yaml:"storage"`
				} `yaml:"requests"`
			} `yaml:"resources"`
		} `yaml:"volumeClaimTemplate"`
	} `yaml:"elasticsearch"`
	HBase struct {
		Version    string `yaml:"version"`
		Deployment struct {
			Mode string `yaml:"mode"`
		} `yaml:"deployment"`
		Stackgraph struct {
			Persistence struct {
				Size string `yaml:"size"`
			} `yaml:"persistence"`
			Resources Resources `yaml:"resources"`
		} `yaml:"stackgraph"`
		Tephra struct {
			Resources    Resources `yaml:"resources"`
			ReplicaCount int       `yaml:"replicaCount"`
		} `yaml:"tephra"`
	} `yaml:"hbase"`
	Kafka struct {
		DefaultReplicationFactor             int       `yaml:"defaultReplicationFactor"`
		OffsetsTopicReplicationFactor        int       `yaml:"offsetsTopicReplicationFactor"`
		ReplicaCount                         int       `yaml:"replicaCount"`
		TransactionStateLogReplicationFactor int       `yaml:"transactionStateLogReplicationFactor"`
		Resources                            Resources `yaml:"resources"`
		Persistence                          struct {
			Size string `yaml:"size"`
		} `yaml:"persistence"`
	} `yaml:"kafka"`
	Stackstate struct {
		Experimental struct {
			Server struct {
				Split bool `yaml:"split"`
			} `yaml:"server"`
		} `yaml:"experimental"`
		Components struct {
			All struct {
				ExtraEnv struct {
					Open map[string]string `yaml:"open"`
				} `yaml:"extraEnv"`
			} `yaml:"all"`
			Server struct {
				ExtraEnv struct {
					Open map[string]string `yaml:"open"`
				} `yaml:"extraEnv"`
				Resources Resources `yaml:"resources"`
			} `yaml:"server"`
			E2ES struct {
				Resources Resources `yaml:"resources"`
			} `yaml:"e2es"`
		} `yaml:"components"`
	} `yaml:"stackstate"`
}

// Resources represents the common resources structure
type Resources struct {
	Limits struct {
		CPU              string `yaml:"cpu"`
		Memory           string `yaml:"memory"`
		EphemeralStorage string `yaml:"ephemeral-storage,omitempty"`
	} `yaml:"limits"`
	Requests struct {
		CPU    string `yaml:"cpu"`
		Memory string `yaml:"memory"`
	} `yaml:"requests"`
}
