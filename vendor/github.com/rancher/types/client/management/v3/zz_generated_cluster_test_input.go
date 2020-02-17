package client

const (
	ClusterTestInputType                       = "clusterTestInput"
	ClusterTestInputFieldClusterName           = "clusterId"
	ClusterTestInputFieldCustomTargetConfig    = "customTargetConfig"
	ClusterTestInputFieldElasticsearchConfig   = "elasticsearchConfig"
	ClusterTestInputFieldFluentForwarderConfig = "fluentForwarderConfig"
	ClusterTestInputFieldGraylogConfig         = "graylogConfig"
	ClusterTestInputFieldKafkaConfig           = "kafkaConfig"
	ClusterTestInputFieldOutputTags            = "outputTags"
	ClusterTestInputFieldSplunkConfig          = "splunkConfig"
	ClusterTestInputFieldSyslogConfig          = "syslogConfig"
)

type ClusterTestInput struct {
	ClusterName           string                 `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	CustomTargetConfig    *CustomTargetConfig    `json:"customTargetConfig,omitempty" yaml:"customTargetConfig,omitempty"`
	ElasticsearchConfig   *ElasticsearchConfig   `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	FluentForwarderConfig *FluentForwarderConfig `json:"fluentForwarderConfig,omitempty" yaml:"fluentForwarderConfig,omitempty"`
	GraylogConfig         *GraylogConfig         `json:"graylogConfig,omitempty" yaml:"graylogConfig,omitempty"`
	KafkaConfig           *KafkaConfig           `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	OutputTags            map[string]string      `json:"outputTags,omitempty" yaml:"outputTags,omitempty"`
	SplunkConfig          *SplunkConfig          `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	SyslogConfig          *SyslogConfig          `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
}
