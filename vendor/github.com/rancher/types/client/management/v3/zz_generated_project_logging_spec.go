package client

const (
	ProjectLoggingSpecType                       = "projectLoggingSpec"
	ProjectLoggingSpecFieldCustomTargetConfig    = "customTargetConfig"
	ProjectLoggingSpecFieldDisplayName           = "displayName"
	ProjectLoggingSpecFieldElasticsearchConfig   = "elasticsearchConfig"
	ProjectLoggingSpecFieldFluentForwarderConfig = "fluentForwarderConfig"
	ProjectLoggingSpecFieldKafkaConfig           = "kafkaConfig"
	ProjectLoggingSpecFieldOutputFlushInterval   = "outputFlushInterval"
	ProjectLoggingSpecFieldOutputTags            = "outputTags"
	ProjectLoggingSpecFieldProjectID             = "projectId"
	ProjectLoggingSpecFieldSplunkConfig          = "splunkConfig"
	ProjectLoggingSpecFieldSyslogConfig          = "syslogConfig"
)

type ProjectLoggingSpec struct {
	CustomTargetConfig    *CustomTargetConfig    `json:"customTargetConfig,omitempty" yaml:"customTargetConfig,omitempty"`
	DisplayName           string                 `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	ElasticsearchConfig   *ElasticsearchConfig   `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	FluentForwarderConfig *FluentForwarderConfig `json:"fluentForwarderConfig,omitempty" yaml:"fluentForwarderConfig,omitempty"`
	KafkaConfig           *KafkaConfig           `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	OutputFlushInterval   int64                  `json:"outputFlushInterval,omitempty" yaml:"outputFlushInterval,omitempty"`
	OutputTags            map[string]string      `json:"outputTags,omitempty" yaml:"outputTags,omitempty"`
	ProjectID             string                 `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SplunkConfig          *SplunkConfig          `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	SyslogConfig          *SyslogConfig          `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
}
