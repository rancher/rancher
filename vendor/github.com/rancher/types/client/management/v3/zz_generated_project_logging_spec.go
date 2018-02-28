package client

const (
	ProjectLoggingSpecType                     = "projectLoggingSpec"
	ProjectLoggingSpecFieldDisplayName         = "displayName"
	ProjectLoggingSpecFieldElasticsearchConfig = "elasticsearchConfig"
	ProjectLoggingSpecFieldKafkaConfig         = "kafkaConfig"
	ProjectLoggingSpecFieldOutputFlushInterval = "outputFlushInterval"
	ProjectLoggingSpecFieldOutputTags          = "outputTags"
	ProjectLoggingSpecFieldProjectId           = "projectId"
	ProjectLoggingSpecFieldSplunkConfig        = "splunkConfig"
	ProjectLoggingSpecFieldSyslogConfig        = "syslogConfig"
)

type ProjectLoggingSpec struct {
	DisplayName         string               `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	ElasticsearchConfig *ElasticsearchConfig `json:"elasticsearchConfig,omitempty" yaml:"elasticsearchConfig,omitempty"`
	KafkaConfig         *KafkaConfig         `json:"kafkaConfig,omitempty" yaml:"kafkaConfig,omitempty"`
	OutputFlushInterval *int64               `json:"outputFlushInterval,omitempty" yaml:"outputFlushInterval,omitempty"`
	OutputTags          map[string]string    `json:"outputTags,omitempty" yaml:"outputTags,omitempty"`
	ProjectId           string               `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SplunkConfig        *SplunkConfig        `json:"splunkConfig,omitempty" yaml:"splunkConfig,omitempty"`
	SyslogConfig        *SyslogConfig        `json:"syslogConfig,omitempty" yaml:"syslogConfig,omitempty"`
}
