package utils

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
)

var (
	rubyCodeBlockReg = regexp.MustCompile(`#\{.*\}`)
)

type LoggingTargetTemplateWrap struct {
	CurrentTarget string
	ElasticsearchTemplateWrap
	SplunkTemplateWrap
	SyslogTemplateWrap
	KafkaTemplateWrap
	FluentForwarderTemplateWrap
	CustomTargetTemplateWrap
}

type ClusterLoggingTemplateWrap struct {
	v3.LoggingCommonField
	LoggingTargetTemplateWrap
	ExcludeNamespace       string
	IncludeSystemComponent bool
	WrapOutputTags         map[string]string
}

type ProjectLoggingTemplateWrap struct {
	ProjectName string
	v3.LoggingCommonField
	LoggingTargetTemplateWrap
	GrepNamespace   string
	IsSystemProject bool
	WrapProjectName string
	WrapOutputTags  map[string]string
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

	var wrapOutputTags map[string]string
	if logging.OutputTags != nil {
		if wrapOutputTags, err = ValidateCustomTags(logging.OutputTags, true); err != nil {
			return nil, err
		}
	}

	return &ClusterLoggingTemplateWrap{
		LoggingCommonField:        logging.LoggingCommonField,
		LoggingTargetTemplateWrap: *wrap,
		ExcludeNamespace:          excludeNamespace,
		IncludeSystemComponent:    includeSystemComponent,
		WrapOutputTags:            wrapOutputTags,
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

	var wrapOutputTags map[string]string
	if logging.OutputTags != nil {
		if wrapOutputTags, err = ValidateCustomTags(logging.OutputTags, true); err != nil {
			return nil, err
		}
	}

	wrapProjectName := strings.Replace(logging.ProjectName, ":", "_", -1)
	return &ProjectLoggingTemplateWrap{
		ProjectName:               logging.ProjectName,
		LoggingCommonField:        logging.LoggingCommonField,
		LoggingTargetTemplateWrap: *wrap,
		GrepNamespace:             grepNamespace,
		IsSystemProject:           isSystemProject,
		WrapProjectName:           wrapProjectName,
		WrapOutputTags:            wrapOutputTags,
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
	Host         string
	Port         string
	WrapSeverity string
}

type FluentForwarderTemplateWrap struct {
	v3.FluentForwarderConfig
	EnableShareKey bool
	FluentServers  []FluentServer
}

type CustomTargetTemplateWrap struct {
	v3.CustomTargetConfig
	WrapContent string
}

type FluentServer struct {
	Host string
	Port string
	v3.FluentServer
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

		wrap, err := newCustomTemplateWrap(cc)
		if err != nil {
			return nil, err
		}
		wp.CustomTargetTemplateWrap = *wrap
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
		WrapSeverity: getWrapSeverity(syslogConfig.Severity),
	}, nil
}

func newKafkaTemplateWrap(kafkaConfig *v3.KafkaConfig) (*KafkaTemplateWrap, error) {
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

func newCustomTemplateWrap(customTargetConfig *v3.CustomTargetConfig) (*CustomTargetTemplateWrap, error) {
	err := ValidateCustomTargetContent(customTargetConfig.Content)
	if err != nil {
		return nil, err
	}

	return &CustomTargetTemplateWrap{
		CustomTargetConfig: *customTargetConfig,
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

func ValidateCustomTargetContent(content string) error {
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		line := strings.TrimSpace(l)
		if line == "" {
			continue
		}

		lineArray := strings.Split(line, " ")
		if len(lineArray) == 0 {
			continue
		}

		key := lineArray[0]
		if err := filterFluentdTags(key); err != nil {
			return err
		}

		value := strings.TrimSpace(strings.TrimPrefix(line, key))
		if err := filterRubyCode(value); err != nil {
			return err
		}
	}

	return nil
}

func ValidateCustomTags(tags map[string]string, enableEscaped bool) (map[string]string, error) {
	newTags := make(map[string]string)
	for key, value := range tags {
		if err := filterFluentdTags(key); err != nil {
			return nil, err
		}

		if err := filterRubyCode(value); err != nil {
			return nil, err
		}

		if enableEscaped {
			newValue := escapeString(value)
			newTags[key] = newValue
		}
	}

	return newTags, nil
}

func filterFluentdTags(key string) error {
	invalidTagKeys := []string{
		"@include",
		"<source", "<parse", "<filter", "<format", "<storage", "<buffer", "<match", "<record", "<system", "<label", "<route",
		"</source>", "</parse>", "</filter>", "</format>", "</storage>", "</buffer>", "</match>", "</record>", "</system>", "</label>", "</route>",
	}

	for _, invalidKey := range invalidTagKeys {
		if strings.Contains(key, invalidKey) {
			return errors.New("invalid custom tag key: " + key)
		}
	}

	return nil
}

func filterRubyCode(s string) error {
	rubyCodeBlocks := rubyCodeBlockReg.FindStringSubmatch(s)
	if len(rubyCodeBlocks) > 0 {
		return errors.New("invalid custom field value: " + fmt.Sprintf("%v", rubyCodeBlocks))
	}
	return nil
}

func escapeString(postDoc string) string {
	var escapeReplacer = strings.NewReplacer(
		"\t", `\\t`,
		"\n", `\\n`,
		"\r", `\\r`,
		"\f", `\\f`,
		"\b", `\\b`,
		"\"", `\\\"`,
		"\\", `\\\\`,
	)

	escapeString := escapeReplacer.Replace(postDoc)
	return fmt.Sprintf(`"%s"`, escapeString)
}
