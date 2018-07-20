package client

const (
	ClusterLoggingSpecType                     = "clusterLoggingSpec"
	ClusterLoggingSpecFieldClusterID           = "clusterId"
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
	ClusterID           string               `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	DisplayName         string               `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	ElasticsearchConfig *ElasticsearchConfig `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	EmbeddedConfig      *EmbeddedConfig      `json:"embeddedConfig,omitempty" yaml:"embeddedConfig,omitempty"`
	KafkaConfig         *KafkaConfig         `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	OutputFlushInterval int64                `json:"outputFlushInterval,omitempty" yaml:"outputFlushInterval,omitempty"`
	OutputTags          map[string]string    `json:"outputTags,omitempty" yaml:"outputTags,omitempty"`
	SplunkConfig        *SplunkConfig        `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	SyslogConfig        *SyslogConfig        `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
}
