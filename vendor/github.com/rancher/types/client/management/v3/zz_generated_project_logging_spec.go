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
	DisplayName         string               `json:"displayName,omitempty"`
	ElasticsearchConfig *ElasticsearchConfig `json:"elasticsearchConfig,omitempty"`
	KafkaConfig         *KafkaConfig         `json:"kafkaConfig,omitempty"`
	OutputFlushInterval *int64               `json:"outputFlushInterval,omitempty"`
	OutputTags          map[string]string    `json:"outputTags,omitempty"`
	ProjectId           string               `json:"projectId,omitempty"`
	SplunkConfig        *SplunkConfig        `json:"splunkConfig,omitempty"`
	SyslogConfig        *SyslogConfig        `json:"syslogConfig,omitempty"`
}
