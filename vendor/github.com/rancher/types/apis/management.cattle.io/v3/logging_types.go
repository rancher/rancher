package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterLogging struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec ClusterLoggingSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status ClusterLoggingStatus `json:"status"`
}

func (c *ClusterLogging) ObjClusterName() string {
	return c.Spec.ObjClusterName()
}

type ProjectLogging struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec ProjectLoggingSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status ProjectLoggingStatus `json:"status"`
}

func (p *ProjectLogging) ObjClusterName() string {
	return p.Spec.ObjClusterName()
}

type LoggingCommonField struct {
	DisplayName         string            `json:"displayName,omitempty"`
	OutputFlushInterval int               `json:"outputFlushInterval,omitempty" norman:"default=60"`
	OutputTags          map[string]string `json:"outputTags,omitempty"`
	EnableJSONParsing   bool              `json:"enableJSONParsing,omitempty"`
}

type LoggingTargets struct {
	ElasticsearchConfig   *ElasticsearchConfig   `json:"elasticsearchConfig,omitempty"`
	SplunkConfig          *SplunkConfig          `json:"splunkConfig,omitempty"`
	KafkaConfig           *KafkaConfig           `json:"kafkaConfig,omitempty"`
	SyslogConfig          *SyslogConfig          `json:"syslogConfig,omitempty"`
	FluentForwarderConfig *FluentForwarderConfig `json:"fluentForwarderConfig,omitempty"`
	CustomTargetConfig    *CustomTargetConfig    `json:"customTargetConfig,omitempty"`
}

type ClusterLoggingSpec struct {
	LoggingTargets
	LoggingCommonField
	ClusterName            string `json:"clusterName" norman:"type=reference[cluster]"`
	IncludeSystemComponent *bool  `json:"includeSystemComponent,omitempty" norman:"default=true"`
}

func (c *ClusterLoggingSpec) ObjClusterName() string {
	return c.ClusterName
}

type ProjectLoggingSpec struct {
	LoggingTargets
	LoggingCommonField
	ProjectName string `json:"projectName" norman:"type=reference[project]"`
}

func (p *ProjectLoggingSpec) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type ClusterLoggingStatus struct {
	Conditions  []LoggingCondition  `json:"conditions,omitempty"`
	AppliedSpec ClusterLoggingSpec  `json:"appliedSpec,omitempty"`
	FailedSpec  *ClusterLoggingSpec `json:"failedSpec,omitempty"`
}

type ProjectLoggingStatus struct {
	Conditions  []LoggingCondition `json:"conditions,omitempty"`
	AppliedSpec ProjectLoggingSpec `json:"appliedSpec,omitempty"`
}

var (
	LoggingConditionProvisioned condition.Cond = "Provisioned"
	LoggingConditionUpdated     condition.Cond = "Updated"
)

type LoggingCondition struct {
	// Type of cluster condition.
	Type condition.Cond `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

type ElasticsearchConfig struct {
	Endpoint      string `json:"endpoint,omitempty" norman:"required"`
	IndexPrefix   string `json:"indexPrefix,omitempty" norman:"required"`
	DateFormat    string `json:"dateFormat,omitempty" norman:"required,type=enum,options=YYYY-MM-DD|YYYY-MM|YYYY,default=YYYY-MM-DD"`
	AuthUserName  string `json:"authUsername,omitempty"`
	AuthPassword  string `json:"authPassword,omitempty" norman:"type=password"`
	Certificate   string `json:"certificate,omitempty"`
	ClientCert    string `json:"clientCert,omitempty"`
	ClientKey     string `json:"clientKey,omitempty"`
	ClientKeyPass string `json:"clientKeyPass,omitempty"`
	SSLVerify     bool   `json:"sslVerify,omitempty"`
	SSLVersion    string `json:"sslVersion,omitempty" norman:"type=enum,options=SSLv23|TLSv1|TLSv1_1|TLSv1_2,default=TLSv1_2"`
}

type SplunkConfig struct {
	Endpoint      string `json:"endpoint,omitempty" norman:"required"`
	Source        string `json:"source,omitempty"`
	Token         string `json:"token,omitempty" norman:"required,type=password"`
	Certificate   string `json:"certificate,omitempty"`
	ClientCert    string `json:"clientCert,omitempty"`
	ClientKey     string `json:"clientKey,omitempty"`
	ClientKeyPass string `json:"clientKeyPass,omitempty"`
	SSLVerify     bool   `json:"sslVerify,omitempty"`
	Index         string `json:"index,omitempty"`
}

type KafkaConfig struct {
	ZookeeperEndpoint  string   `json:"zookeeperEndpoint,omitempty"`
	BrokerEndpoints    []string `json:"brokerEndpoints,omitempty"`
	Topic              string   `json:"topic,omitempty" norman:"required"`
	Certificate        string   `json:"certificate,omitempty"`
	ClientCert         string   `json:"clientCert,omitempty"`
	ClientKey          string   `json:"clientKey,omitempty"`
	SaslUsername       string   `json:"saslUsername,omitempty"`
	SaslPassword       string   `json:"saslPassword,omitempty" norman:"type=password"`
	SaslScramMechanism string   `json:"saslScramMechanism,omitempty" norman:"type=enum,options=sha256|sha512"`
	SaslType           string   `json:"saslType,omitempty" norman:"type=enum,options=plain|scram"`
}

type SyslogConfig struct {
	Endpoint    string `json:"endpoint,omitempty" norman:"required"`
	Severity    string `json:"severity,omitempty" norman:"default=notice,type=enum,options=emerg|alert|crit|err|warning|notice|info|debug"`
	Program     string `json:"program,omitempty"`
	Protocol    string `json:"protocol,omitempty" norman:"default=udp,type=enum,options=udp|tcp"`
	Token       string `json:"token,omitempty" norman:"type=password"`
	EnableTLS   bool   `json:"enableTls,omitempty" norman:"default=false"`
	Certificate string `json:"certificate,omitempty"`
	ClientCert  string `json:"clientCert,omitempty"`
	ClientKey   string `json:"clientKey,omitempty"`
	SSLVerify   bool   `json:"sslVerify,omitempty"`
}

type FluentForwarderConfig struct {
	EnableTLS     bool           `json:"enableTls,omitempty" norman:"default=false"`
	Certificate   string         `json:"certificate,omitempty"`
	ClientCert    string         `json:"clientCert,omitempty"`
	ClientKey     string         `json:"clientKey,omitempty"`
	ClientKeyPass string         `json:"clientKeyPass,omitempty"`
	SSLVerify     bool           `json:"sslVerify,omitempty"`
	Compress      bool           `json:"compress,omitempty" norman:"default=true"`
	FluentServers []FluentServer `json:"fluentServers,omitempty" norman:"required"`
}

type FluentServer struct {
	Endpoint  string `json:"endpoint,omitempty" norman:"required"`
	Hostname  string `json:"hostname,omitempty"`
	Weight    int    `json:"weight,omitempty" norman:"default=100"`
	Standby   bool   `json:"standby,omitempty" norman:"default=false"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty" norman:"type=password"`
	SharedKey string `json:"sharedKey,omitempty" norman:"type=password"`
}

type CustomTargetConfig struct {
	Content     string `json:"content,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	ClientCert  string `json:"clientCert,omitempty"`
	ClientKey   string `json:"clientKey,omitempty"`
}

type ClusterTestInput struct {
	ClusterName string `json:"clusterId" norman:"required,type=reference[cluster]"`
	LoggingTargets
	OutputTags map[string]string `json:"outputTags,omitempty"`
}

func (c *ClusterTestInput) ObjClusterName() string {
	return c.ClusterName
}

type ProjectTestInput struct {
	ProjectName string `json:"projectId" norman:"required,type=reference[project]"`
	LoggingTargets
	OutputTags map[string]string `json:"outputTags,omitempty"`
}

func (p *ProjectTestInput) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}
