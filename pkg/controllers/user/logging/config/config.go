package constant

const (
	LoggingNamespace   = "cattle-logging"
	ClusterLoggingName = "cluster-logging"
	ProjectLoggingName = "project-logging"
)

//daemonset
const (
	FluentdName        = "fluentd"
	FluentdHelperName  = "fluentd-helper"
	FluentdHelperImage = "rancher/fluentd-helper:v0.1.1"
	FluentdImages      = "rancher/fluentd:v0.1.4"
)

//embedded
const (
	EmbeddedESName     = "elasticsearch"
	EmbeddedKibanaName = "kibana"
	ESImage            = "rancher/docker-elasticsearch-kubernetes:5.6.2"
	KibanaImage        = "kibana:5.6.4"
	BusyboxImage       = "busybox"
)

//configmap
const (
	ClusterFileName   = "cluster.conf"
	ProjectFileName   = "project.conf"
	ClusterConfigPath = "/tmp/cluster.conf"
	ProjectConfigPath = "/tmp/project.conf"
)

//target
const (
	Elasticsearch = "elasticsearch"
	Splunk        = "splunk"
	Kafka         = "kafka"
	Embedded      = "embedded"
	Syslog        = "syslog"
)

//app label
const (
	LabelK8sApp = "k8s-app"
)
