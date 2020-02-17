package client

const (
	ProjectTestInputType                       = "projectTestInput"
	ProjectTestInputFieldCustomTargetConfig    = "customTargetConfig"
	ProjectTestInputFieldElasticsearchConfig   = "elasticsearchConfig"
	ProjectTestInputFieldFluentForwarderConfig = "fluentForwarderConfig"
	ProjectTestInputFieldGraylogConfig         = "graylogConfig"
	ProjectTestInputFieldKafkaConfig           = "kafkaConfig"
	ProjectTestInputFieldOutputTags            = "outputTags"
	ProjectTestInputFieldProjectName           = "projectId"
	ProjectTestInputFieldSplunkConfig          = "splunkConfig"
	ProjectTestInputFieldSyslogConfig          = "syslogConfig"
)

type ProjectTestInput struct {
	CustomTargetConfig    *CustomTargetConfig    `json:"customTargetConfig,omitempty" yaml:"customTargetConfig,omitempty"`
	ElasticsearchConfig   *ElasticsearchConfig   `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	FluentForwarderConfig *FluentForwarderConfig `json:"fluentForwarderConfig,omitempty" yaml:"fluentForwarderConfig,omitempty"`
	GraylogConfig         *GraylogConfig         `json:"graylogConfig,omitempty" yaml:"graylogConfig,omitempty"`
	KafkaConfig           *KafkaConfig           `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	OutputTags            map[string]string      `json:"outputTags,omitempty" yaml:"outputTags,omitempty"`
	ProjectName           string                 `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SplunkConfig          *SplunkConfig          `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	SyslogConfig          *SyslogConfig          `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
}
