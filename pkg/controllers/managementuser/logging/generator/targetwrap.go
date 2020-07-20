package generator

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/managementuser/logging/config"
	"github.com/rancher/rancher/pkg/controllers/managementuser/logging/utils"
)

type LoggingTargetTemplateWrap struct {
	CurrentTarget string
	ElasticsearchTemplateWrap
	SplunkTemplateWrap
	SyslogTemplateWrap
	KafkaTemplateWrap
	FluentForwarderTemplateWrap
	CustomTargetWrap
}

type ClusterLoggingTemplateWrap struct {
	ExcludeNamespace string

	v32.LoggingCommonField
	LoggingTargetTemplateWrap
	IncludeRke              bool
	CertFilePrefix          string
	BufferFile              string
	ContainerLogSourceTag   string
	CustomLogSourceTag      string
	ContainerLogPosFilename string
	RkeLogTag               string
	RkeLogPosFilename       string
}

type ProjectLoggingTemplateWrap struct {
	ContainerSourcePath string

	v32.LoggingCommonField
	LoggingTargetTemplateWrap
	IncludeRke              bool
	CertFilePrefix          string
	BufferFile              string
	ContainerLogSourceTag   string
	CustomLogSourceTag      string
	ContainerLogPosFilename string
	RkeLogTag               string
	RkeLogPosFilename       string
}

func newWrapClusterLogging(logging v32.ClusterLoggingSpec, excludeNamespace, certDir string) (*ClusterLoggingTemplateWrap, error) {
	wrap, err := NewLoggingTargetTemplateWrap(logging.LoggingTargets)
	if err != nil {
		return nil, errors.Wrapf(err, "wrapper logging target failed")
	}

	if wrap == nil {
		return nil, nil
	}

	includeSystemComponent := true
	if logging.IncludeSystemComponent != nil {
		includeSystemComponent = *logging.IncludeSystemComponent
	}

	level := "cluster"
	certFilePrefix := getCertFilePrefix(certDir, level, logging.ClusterName)
	bufferFile := getBufferFilename(level, "")
	customLogSourceTag := getCustomLogSourceTag(level, "")
	containerLogPosFilename := getContainerLogPosFilename(level, "")
	return &ClusterLoggingTemplateWrap{
		ExcludeNamespace:          excludeNamespace,
		LoggingCommonField:        logging.LoggingCommonField,
		LoggingTargetTemplateWrap: *wrap,
		IncludeRke:                includeSystemComponent,
		CertFilePrefix:            certFilePrefix,
		BufferFile:                bufferFile,
		ContainerLogSourceTag:     level,
		CustomLogSourceTag:        customLogSourceTag,
		ContainerLogPosFilename:   containerLogPosFilename,
		RkeLogTag:                 "rke",
		RkeLogPosFilename:         "fluentd-rke-logging.pos",
	}, nil
}

func newWrapProjectLogging(logging v32.ProjectLoggingSpec, containerSourcePath, certDir string, isSystemProject bool) (*ProjectLoggingTemplateWrap, error) {
	wrap, err := NewLoggingTargetTemplateWrap(logging.LoggingTargets)
	if err != nil {
		return nil, errors.Wrapf(err, "wrapper logging target failed")
	}

	if wrap == nil {
		return nil, nil
	}

	level := "project"
	wrapProjectName := strings.Replace(logging.ProjectName, ":", "_", -1)
	certFilePrefix := getCertFilePrefix(certDir, level, wrapProjectName)
	bufferFile := getBufferFilename(level, wrapProjectName)
	customLogSourceTag := getCustomLogSourceTag(level, logging.ProjectName)
	containerLogPosFilename := getContainerLogPosFilename(level, logging.ProjectName)

	return &ProjectLoggingTemplateWrap{
		ContainerSourcePath:       containerSourcePath,
		LoggingCommonField:        logging.LoggingCommonField,
		LoggingTargetTemplateWrap: *wrap,
		IncludeRke:                isSystemProject,
		CertFilePrefix:            certFilePrefix,
		BufferFile:                bufferFile,
		ContainerLogSourceTag:     logging.ProjectName,
		CustomLogSourceTag:        customLogSourceTag,
		ContainerLogPosFilename:   containerLogPosFilename,
		RkeLogTag:                 "rke-system-project",
		RkeLogPosFilename:         "fluentd-rke-logging-system-project.pos",
	}, nil
}

type ElasticsearchTemplateWrap struct {
	v32.ElasticsearchConfig
	DateFormat string
	Host       string
	Scheme     string
}

type SplunkTemplateWrap struct {
	v32.SplunkConfig
	Host   string
	Port   string
	Scheme string
}

type KafkaTemplateWrap struct {
	v32.KafkaConfig
	Brokers   string
	Zookeeper string
	IsSSL     bool
}

type SyslogTemplateWrap struct {
	v32.SyslogConfig
	Host         string
	Port         string
	WrapSeverity string
}

type FluentForwarderTemplateWrap struct {
	v32.FluentForwarderConfig
	EnableShareKey bool
	FluentServers  []FluentServer
}

type FluentServer struct {
	Host string
	Port string
	v32.FluentServer
}

type CustomTargetWrap struct {
	v32.CustomTargetConfig
}

func NewLoggingTargetTemplateWrap(loggingTagets v32.LoggingTargets) (wrapLogging *LoggingTargetTemplateWrap, err error) {
	wp := &LoggingTargetTemplateWrap{}
	if loggingTagets.ElasticsearchConfig != nil {

		wrap, err := newElasticsearchTemplateWrap(loggingTagets.ElasticsearchConfig)
		if err != nil {
			return nil, err
		}
		wp.ElasticsearchTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Elasticsearch
		return wp, nil

	} else if loggingTagets.SplunkConfig != nil {

		wrap, err := newSplunkTemplateWrap(loggingTagets.SplunkConfig)
		if err != nil {
			return nil, err
		}
		wp.SplunkTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Splunk
		return wp, nil

	} else if loggingTagets.SyslogConfig != nil {

		wrap, err := newSyslogTemplateWrap(loggingTagets.SyslogConfig)
		if err != nil {
			return nil, err
		}
		wp.SyslogTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Syslog
		return wp, nil

	} else if loggingTagets.KafkaConfig != nil {

		wrap, err := newKafkaTemplateWrap(loggingTagets.KafkaConfig)
		if err != nil {
			return nil, err
		}
		wp.KafkaTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Kafka
		return wp, nil

	} else if loggingTagets.FluentForwarderConfig != nil {

		wrap, err := newFluentForwarderTemplateWrap(loggingTagets.FluentForwarderConfig)
		if err != nil {
			return nil, err
		}
		wp.FluentForwarderTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.FluentForwarder
		return wp, nil

	} else if loggingTagets.CustomTargetConfig != nil {

		wrap := CustomTargetWrap{*loggingTagets.CustomTargetConfig}
		wp.CustomTargetWrap = wrap
		wp.CurrentTarget = loggingconfig.CustomTarget
		return wp, nil
	}

	return nil, nil
}

func newElasticsearchTemplateWrap(elasticsearchConfig *v32.ElasticsearchConfig) (*ElasticsearchTemplateWrap, error) {
	h, s, err := parseEndpoint(elasticsearchConfig.Endpoint)
	if err != nil {
		return nil, err
	}
	return &ElasticsearchTemplateWrap{
		ElasticsearchConfig: *elasticsearchConfig,
		Host:                h,
		Scheme:              s,
		DateFormat:          utils.GetDateFormat(elasticsearchConfig.DateFormat),
	}, nil
}

func newSplunkTemplateWrap(splunkConfig *v32.SplunkConfig) (*SplunkTemplateWrap, error) {
	h, s, err := parseEndpoint(splunkConfig.Endpoint)
	if err != nil {
		return nil, err
	}

	host, port, err := net.SplitHostPort(h)
	if err != nil {
		return nil, err
	}
	return &SplunkTemplateWrap{
		SplunkConfig: *splunkConfig,
		Host:         host,
		Scheme:       s,
		Port:         port,
	}, nil
}

func newSyslogTemplateWrap(syslogConfig *v32.SyslogConfig) (*SyslogTemplateWrap, error) {
	host, port, err := net.SplitHostPort(syslogConfig.Endpoint)
	if err != nil {
		return nil, err
	}

	return &SyslogTemplateWrap{
		SyslogConfig: *syslogConfig,
		Host:         host,
		Port:         port,
		WrapSeverity: utils.GetWrapSeverity(syslogConfig.Severity),
	}, nil
}

func newKafkaTemplateWrap(kafkaConfig *v32.KafkaConfig) (*KafkaTemplateWrap, error) {
	wrap := &KafkaTemplateWrap{
		KafkaConfig: *kafkaConfig,
	}
	if len(kafkaConfig.BrokerEndpoints) == 0 && kafkaConfig.ZookeeperEndpoint == "" {
		err := errors.New("one of the kafka endpoints must be set")
		return nil, err
	}
	if len(kafkaConfig.BrokerEndpoints) != 0 {
		var bs []string
		var h, s string
		var err error
		for _, v := range kafkaConfig.BrokerEndpoints {
			h, s, err = parseEndpoint(v)
			if err != nil {
				return nil, err
			}
			bs = append(bs, h)
		}
		wrap.IsSSL = s == "https"
		wrap.Brokers = strings.Join(bs, ",")
	} else {
		if kafkaConfig.ZookeeperEndpoint != "" {
			h, _, err := parseEndpoint(kafkaConfig.ZookeeperEndpoint)
			if err != nil {
				return nil, err
			}
			wrap.Zookeeper = h
		}
	}
	return wrap, nil
}

func newFluentForwarderTemplateWrap(fluentForwarderConfig *v32.FluentForwarderConfig) (*FluentForwarderTemplateWrap, error) {
	var enableShareKey bool
	var fss []FluentServer
	for _, v := range fluentForwarderConfig.FluentServers {
		host, port, err := net.SplitHostPort(v.Endpoint)
		if err != nil {
			return nil, err
		}
		if v.SharedKey != "" {
			enableShareKey = true
		}
		fs := FluentServer{
			Host:         host,
			Port:         port,
			FluentServer: v,
		}
		fss = append(fss, fs)
	}

	return &FluentForwarderTemplateWrap{
		FluentForwarderConfig: *fluentForwarderConfig,
		EnableShareKey:        enableShareKey,
		FluentServers:         fss,
	}, nil
}

func parseEndpoint(endpoint string) (host string, scheme string, err error) {
	u, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return "", "", errors.Wrapf(err, "couldn't parse url %s", endpoint)
	}

	if u.Host == "" || u.Scheme == "" {
		return "", "", errors.New("invalid url " + endpoint + " empty host or scheme")
	}

	return u.Host, u.Scheme, nil
}

func getCertFilePrefix(certDir, level, identy string) string {
	return fmt.Sprintf("%s/%s_%s", certDir, level, identy)
}

func getBufferFilename(level, identify string) string {
	if identify != "" {
		return fmt.Sprintf("%s.%s.buffer", level, identify)
	}
	return fmt.Sprintf("%s.buffer", level)
}

func getCustomLogSourceTag(level, identify string) string {
	if identify != "" {
		return fmt.Sprintf("%s-custom.%s", level, identify)
	}
	return fmt.Sprintf("%s-custom", level)
}

func getContainerLogPosFilename(level, identify string) string {
	if identify != "" {
		return fmt.Sprintf("fluentd-%s-%s-logging.pos", level, identify)
	}
	return fmt.Sprintf("fluentd-%s-logging.pos", level)
}
