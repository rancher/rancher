package constant

import (
	"fmt"

	cutils "github.com/rancher/rancher/pkg/catalog/utils"
)

const (
	AppName        = "rancher-logging"
	TesterAppName  = "rancher-logging-tester"
	AppInitVersion = "initializing"
	templateName   = "rancher-logging"
)

const (
	LoggingNamespace = "cattle-logging"
)

//daemonset, pod, container name
const (
	FluentdName                = "fluentd"
	FluentdHelperName          = "fluentd-helper"
	LogAggregatorName          = "log-aggregator"
	FluentdTesterName          = "fluentd-test"
	FluentdTesterContainerName = "dry-run"
)

//config
const (
	LoggingSecretName             = "fluentd"
	LoggingSSLSecretName          = "fluentd-ssl"
	LoggingSecretClusterConfigKey = "cluster.conf"
	LoggingSecretProjectConfigKey = "project.conf"
)

//target
const (
	Elasticsearch   = "elasticsearch"
	Splunk          = "splunk"
	Kafka           = "kafka"
	Syslog          = "syslog"
	FluentForwarder = "fluentforwarder"
	CustomTarget    = "customtarget"
	Graylog         = "graylog"
)

const (
	GoogleKubernetesEngine = "googleKubernetesEngine"
)

//ssl
const (
	DefaultCertDir = "/fluentd/etc/config/ssl"
	CaFileName     = "ca.pem"
	ClientCertName = "client-cert.pem"
	ClientKeyName  = "client-key.pem"
)

const (
	ClusterLevel = "cluster"
	ProjectLevel = "project"
)

var (
	FluentdTesterSelector = map[string]string{"app": "fluentd-tester"}
	FluentdSelector       = map[string]string{"app": "fluentd"}
	LogAggregatorSelector = map[string]string{"app": "log-aggregator"}
)

func SecretDataKeyCa(level, name string) string {
	return fmt.Sprintf("%s_%s_%s", level, name, CaFileName)
}

func SecretDataKeyCert(level, name string) string {
	return fmt.Sprintf("%s_%s_%s", level, name, ClientCertName)
}

func SecretDataKeyCertKey(level, name string) string {
	return fmt.Sprintf("%s_%s_%s", level, name, ClientKeyName)
}

func RancherLoggingTemplateID() string {
	return fmt.Sprintf("%s-%s", cutils.SystemLibraryName, templateName)
}

func RancherLoggingInitVersion() string {
	return fmt.Sprintf("%s-%s-%s", cutils.SystemLibraryName, templateName, AppInitVersion)
}

func RancherLoggingCatalogID(version string) string {
	return fmt.Sprintf(cutils.CatalogExternalIDFormat, cutils.SystemLibraryName, templateName, version)
}

func RancherLoggingConfigSecretName() string {
	return fmt.Sprintf("%s-%s", AppName, LoggingSecretName)
}

func RancherLoggingSSLSecretName() string {
	return fmt.Sprintf("%s-%s", AppName, LoggingSSLSecretName)
}

func GetNamespacePattern(namespace string) string {
	return fmt.Sprintf("^%s$", namespace)
}

func GetNamespacePathPattern(namespace string) string {
	return fmt.Sprintf("/var/log/containers/*_%s_*.log", namespace)
}
