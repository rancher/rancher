package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

type WrapClusterLogging struct {
	CurrentTarget string
	v3.ClusterLoggingSpec
	WrapSyslog
	WrapSplunk
	WrapEmbedded
	WrapElasticsearch
	WrapKafka
}

type WrapProjectLogging struct {
	CurrentTarget string
	v3.ProjectLoggingSpec
	GrepNamespace string
	WrapSyslog
	WrapSplunk
	WrapElasticsearch
	WrapKafka
}

type WrapEmbedded struct {
	DateFormat string
}

type WrapElasticsearch struct {
	DateFormat string
	Host       string
	Scheme     string
}

type WrapSplunk struct {
	Server string
	Scheme string
}

type WrapKafka struct {
	Brokers string
}

type WrapSyslog struct {
	Host string
	Port string
}

func (w *WrapClusterLogging) Validate() error {
	curtg, _, _, _, _, err := getWrapConfig(w.ElasticsearchConfig, w.SplunkConfig, w.SyslogConfig, w.KafkaConfig)
	if err != nil {
		return err
	}
	if w.EmbeddedConfig != nil {
		curtg = loggingconfig.Embedded
	}

	if curtg == "" {
		return fmt.Errorf("one of the target must set")
	}
	return err
}

func (w *WrapProjectLogging) Validate() error {
	curtg, _, _, _, _, err := getWrapConfig(w.ElasticsearchConfig, w.SplunkConfig, w.SyslogConfig, w.KafkaConfig)
	if curtg == "" {
		return fmt.Errorf("one of the target must set")
	}
	return err
}

func ToWrapClusterLogging(clusterLogging v3.ClusterLoggingSpec) (*WrapClusterLogging, error) {
	wp := WrapClusterLogging{
		ClusterLoggingSpec: clusterLogging,
	}

	curtg, wes, wsp, wsl, wkf, err := getWrapConfig(clusterLogging.ElasticsearchConfig, clusterLogging.SplunkConfig, clusterLogging.SyslogConfig, clusterLogging.KafkaConfig)
	if err != nil {
		return nil, err
	}
	wp.WrapElasticsearch = wes
	wp.WrapSplunk = wsp
	wp.WrapSyslog = wsl
	wp.WrapKafka = wkf

	if clusterLogging.EmbeddedConfig != nil {
		curtg = loggingconfig.Embedded
		wp.WrapEmbedded.DateFormat = getDateFormat(clusterLogging.EmbeddedConfig.DateFormat)
	}
	wp.CurrentTarget = curtg
	return &wp, nil
}

func ToWrapProjectLogging(grepNamespace string, projectLogging v3.ProjectLoggingSpec) (*WrapProjectLogging, error) {
	wp := WrapProjectLogging{
		ProjectLoggingSpec: projectLogging,
		GrepNamespace:      grepNamespace,
	}

	curtg, wes, wsp, wsl, wkf, err := getWrapConfig(projectLogging.ElasticsearchConfig, projectLogging.SplunkConfig, projectLogging.SyslogConfig, projectLogging.KafkaConfig)

	if err != nil {
		return nil, err
	}
	wp.CurrentTarget = curtg
	wp.WrapElasticsearch = wes
	wp.WrapSplunk = wsp
	wp.WrapSyslog = wsl
	wp.WrapKafka = wkf

	return &wp, nil
}

func getWrapConfig(es *v3.ElasticsearchConfig, sp *v3.SplunkConfig, sl *v3.SyslogConfig, kf *v3.KafkaConfig) (currentTarget string, wes WrapElasticsearch, wsp WrapSplunk, wsl WrapSyslog, wkf WrapKafka, err error) {
	if es != nil {
		currentTarget = loggingconfig.Elasticsearch
		var u *url.URL
		u, err = url.Parse(es.Endpoint)
		if err != nil {
			return
		}
		wes = WrapElasticsearch{
			Host:       u.Host,
			Scheme:     u.Scheme,
			DateFormat: getDateFormat(es.DateFormat),
		}
	}

	if sp != nil {
		currentTarget = loggingconfig.Splunk
		var u *url.URL
		u, err = url.Parse(sp.Endpoint)
		if err != nil {
			return
		}
		wsp = WrapSplunk{
			Server: u.Host,
			Scheme: u.Scheme,
		}
	}

	if sl != nil {
		currentTarget = loggingconfig.Syslog
		var host, port string
		host, port, err = net.SplitHostPort(sl.Endpoint)
		if err != nil {
			return
		}
		wsl = WrapSyslog{
			Host: host,
			Port: port,
		}
	}

	if kf != nil && len(kf.BrokerEndpoints) != 0 {
		currentTarget = loggingconfig.Kafka
		wkf = WrapKafka{
			Brokers: strings.Join(kf.BrokerEndpoints, ","),
		}
	}
	return
}

func getDateFormat(dateformat string) string {
	ToRealMap := map[string]string{
		"YYYY.MM.DD": "%Y.%m.%d",
		"YYYY.MM":    "%Y.%m.",
		"YYYY":       "%Y.",
	}
	if _, ok := ToRealMap[dateformat]; ok {
		return ToRealMap[dateformat]
	}
	return "%Y.%m.%d"
}

func GetClusterTarget(spec v3.ClusterLoggingSpec) string {
	if spec.EmbeddedConfig != nil {
		return "embedded"
	} else if spec.ElasticsearchConfig != nil {
		return "elasticsearch"
	} else if spec.SplunkConfig != nil {
		return "splunk"
	} else if spec.KafkaConfig != nil {
		return "kafka"
	} else if spec.SyslogConfig != nil {
		return "syslog"
	}
	return "none"
}
