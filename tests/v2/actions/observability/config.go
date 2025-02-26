package observability

type StackStateConfig struct {
	ServiceToken   string `json:"serviceToken" yaml:"serviceToken"`
	Url            string `json:"url" yaml:"url"`
	ClusterApiKey  string `json:"clusterApiKey" yaml:"clusterApiKey"`
	UpgradeVersion string `json:"upgradeVersion" yaml:"upgradeVersion"`
	License        string `json:"license" yaml:"license"`
	AdminPassword  string `json:"adminPassword" yaml:"adminPassword"`
}

// GlobalConfig represents global configuration values
type GlobalConfig struct {
	ImageRegistry string `json:"imageRegistry" yaml:"imageRegistry"`
}

// AuthenticationConfig represents the authentication configuration
type AuthenticationConfig struct {
	AdminPassword string `json:"adminPassword" yaml:"adminPassword"`
}

// ApiKeyConfig represents API key configuration
type ApiKeyConfig struct {
	Key string `json:"key" yaml:"key"`
}

// LicenseConfig represents the license configuration
type LicenseConfig struct {
	Key string `json:"key" yaml:"key"`
}

// StackstateServerConfig groups the various StackState configuration options
type StackstateServerConfig struct {
	BaseUrl        string               `json:"baseUrl" yaml:"baseUrl"`
	Authentication AuthenticationConfig `json:"authentication" yaml:"authentication"`
	ApiKey         ApiKeyConfig         `json:"apiKey" yaml:"apiKey"`
	License        LicenseConfig        `json:"license" yaml:"license"`
}

// BaseConfig represents the base configuration values
type BaseConfig struct {
	Global     GlobalConfig           `json:"global" yaml:"global"`
	Stackstate StackstateServerConfig `json:"stackstate" yaml:"stackstate"`
}

// ResourcesConfig defines common CPU and Memory configurations for Requests and Limits
type ResourcesConfig struct {
	CPU    string `json:"cpu" yaml:"cpu"`
	Memory string `json:"memory" yaml:"memory"`
}

// PersistenceConfig defines common persistence configurations like size
type PersistenceConfig struct {
	Size string `json:"size" yaml:"size"`
}

// SizingConfig represents the sizing configuration values
type SizingConfig struct {
	Clickhouse struct {
		ReplicaCount int               `json:"replicaCount" yaml:"replicaCount"`
		Persistence  PersistenceConfig `json:"persistence" yaml:"persistence"`
	} `json:"clickhouse" yaml:"clickhouse"`

	Elasticsearch struct {
		ExporterResources  ResourcesConfig `json:"prometheusElasticsearchExporterResources" yaml:"prometheusElasticsearchExporterResources"`
		MinimumMasterNodes int             `json:"minimumMasterNodes" yaml:"minimumMasterNodes"`
		Replicas           int             `json:"replicas" yaml:"replicas"`
		EsJavaOpts         string          `json:"esJavaOpts" yaml:"esJavaOpts"`
		Resources          struct {
			Requests ResourcesConfig `json:"requests" yaml:"requests"`
			Limits   ResourcesConfig `json:"limits" yaml:"limits"`
		} `json:"resources" yaml:"resources"`
		VolumeClaimTemplate struct {
			Requests struct {
				Storage string `json:"storage" yaml:"storage"`
			} `json:"requests" yaml:"requests"`
		} `json:"volumeClaimTemplate" yaml:"volumeClaimTemplate"`
	} `json:"elasticsearch" yaml:"elasticsearch"`

	Hbase struct {
		Version    string `json:"version" yaml:"version"`
		Deployment struct {
			Mode string `json:"mode" yaml:"mode"`
		} `json:"deployment" yaml:"deployment"`
		Stackgraph struct {
			Persistence PersistenceConfig `json:"persistence" yaml:"persistence"`
			Resources   struct {
				Requests ResourcesConfig `json:"requests" yaml:"requests"`
				Limits   ResourcesConfig `json:"limits" yaml:"limits"`
			} `json:"resources" yaml:"resources"`
		} `json:"stackgraph" yaml:"stackgraph"`
		Tephra struct {
			Resources struct {
				Requests ResourcesConfig `json:"requests" yaml:"requests"`
				Limits   ResourcesConfig `json:"limits" yaml:"limits"`
			} `json:"resources" yaml:"resources"`
			ReplicaCount int `json:"replicaCount" yaml:"replicaCount"`
		} `json:"tephra" yaml:"tephra"`
	} `json:"hbase" yaml:"hbase"`

	Kafka struct {
		DefaultReplicationFactor             int `json:"defaultReplicationFactor" yaml:"defaultReplicationFactor"`
		OffsetsTopicReplicationFactor        int `json:"offsetsTopicReplicationFactor" yaml:"offsetsTopicReplicationFactor"`
		ReplicaCount                         int `json:"replicaCount" yaml:"replicaCount"`
		TransactionStateLogReplicationFactor int `json:"transactionStateLogReplicationFactor" yaml:"transactionStateLogReplicationFactor"`
		Resources                            struct {
			Requests ResourcesConfig `json:"requests" yaml:"requests"`
			Limits   ResourcesConfig `json:"limits" yaml:"limits"`
		} `json:"resources" yaml:"resources"`
		Persistence PersistenceConfig `json:"persistence" yaml:"persistence"`
	} `json:"kafka" yaml:"kafka"`

	Stackstate struct {
		Experimental struct {
			Server struct {
				Split bool `json:"split" yaml:"split"`
			} `json:"server" yaml:"server"`
		} `json:"experimental" yaml:"experimental"`
		Components struct {
			All struct {
				ExtraEnv struct {
					Open map[string]string `json:"open" yaml:"open"` // Simplified with a map
				} `json:"extraEnv" yaml:"extraEnv"`
			} `json:"all" yaml:"all"`
			Server struct {
				ExtraEnv struct {
					Open map[string]string `json:"open" yaml:"open"` // Simplified with a map
				} `json:"extraEnv" yaml:"extraEnv"`
				Resources struct {
					Limits   ResourcesConfig `json:"limits" yaml:"limits"`
					Requests ResourcesConfig `json:"requests" yaml:"requests"`
				} `json:"resources" yaml:"resources"`
			} `json:"server" yaml:"server"`
		} `json:"components" yaml:"components"`
	} `json:"stackstate" yaml:"stackstate"`
}

type IngressConfig struct {
	Ingress Ingress `yaml:"ingress"`
}

type Ingress struct {
	Enabled     bool              `yaml:"enabled"`
	Annotations map[string]string `yaml:"annotations"`
	Hosts       []Host            `yaml:"hosts"`
	TLS         []TLSConfig       `yaml:"tls"`
}

type Host struct {
	Host string `yaml:"host"`
}

type TLSConfig struct {
	Hosts      []string `yaml:"hosts"`
	SecretName string   `yaml:"secretName"`
}
