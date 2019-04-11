package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/rancher/types/apis/management.cattle.io/v3"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
)

type WrapLogging struct {
	CurrentTarget string
	WrapSyslog
	WrapSplunk
	WrapElasticsearch
	WrapKafka
}

type WrapClusterLogging struct {
	v3.ClusterLoggingSpec
	WrapEmbedded
	WrapLogging
	ContainerLogSourceTag string
}

type WrapProjectLogging struct {
	v3.ProjectLoggingSpec
	GrepNamespace string
	WrapLogging
	WrapProjectName       string
	ContainerLogSourceTag string
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
	Host   string
	Port   string
	Scheme string
}

type WrapKafka struct {
	Brokers   string
	Zookeeper string
}

type WrapSyslog struct {
	Host string
	Port string
}

func ToWrapClusterLogging(clusterLogging v3.ClusterLoggingSpec) (*WrapClusterLogging, error) {
	wp := WrapClusterLogging{
		ClusterLoggingSpec:    clusterLogging,
		ContainerLogSourceTag: loggingconfig.ClusterLevel,
	}

	wrapLogging, wem, err := GetWrapConfig(clusterLogging.ElasticsearchConfig, clusterLogging.SplunkConfig, clusterLogging.SyslogConfig, clusterLogging.KafkaConfig, clusterLogging.EmbeddedConfig)
	if err != nil {
		return nil, err
	}
	wp.WrapLogging = wrapLogging
	wp.WrapEmbedded = wem

	conf := map[string]interface{}{
		"clusterTarget": wp,
		"clusterName":   wp.ClusterName,
	}

	if err := generator.ValidateCustomTags(wp); err != nil {
		return nil, err
	}

	if err := generator.ValidateClusterOutput(conf); err != nil {
		return nil, err
	}

	if clusterLogging.SyslogConfig != nil && clusterLogging.SyslogConfig.Token != "" {
		if err := generator.ValidateClusterSyslogToken(conf); err != nil {
			return nil, err
		}
	}
	return &wp, nil
}

func ToWrapProjectLogging(grepNamespace string, projectLogging v3.ProjectLoggingSpec) (*WrapProjectLogging, error) {
	wp := WrapProjectLogging{
		ProjectLoggingSpec:    projectLogging,
		GrepNamespace:         grepNamespace,
		WrapProjectName:       strings.Replace(projectLogging.ProjectName, ":", "_", -1),
		ContainerLogSourceTag: projectLogging.ProjectName,
	}

	wrapLogging, _, err := GetWrapConfig(projectLogging.ElasticsearchConfig, projectLogging.SplunkConfig, projectLogging.SyslogConfig, projectLogging.KafkaConfig, nil)

	if err != nil {
		return nil, err
	}

	wp.WrapLogging = wrapLogging
	if err := generator.ValidateCustomTags(wp); err != nil {
		return nil, err
	}

	if err := generator.ValidateProjectOutput(wp); err != nil {
		return nil, err
	}

	if projectLogging.SyslogConfig != nil && projectLogging.SyslogConfig.Token != "" {
		if err := generator.ValidateProjectSyslogToken(wp); err != nil {
			return nil, err
		}
	}

	return &wp, nil
}

func GetWrapConfig(es *v3.ElasticsearchConfig, sp *v3.SplunkConfig, sl *v3.SyslogConfig, kf *v3.KafkaConfig, em *v3.EmbeddedConfig) (wrapLogging WrapLogging, wem WrapEmbedded, err error) {
	if es != nil {
		var h, s string
		h, s, err = parseEndpoint(es.Endpoint)
		if err != nil {
			return
		}
		err = testReachable("tcp", h)
		if err != nil {
			return
		}
		wrapLogging.WrapElasticsearch = WrapElasticsearch{
			Host:       h,
			Scheme:     s,
			DateFormat: getDateFormat(es.DateFormat),
		}
		wrapLogging.CurrentTarget = loggingconfig.Elasticsearch
	}

	if sp != nil {
		var h, s, host, port string
		h, s, err = parseEndpoint(sp.Endpoint)
		if err != nil {
			return
		}
		err = testReachable("tcp", h)
		if err != nil {
			return
		}

		host, port, err = net.SplitHostPort(h)
		if err != nil {
			return
		}
		wrapLogging.WrapSplunk = WrapSplunk{
			Host:   host,
			Port:   port,
			Scheme: s,
		}

		wrapLogging.CurrentTarget = loggingconfig.Splunk
	}

	if sl != nil {
		err = testReachable(sl.Protocol, sl.Endpoint)
		if err != nil {
			return
		}
		var host, port string
		host, port, err = net.SplitHostPort(sl.Endpoint)
		if err != nil {
			return
		}
		wrapLogging.WrapSyslog = WrapSyslog{
			Host: host,
			Port: port,
		}
		wrapLogging.CurrentTarget = loggingconfig.Syslog
	}

	if kf != nil {
		if len(kf.BrokerEndpoints) == 0 && kf.ZookeeperEndpoint == "" {
			err = errors.New("one of the kafka endpoint must be set")
			return
		}
		if len(kf.BrokerEndpoints) != 0 {
			var bs []string
			var h string
			for _, v := range kf.BrokerEndpoints {
				h, _, err = parseEndpoint(v)
				if err != nil {
					return
				}
				err = testReachable("tcp", h)
				if err != nil {
					return
				}
				bs = append(bs, h)
			}
			wrapLogging.WrapKafka = WrapKafka{
				Brokers: strings.Join(bs, ","),
			}
		} else {
			if kf.ZookeeperEndpoint != "" {
				var h string
				if h, _, err = parseEndpoint(kf.ZookeeperEndpoint); err != nil {
					return
				}
				err = testReachable("tcp", h)
				if err != nil {
					return
				}
				wrapLogging.WrapKafka = WrapKafka{
					Zookeeper: h,
				}
			}
		}
		wrapLogging.CurrentTarget = loggingconfig.Kafka
	}

	if em != nil {
		if em.LimitsCPU != 0 && em.LimitsCPU < em.RequestsCPU {
			err = fmt.Errorf("limits cpu %d is less than request cpu %d", em.LimitsCPU, em.RequestsCPU)
			return
		}
		if em.LimitsMemery != 0 && em.LimitsMemery < em.RequestsMemery {
			err = fmt.Errorf("limits memory %d is less than request memory %d", em.LimitsMemery, em.RequestsMemery)
			return
		}
		wem = WrapEmbedded{
			DateFormat: getDateFormat(em.DateFormat),
		}
		wrapLogging.CurrentTarget = loggingconfig.Embedded
	}
	return
}

func parseEndpoint(endpoint string) (host string, scheme string, err error) {
	u, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return "", "", errors.Wrapf(err, "invalid endpoint %s", endpoint)
	}

	if u.Host == "" || u.Scheme == "" {
		return "", "", fmt.Errorf("invalid endpoint %s, empty host or schema", endpoint)
	}

	return u.Host, u.Scheme, nil
}

func getDateFormat(dateformat string) string {
	ToRealMap := map[string]string{
		"YYYY-MM-DD": "%Y-%m-%d",
		"YYYY-MM":    "%Y-%m",
		"YYYY":       "%Y",
	}
	if _, ok := ToRealMap[dateformat]; ok {
		return ToRealMap[dateformat]
	}
	return "%Y-%m-%d"
}

func testReachable(network string, url string) error {
	timeout := time.Duration(10 * time.Second)
	conn, err := net.DialTimeout(network, url, timeout)
	if err != nil {
		return fmt.Errorf("url %s unreachable, error: %v", url, err)
	}
	conn.Close()
	return nil
}

func GetClusterTarget(spec v3.ClusterLoggingSpec) string {
	if spec.EmbeddedConfig != nil {
		return loggingconfig.Embedded
	} else if spec.ElasticsearchConfig != nil {
		return loggingconfig.Elasticsearch
	} else if spec.SplunkConfig != nil {
		return loggingconfig.Splunk
	} else if spec.KafkaConfig != nil {
		return loggingconfig.Kafka
	} else if spec.SyslogConfig != nil {
		return loggingconfig.Syslog
	}
	return "none"
}

func GetProjectTarget(spec v3.ProjectLoggingSpec) string {
	if spec.ElasticsearchConfig != nil {
		return loggingconfig.Elasticsearch
	} else if spec.SplunkConfig != nil {
		return loggingconfig.Splunk
	} else if spec.KafkaConfig != nil {
		return loggingconfig.Kafka
	} else if spec.SyslogConfig != nil {
		return loggingconfig.Syslog
	}
	return "none"
}
