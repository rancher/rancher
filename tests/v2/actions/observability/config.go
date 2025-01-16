package observability

type StackStateConfig struct {
	ServiceToken   string `json:"serviceToken" yaml:"serviceToken"`
	Url            string `json:"url" yaml:"url"`
	ClusterApiKey  string `json:"clusterApiKey" yaml:"clusterApiKey"`
	UpgradeVersion string `json:"upgradeVersion" yaml:"upgradeVersion"`
}

// StackStateServerValuesConfig represents the StateState Server configuration values
// CombinedConfig represents the unified configuration values
type StackStateServerValuesConfig struct {
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
		Experimental struct {
			Server struct {
				Split bool `yaml:"split"`
			} `yaml:"server"`
		} `yaml:"experimental"`
		Components struct {
			All struct {
				ExtraEnv struct {
					Open struct {
						CONFIGFORCEStackstateTopologyQueryServiceMaxStackElementsPerQuery  string `yaml:"CONFIG_FORCE_stackstate_topologyQueryService_maxStackElementsPerQuery"`
						CONFIGFORCEStackstateTopologyQueryServiceMaxLoadedElementsPerQuery string `yaml:"CONFIG_FORCE_stackstate_topologyQueryService_maxLoadedElementsPerQuery"`
						CONFIGFORCEStackstateAgentsAgentLimit                              string `yaml:"CONFIG_FORCE_stackstate_agents_agentLimit"`
					} `yaml:"open"`
				} `yaml:"extraEnv"`
			} `yaml:"all"`
			Server struct {
				ExtraEnv struct {
					Open struct {
						CONFIGFORCEStackstateSyncInitializationBatchParallelism     string `yaml:"CONFIG_FORCE_stackstate_sync_initializationBatchParallelism"`
						CONFIGFORCEStackstateHealthSyncInitialLoadParallelism       string `yaml:"CONFIG_FORCE_stackstate_healthSync_initialLoadParallelism"`
						CONFIGFORCEStackstateStateServiceInitializationParallelism  string `yaml:"CONFIG_FORCE_stackstate_stateService_initializationParallelism"`
						CONFIGFORCEStackstateStateServiceInitialLoadTransactionSize string `yaml:"CONFIG_FORCE_stackstate_stateService_initialLoadTransactionSize"`
					} `yaml:"open"`
				} `yaml:"extraEnv"`
				Resources struct {
					Limits struct {
						EphemeralStorage string `yaml:"ephemeral-storage"`
						CPU              string `yaml:"cpu"`
						Memory           string `yaml:"memory"`
					} `yaml:"limits"`
					Requests struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"requests"`
				} `yaml:"resources"`
			} `yaml:"server"`
			E2Es struct {
				Resources struct {
					Requests struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"requests"`
					Limits struct {
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
				} `yaml:"resources"`
			} `yaml:"e2es"`
			Correlate struct {
				Resources struct {
					Requests struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"requests"`
					Limits struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
				} `yaml:"resources"`
			} `yaml:"correlate"`
			Receiver struct {
				Split struct {
					Enabled bool `yaml:"enabled"`
				} `yaml:"split"`
				ExtraEnv struct {
					Open struct {
						CONFIGFORCEAkkaHTTPHostConnectionPoolMaxOpenRequests string `yaml:"CONFIG_FORCE_akka_http_host__connection__pool_max__open__requests"`
					} `yaml:"open"`
				} `yaml:"extraEnv"`
				Resources struct {
					Requests struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"requests"`
					Limits struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"limits"`
				} `yaml:"resources"`
			} `yaml:"receiver"`
			Vmagent struct {
				Resources struct {
					Limits struct {
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
					Requests struct {
						Memory string `yaml:"memory"`
					} `yaml:"requests"`
				} `yaml:"resources"`
			} `yaml:"vmagent"`
			UI struct {
				ReplicaCount int `yaml:"replicaCount"`
			} `yaml:"ui"`
		} `yaml:"components"`
	} `yaml:"stackstate"`
	VictoriaMetrics0 struct {
		Server struct {
			Resources struct {
				Requests struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				} `yaml:"requests"`
				Limits struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				} `yaml:"limits"`
			} `yaml:"resources"`
			PersistentVolume struct {
				Size string `yaml:"size"`
			} `yaml:"persistentVolume"`
		} `yaml:"server"`
		Backup struct {
			Vmbackup struct {
				Resources struct {
					Requests struct {
						Memory string `yaml:"memory"`
					} `yaml:"requests"`
					Limits struct {
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
				} `yaml:"resources"`
			} `yaml:"vmbackup"`
		} `yaml:"backup"`
	} `yaml:"victoria-metrics-0"`
	VictoriaMetrics1 struct {
		Enabled bool `yaml:"enabled"`
		Server  struct {
			PersistentVolume struct {
				Size string `yaml:"size"`
			} `yaml:"persistentVolume"`
		} `yaml:"server"`
	} `yaml:"victoria-metrics-1"`
	Zookeeper struct {
		ReplicaCount int `yaml:"replicaCount"`
	} `yaml:"zookeeper"`
	Sizing struct {
		Clickhouse struct {
			ReplicaCount int `yaml:"replicaCount"`
			Persistence  struct {
				Size string `yaml:"size"`
			} `yaml:"persistence"`
		} `yaml:"clickhouse"`
		Elasticsearch struct {
			PrometheusElasticsearchExporter struct {
				Resources struct {
					Limits struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
					Requests struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"requests"`
				} `yaml:"resources"`
			} `yaml:"prometheus-elasticsearch-exporter"`
			MinimumMasterNodes int    `yaml:"minimumMasterNodes"`
			Replicas           int    `yaml:"replicas"`
			EsJavaOpts         string `yaml:"esJavaOpts"`
			Resources          struct {
				Requests struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				} `yaml:"requests"`
				Limits struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				} `yaml:"limits"`
			} `yaml:"resources"`
			VolumeClaimTemplate struct {
				Resources struct {
					Requests struct {
						Storage string `yaml:"storage"`
					} `yaml:"resources"`
				} `yaml:"resources"`
			} `yaml:"volumeClaimTemplate"`
		} `yaml:"elasticsearch"`
		Hbase struct {
			Version    string `yaml:"version"`
			Deployment struct {
				Mode string `yaml:"mode"`
			} `yaml:"deployment"`
			Stackgraph struct {
				Persistence struct {
					Size string `yaml:"size"`
				} `yaml:"persistence"`
				Resources struct {
					Requests struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"requests"`
					Limits struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
				} `yaml:"resources"`
			} `yaml:"stackgraph"`
			Tephra struct {
				Resources struct {
					Limits struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
					Requests struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"requests"`
				} `yaml:"resources"`
				ReplicaCount int `yaml:"replicaCount"`
			} `yaml:"tephra"`
		} `yaml:"hbase"`
		Kafka struct {
			DefaultReplicationFactor             int `yaml:"defaultReplicationFactor"`
			OffsetsTopicReplicationFactor        int `yaml:"offsetsTopicReplicationFactor"`
			ReplicaCount                         int `yaml:"replicaCount"`
			TransactionStateLogReplicationFactor int `yaml:"transactionStateLogReplicationFactor"`
			Resources                            struct {
				Requests struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				} `yaml:"requests"`
				Limits struct {
					Memory string `yaml:"memory"`
					CPU    string `yaml:"cpu"`
				} `yaml:"limits"`
			} `yaml:"resources"`
			Persistence struct {
				Size string `yaml:"size"`
			} `yaml:"persistence"`
		} `yaml:"kafka"`
	} `yaml:"sizing"`
}
