package constant

const (
	LoggingNamespace   = "cattle-logging"
	ClusterLoggingName = "cluster-logging"
	ProjectLoggingName = "project-logging"
)

//daemonset
const (
	FluentdName       = "fluentd"
	FluentdHelperName = "fluentd-helper"
	LogAggregatorName = "log-aggregator"
)

//embedded
const (
	EmbeddedESName     = "elasticsearch"
	EmbeddedKibanaName = "kibana"
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

const (
	GoogleKubernetesEngine = "googleKubernetesEngine"
)

//ssl
const (
	SSLSecretName  = "sslconfig"
	CaFileName     = "ca.pem"
	ClientCertName = "client-cert.pem"
	ClientKeyName  = "client-key.pem"
)
