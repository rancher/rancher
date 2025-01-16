package observability

type StackStateConfig struct {
	ServiceToken   string `json:"serviceToken" yaml:"serviceToken"`
	Url            string `json:"url" yaml:"url"`
	ClusterApiKey  string `json:"clusterApiKey" yaml:"clusterApiKey"`
	UpgradeVersion string `json:"upgradeVersion" yaml:"upgradeVersion"`
}

// BaseConfig represents the base configuration values
type BaseConfig struct {
	Global struct {
		ImageRegistry string `json:"imageRegistry" yaml:"imageRegistry"`
	} `json:"global" yaml:"global"`
	Stackstate struct {
		BaseUrl        string `json:"baseUrl" yaml:"baseUrl"`
		Authentication struct {
			AdminPassword string `json:"adminPassword" yaml:"adminPassword"`
		} `json:"authentication" yaml:"authentication"`
		ApiKey struct {
			Key string `json:"key" yaml:"key"`
		} `json:"apiKey" yaml:"apiKey"`
		License struct {
			Key string `json:"key" yaml:"key"`
		} `json:"license" yaml:"license"`
	} `json:"stackstate" yaml:"stackstate"`
}

// SizingConfig represents the sizing configuration values
type SizingConfig struct {
	Clickhouse struct {
		ReplicaCount int `json:"replicaCount" yaml:"replicaCount"`
		Persistence  struct {
			Size string `json:"size" yaml:"size"`
		} `json:"persistence" yaml:"persistence"`
	} `json:"clickhouse" yaml:"clickhouse"`
	Elasticsearch struct {
		PrometheusElasticsearchExporter struct {
			Resources struct {
				Limits struct {
					CPU    string `json:"cpu" yaml:"cpu"`
					Memory string `json:"memory" yaml:"memory"`
				} `json:"limits" yaml:"limits"`
				Requests struct {
					CPU    string `json:"cpu" yaml:"cpu"`
					Memory string `json:"memory" yaml:"memory"`
				} `json:"requests" yaml:"requests"`
			} `json:"resources" yaml:"resources"`
		} `json:"prometheus-elasticsearch-exporter" yaml:"prometheus-elasticsearch-exporter"`
		MinimumMasterNodes int    `json:"minimumMasterNodes" yaml:"minimumMasterNodes"`
		Replicas           int    `json:"replicas" yaml:"replicas"`
		EsJavaOpts         string `json:"esJavaOpts" yaml:"esJavaOpts"`
		Resources          struct {
			Requests struct {
				CPU    string `json:"cpu" yaml:"cpu"`
				Memory string `json:"memory" yaml:"memory"`
			} `json:"requests" yaml:"requests"`
			Limits struct {
				CPU    string `json:"cpu" yaml:"cpu"`
				Memory string `json:"memory" yaml:"memory"`
			} `json:"limits" yaml:"limits"`
		} `json:"resources" yaml:"resources"`
		VolumeClaimTemplate struct {
			Resources struct {
				Requests struct {
					Storage string `json:"storage" yaml:"storage"`
				} `json:"requests" yaml:"requests"`
			} `json:"resources" yaml:"resources"`
		} `json:"volumeClaimTemplate" yaml:"volumeClaimTemplate"`
	} `json:"elasticsearch" yaml:"elasticsearch"`
	Hbase struct {
		Version    string `json:"version" yaml:"version"`
		Deployment struct {
			Mode string `json:"mode" yaml:"mode"`
		} `json:"deployment" yaml:"deployment"`
		Stackgraph struct {
			Persistence struct {
				Size string `json:"size" yaml:"size"`
			} `json:"persistence" yaml:"persistence"`
			Resources struct {
				Requests struct {
					Memory string `json:"memory" yaml:"memory"`
					CPU    string `json:"cpu" yaml:"cpu"`
				} `json:"requests" yaml:"requests"`
				Limits struct {
					CPU    string `json:"cpu" yaml:"cpu"`
					Memory string `json:"memory" yaml:"memory"`
				} `json:"limits" yaml:"limits"`
			} `json:"resources" yaml:"resources"`
		} `json:"stackgraph" yaml:"stackgraph"`
		Tephra struct {
			Resources struct {
				Limits struct {
					CPU    string `json:"cpu" yaml:"cpu"`
					Memory string `json:"memory" yaml:"memory"`
				} `json:"limits" yaml:"limits"`
				Requests struct {
					Memory string `json:"memory" yaml:"memory"`
					CPU    string `json:"cpu" yaml:"cpu"`
				} `json:"requests" yaml:"requests"`
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
			Requests struct {
				CPU    string `json:"cpu" yaml:"cpu"`
				Memory string `json:"memory" yaml:"memory"`
			} `json:"requests" yaml:"requests"`
			Limits struct {
				Memory string `json:"memory" yaml:"memory"`
				CPU    string `json:"cpu" yaml:"cpu"`
			} `json:"limits" yaml:"limits"`
		} `json:"resources" yaml:"resources"`
		Persistence struct {
			Size string `json:"size" yaml:"size"`
		} `json:"persistence" yaml:"persistence"`
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
					Open struct {
						CONFIGFORCEStackstateTopologyQueryServiceMaxStackElementsPerQuery  string `json:"CONFIG_FORCE_stackstate_topologyQueryService_maxStackElementsPerQuery" yaml:"CONFIG_FORCE_stackstate_topologyQueryService_maxStackElementsPerQuery"`
						CONFIGFORCEStackstateTopologyQueryServiceMaxLoadedElementsPerQuery string `json:"CONFIG_FORCE_stackstate_topologyQueryService_maxLoadedElementsPerQuery" yaml:"CONFIG_FORCE_stackstate_topologyQueryService_maxLoadedElementsPerQuery"`
						CONFIGFORCEStackstateAgentsAgentLimit                              string `json:"CONFIG_FORCE_stackstate_agents_agentLimit" yaml:"CONFIG_FORCE_stackstate_agents_agentLimit"`
					} `json:"open" yaml:"open"`
				} `json:"extraEnv" yaml:"extraEnv"`
			} `json:"all" yaml:"all"`
			Server struct {
				ExtraEnv struct {
					Open struct {
						CONFIGFORCEStackstateSyncInitializationBatchParallelism     string `json:"CONFIG_FORCE_stackstate_sync_initializationBatchParallelism" yaml:"CONFIG_FORCE_stackstate_sync_initializationBatchParallelism"`
						CONFIGFORCEStackstateHealthSyncInitialLoadParallelism       string `json:"CONFIG_FORCE_stackstate_healthSync_initialLoadParallelism" yaml:"CONFIG_FORCE_stackstate_healthSync_initialLoadParallelism"`
						CONFIGFORCEStackstateStateServiceInitializationParallelism  string `json:"CONFIG_FORCE_stackstate_stateService_initializationParallelism" yaml:"CONFIG_FORCE_stackstate_stateService_initializationParallelism"`
						CONFIGFORCEStackstateStateServiceInitialLoadTransactionSize string `json:"CONFIG_FORCE_stackstate_stateService_initialLoadTransactionSize" yaml:"CONFIG_FORCE_stackstate_stateService_initialLoadTransactionSize"`
					} `json:"open" yaml:"open"`
				} `json:"extraEnv" yaml:"extraEnv"`
				Resources struct {
					Limits struct {
						EphemeralStorage string `json:"ephemeral-storage" yaml:"ephemeral-storage"`
						CPU              string `json:"cpu" yaml:"cpu"`
						Memory           string `json:"memory" yaml:"memory"`
					} `json:"limits" yaml:"limits"`
					Requests struct {
						CPU    string `json:"cpu" yaml:"cpu"`
						Memory string `json:"memory" yaml:"memory"`
					} `json:"requests" yaml:"requests"`
				} `json:"resources" yaml:"resources"`
			} `json:"server" yaml:"server"`
			E2Es struct {
				Resources struct {
					Requests struct {
						Memory string `json:"memory" yaml:"memory"`
						CPU    string `json:"cpu" yaml:"cpu"`
					} `json:"requests" yaml:"requests"`
					Limits struct {
						Memory string `json:"memory" yaml:"memory"`
					} `json:"limits" yaml:"limits"`
				} `json:"resources" yaml:"resources"`
			} `json:"e2es" yaml:"e2es"`
			Correlate struct {
				Resources struct {
					Requests struct {
						Memory string `json:"memory" yaml:"memory"`
						CPU    string `json:"cpu" yaml:"cpu"`
					} `json:"requests" yaml:"requests"`
					Limits struct {
						CPU    string `json:"cpu" yaml:"cpu"`
						Memory string `json:"memory" yaml:"memory"`
					} `json:"limits" yaml:"limits"`
				} `json:"resources" yaml:"resources"`
			} `json:"correlate" yaml:"correlate"`
			Receiver struct {
				Split struct {
					Enabled bool `json:"enabled" yaml:"enabled"`
				} `json:"split" yaml:"split"`
				ExtraEnv struct {
					Open struct {
						CONFIGFORCEAkkaHTTPHostConnectionPoolMaxOpenRequests string `json:"CONFIG_FORCE_akka_http_host__connection__pool_max__open__requests" yaml:"CONFIG_FORCE_akka_http_host__connection__pool_max__open__requests"`
					} `json:"open" yaml:"open"`
				} `json:"extraEnv" yaml:"extraEnv"`
				Resources struct {
					Requests struct {
						Memory string `json:"memory" yaml:"memory"`
						CPU    string `json:"cpu" yaml:"cpu"`
					} `json:"requests" yaml:"requests"`
					Limits struct {
						Memory string `json:"memory" yaml:"memory"`
						CPU    string `json:"cpu" yaml:"cpu"`
					} `json:"limits" yaml:"limits"`
				} `json:"resources" yaml:"resources"`
			} `json:"receiver" yaml:"receiver"`
			Vmagent struct {
				Resources struct {
					Limits struct {
						Memory string `json:"memory" yaml:"memory"`
					} `json:"limits" yaml:"limits"`
					Requests struct {
						Memory string `json:"memory" yaml:"memory"`
					} `json:"requests" yaml:"requests"`
				} `json:"resources" yaml:"resources"`
			} `json:"vmagent" yaml:"vmagent"`
			UI struct {
				ReplicaCount int `json:"replicaCount" yaml:"replicaCount"`
			} `json:"ui" yaml:"ui"`
		} `json:"components" yaml:"components"`
	} `json:"stackstate" yaml:"stackstate"`
	VictoriaMetrics0 struct {
		Server struct {
			Resources struct {
				Requests struct {
					CPU    string `json:"cpu" yaml:"cpu"`
					Memory string `json:"memory" yaml:"memory"`
				} `json:"requests" yaml:"requests"`
				Limits struct {
					CPU    string `json:"cpu" yaml:"cpu"`
					Memory string `json:"memory" yaml:"memory"`
				} `json:"limits" yaml:"limits"`
			} `json:"resources" yaml:"resources"`
			PersistentVolume struct {
				Size string `json:"size" yaml:"size"`
			} `json:"persistentVolume" yaml:"persistentVolume"`
		} `json:"server" yaml:"server"`
		Backup struct {
			Vmbackup struct {
				Resources struct {
					Requests struct {
						Memory string `json:"memory" yaml:"memory"`
					} `json:"requests" yaml:"requests"`
					Limits struct {
						Memory string `json:"memory" yaml:"memory"`
					} `json:"limits" yaml:"limits"`
				} `json:"resources" yaml:"resources"`
			} `json:"vmbackup" yaml:"vmbackup"`
		} `json:"backup" yaml:"backup"`
	} `json:"victoria-metrics-0" yaml:"victoria-metrics-0"`
	VictoriaMetrics1 struct {
		Enabled bool `json:"enabled" yaml:"enabled"`
		Server  struct {
			PersistentVolume struct {
				Size string `json:"size" yaml:"size"`
			} `json:"persistentVolume" yaml:"persistentVolume"`
		} `json:"server" yaml:"server"`
	} `json:"victoria-metrics-1" yaml:"victoria-metrics-1"`
	Zookeeper struct {
		ReplicaCount int `json:"replicaCount" yaml:"replicaCount"`
	} `json:"zookeeper" yaml:"zookeeper"`
}
