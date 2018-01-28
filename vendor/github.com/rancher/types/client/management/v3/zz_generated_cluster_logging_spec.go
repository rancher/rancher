package client

const (
	ClusterLoggingSpecType                     = "clusterLoggingSpec"
	ClusterLoggingSpecFieldClusterId           = "clusterId"
	ClusterLoggingSpecFieldDisplayName         = "displayName"
	ClusterLoggingSpecFieldElasticsearchConfig = "elasticsearchConfig"
	ClusterLoggingSpecFieldEmbeddedConfig      = "embeddedConfig"
	ClusterLoggingSpecFieldKafkaConfig         = "kafkaConfig"
	ClusterLoggingSpecFieldOutputFlushInterval = "outputFlushInterval"
	ClusterLoggingSpecFieldOutputTags          = "outputTags"
	ClusterLoggingSpecFieldSplunkConfig        = "splunkConfig"
	ClusterLoggingSpecFieldSyslogConfig        = "syslogConfig"
)

type ClusterLoggingSpec struct {
	ClusterId           string               `json:"clusterId,omitempty"`
	DisplayName         string               `json:"displayName,omitempty"`
	ElasticsearchConfig *ElasticsearchConfig `json:"elasticsearchConfig,omitempty"`
	EmbeddedConfig      *EmbeddedConfig      `json:"embeddedConfig,omitempty"`
	KafkaConfig         *KafkaConfig         `json:"kafkaConfig,omitempty"`
	OutputFlushInterval *int64               `json:"outputFlushInterval,omitempty"`
	OutputTags          map[string]string    `json:"outputTags,omitempty"`
	SplunkConfig        *SplunkConfig        `json:"splunkConfig,omitempty"`
	SyslogConfig        *SyslogConfig        `json:"syslogConfig,omitempty"`
}
