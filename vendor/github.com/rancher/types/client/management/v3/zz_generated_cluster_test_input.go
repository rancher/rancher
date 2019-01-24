package client

const (
	ClusterTestInputType                       = "clusterTestInput"
	ClusterTestInputFieldClusterName           = "clusterId"
	ClusterTestInputFieldCustomTargetConfig    = "customTargetConfig"
	ClusterTestInputFieldElasticsearchConfig   = "elasticsearchConfig"
	ClusterTestInputFieldFluentForwarderConfig = "fluentForwarderConfig"
	ClusterTestInputFieldKafkaConfig           = "kafkaConfig"
	ClusterTestInputFieldSplunkConfig          = "splunkConfig"
	ClusterTestInputFieldSyslogConfig          = "syslogConfig"
)

type ClusterTestInput struct {
	ClusterName           string                 `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	CustomTargetConfig    *CustomTargetConfig    `json:"customTargetConfig,omitempty" yaml:"customTargetConfig,omitempty"`
	ElasticsearchConfig   *ElasticsearchConfig   `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	FluentForwarderConfig *FluentForwarderConfig `json:"fluentForwarderConfig,omitempty" yaml:"fluentForwarderConfig,omitempty"`
	KafkaConfig           *KafkaConfig           `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	SplunkConfig          *SplunkConfig          `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	SyslogConfig          *SyslogConfig          `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
}
