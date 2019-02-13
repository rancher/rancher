package utils

import (
	"net"
	"net/url"
	"strings"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
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
	v3.LoggingCommonField
	LoggingTargetTemplateWrap
	ExcludeNamespace       string
	IncludeSystemComponent bool
}

type ProjectLoggingTemplateWrap struct {
	ProjectName string
	v3.LoggingCommonField
	LoggingTargetTemplateWrap
	GrepNamespace   string
	IsSystemProject bool
	WrapProjectName string
}

func NewWrapClusterLogging(logging v3.ClusterLoggingSpec, excludeNamespace string) (*ClusterLoggingTemplateWrap, error) {
	wrap, err := newLoggingTargetTemplateWrap(logging.ElasticsearchConfig, logging.SplunkConfig, logging.SyslogConfig, logging.KafkaConfig, logging.FluentForwarderConfig, logging.CustomTargetConfig)
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

	return &ClusterLoggingTemplateWrap{
		LoggingCommonField:        logging.LoggingCommonField,
		LoggingTargetTemplateWrap: *wrap,
		ExcludeNamespace:          excludeNamespace,
		IncludeSystemComponent:    includeSystemComponent,
	}, nil
}

func NewWrapProjectLogging(logging v3.ProjectLoggingSpec, grepNamespace string, isSystemProject bool) (*ProjectLoggingTemplateWrap, error) {
	wrap, err := newLoggingTargetTemplateWrap(logging.ElasticsearchConfig, logging.SplunkConfig, logging.SyslogConfig, logging.KafkaConfig, logging.FluentForwarderConfig, logging.CustomTargetConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "wrapper logging target failed")
	}

	if wrap == nil {
		return nil, nil
	}

	wrapProjectName := strings.Replace(logging.ProjectName, ":", "_", -1)
	return &ProjectLoggingTemplateWrap{
		ProjectName:               logging.ProjectName,
		LoggingCommonField:        logging.LoggingCommonField,
		LoggingTargetTemplateWrap: *wrap,
		GrepNamespace:             grepNamespace,
		IsSystemProject:           isSystemProject,
		WrapProjectName:           wrapProjectName,
	}, nil
}

type ElasticsearchTemplateWrap struct {
	v3.ElasticsearchConfig
	DateFormat string
	Host       string
	Scheme     string
}

type SplunkTemplateWrap struct {
	v3.SplunkConfig
	Host   string
	Port   string
	Scheme string
}

type KafkaTemplateWrap struct {
	v3.KafkaConfig
	Brokers   string
	Zookeeper string
	IsSSL     bool
}

type SyslogTemplateWrap struct {
	v3.SyslogConfig
	Host string
	Port string
}

type FluentForwarderTemplateWrap struct {
	v3.FluentForwarderConfig
	EnableShareKey bool
	FluentServers  []FluentServer
}

type FluentServer struct {
	Host string
	Port string
	v3.FluentServer
}

type CustomTargetWrap struct {
	v3.CustomTargetConfig
}

func newLoggingTargetTemplateWrap(es *v3.ElasticsearchConfig, sp *v3.SplunkConfig, sl *v3.SyslogConfig, kf *v3.KafkaConfig, ff *v3.FluentForwarderConfig, cc *v3.CustomTargetConfig) (wrapLogging *LoggingTargetTemplateWrap, err error) {
	wp := &LoggingTargetTemplateWrap{}
	if es != nil {

		wrap, err := newElasticsearchTemplateWrap(es)
		if err != nil {
			return nil, err
		}
		wp.ElasticsearchTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Elasticsearch
		return wp, nil

	} else if sp != nil {

		wrap, err := newSplunkTemplateWrap(sp)
		if err != nil {
			return nil, err
		}
		wp.SplunkTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Splunk
		return wp, nil

	} else if sl != nil {

		wrap, err := newSyslogTemplateWrap(sl)
		if err != nil {
			return nil, err
		}
		wp.SyslogTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Syslog
		return wp, nil

	} else if kf != nil {

		wrap, err := newKafkaTemplateWrap(kf)
		if err != nil {
			return nil, err
		}
		wp.KafkaTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.Kafka
		return wp, nil

	} else if ff != nil {

		wrap, err := newFluentForwarderTemplateWrap(ff)
		if err != nil {
			return nil, err
		}
		wp.FluentForwarderTemplateWrap = *wrap
		wp.CurrentTarget = loggingconfig.FluentForwarder
		return wp, nil

	} else if cc != nil {

		wrap := CustomTargetWrap{*cc}
		wp.CustomTargetWrap = wrap
		wp.CurrentTarget = loggingconfig.CustomTarget
		return wp, nil
	}

	return nil, nil
}

func newElasticsearchTemplateWrap(elasticsearchConfig *v3.ElasticsearchConfig) (*ElasticsearchTemplateWrap, error) {
	h, s, err := parseEndpoint(elasticsearchConfig.Endpoint)
	if err != nil {
		return nil, err
	}
	return &ElasticsearchTemplateWrap{
		ElasticsearchConfig: *elasticsearchConfig,
		Host:                h,
		Scheme:              s,
		DateFormat:          getDateFormat(elasticsearchConfig.DateFormat),
	}, nil
}

func newSplunkTemplateWrap(splunkConfig *v3.SplunkConfig) (*SplunkTemplateWrap, error) {
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

func newSyslogTemplateWrap(syslogConfig *v3.SyslogConfig) (*SyslogTemplateWrap, error) {
	host, port, err := net.SplitHostPort(syslogConfig.Endpoint)
	if err != nil {
		return nil, err
	}
	return &SyslogTemplateWrap{
		SyslogConfig: *syslogConfig,
		Host:         host,
		Port:         port,
	}, nil
}

func newKafkaTemplateWrap(kafkaConfig *v3.KafkaConfig) (*KafkaTemplateWrap, error) {
	wrap := &KafkaTemplateWrap{
		KafkaConfig: *kafkaConfig,
	}
	if len(kafkaConfig.BrokerEndpoints) == 0 && kafkaConfig.ZookeeperEndpoint == "" {
		err := errors.New("one of the kafka endpoint must be set")
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

func newFluentForwarderTemplateWrap(fluentForwarderConfig *v3.FluentForwarderConfig) (*FluentForwarderTemplateWrap, error) {
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
		return "", "", errors.Wrapf(err, "invalid endpoint %s", endpoint)
	}

	if u.Host == "" || u.Scheme == "" {
		return "", "", errors.New("invalid endpoint " + endpoint + " empty host or schema")
	}

	return u.Host, u.Scheme, nil
}
