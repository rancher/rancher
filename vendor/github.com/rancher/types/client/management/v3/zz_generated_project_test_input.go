package client

const (
	ProjectTestInputType                       = "projectTestInput"
	ProjectTestInputFieldContinuousLineRegexp  = "continuousLineRegexp"
	ProjectTestInputFieldCustomTargetConfig    = "customTargetConfig"
	ProjectTestInputFieldDisplayName           = "displayName"
	ProjectTestInputFieldElasticsearchConfig   = "elasticsearchConfig"
	ProjectTestInputFieldEnableMultiLineFilter = "enableMultiLineFilter"
	ProjectTestInputFieldFluentForwarderConfig = "fluentForwarderConfig"
	ProjectTestInputFieldKafkaConfig           = "kafkaConfig"
	ProjectTestInputFieldMultiLineEndRegexp    = "multiLineEndRegexp"
	ProjectTestInputFieldMultiLineStartRegexp  = "multiLineStartRegexp"
	ProjectTestInputFieldOutputFlushInterval   = "outputFlushInterval"
	ProjectTestInputFieldOutputTags            = "outputTags"
	ProjectTestInputFieldProjectName           = "projectId"
	ProjectTestInputFieldSplunkConfig          = "splunkConfig"
	ProjectTestInputFieldSyslogConfig          = "syslogConfig"
)

type ProjectTestInput struct {
	ContinuousLineRegexp  string                 `json:"continuousLineRegexp,omitempty" yaml:"continuousLineRegexp,omitempty"`
	CustomTargetConfig    *CustomTargetConfig    `json:"customTargetConfig,omitempty" yaml:"customTargetConfig,omitempty"`
	DisplayName           string                 `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	ElasticsearchConfig   *ElasticsearchConfig   `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	EnableMultiLineFilter bool                   `json:"enableMultiLineFilter,omitempty" yaml:"enableMultiLineFilter,omitempty"`
	FluentForwarderConfig *FluentForwarderConfig `json:"fluentForwarderConfig,omitempty" yaml:"fluentForwarderConfig,omitempty"`
	KafkaConfig           *KafkaConfig           `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	MultiLineEndRegexp    string                 `json:"multiLineEndRegexp,omitempty" yaml:"multiLineEndRegexp,omitempty"`
	MultiLineStartRegexp  string                 `json:"multiLineStartRegexp,omitempty" yaml:"multiLineStartRegexp,omitempty"`
	OutputFlushInterval   int64                  `json:"outputFlushInterval,omitempty" yaml:"outputFlushInterval,omitempty"`
	OutputTags            map[string]string      `json:"outputTags,omitempty" yaml:"outputTags,omitempty"`
	ProjectName           string                 `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SplunkConfig          *SplunkConfig          `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	SyslogConfig          *SyslogConfig          `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
}
