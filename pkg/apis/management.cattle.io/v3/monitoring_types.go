package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MonitoringStatus struct {
	GrafanaEndpoint string                `json:"grafanaEndpoint,omitempty"`
	Conditions      []MonitoringCondition `json:"conditions,omitempty"`
}

type MonitoringCondition struct {
	// Type of cluster condition.
	Type ClusterConditionType `json:"type"`
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

const (
	MonitoringConditionGrafanaDeployed           condition.Cond = "GrafanaDeployed"
	MonitoringConditionPrometheusDeployed        condition.Cond = "PrometheusDeployed"
	MonitoringConditionAlertmaanagerDeployed     condition.Cond = "AlertmanagerDeployed"
	MonitoringConditionNodeExporterDeployed      condition.Cond = "NodeExporterDeployed"
	MonitoringConditionKubeStateExporterDeployed condition.Cond = "KubeStateExporterDeployed"
	MonitoringConditionMetricExpressionDeployed  condition.Cond = "MetricExpressionDeployed"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterMonitorGraph struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterMonitorGraphSpec `json:"spec"`
}

func (c *ClusterMonitorGraph) ObjClusterName() string {
	return c.Spec.ObjClusterName()
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ProjectMonitorGraph struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProjectMonitorGraphSpec `json:"spec"`
}

func (p *ProjectMonitorGraph) ObjClusterName() string {
	return p.Spec.ObjClusterName()
}

type ClusterMonitorGraphSpec struct {
	ClusterName         string `json:"clusterName" norman:"type=reference[cluster]"`
	ResourceType        string `json:"resourceType,omitempty"  norman:"type=enum,options=node|cluster|etcd|apiserver|scheduler|controllermanager|fluentd|istiocluster|istioproject"`
	DisplayResourceType string `json:"displayResourceType,omitempty" norman:"type=enum,options=node|cluster|etcd|kube-component|rancher-component"`
	CommonMonitorGraphSpec
}

func (c *ClusterMonitorGraphSpec) ObjClusterName() string {
	return c.ClusterName
}

type ProjectMonitorGraphSpec struct {
	ProjectName         string `json:"projectName" norman:"type=reference[project]"`
	ResourceType        string `json:"resourceType,omitempty" norman:"type=enum,options=workload|pod|container"`
	DisplayResourceType string `json:"displayResourceType,omitempty" norman:"type=enum,options=workload|pod|container"`
	CommonMonitorGraphSpec
}

func (p *ProjectMonitorGraphSpec) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type CommonMonitorGraphSpec struct {
	Description            string            `json:"description,omitempty"`
	MetricsSelector        map[string]string `json:"metricsSelector,omitempty"`
	DetailsMetricsSelector map[string]string `json:"detailsMetricsSelector,omitempty"`
	YAxis                  YAxis             `json:"yAxis,omitempty"`
	Priority               int               `json:"priority,omitempty"`
	GraphType              string            `json:"graphType,omitempty" norman:"type=enum,options=graph|singlestat"`
}

type YAxis struct {
	Unit string `json:"unit,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MonitorMetric struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MonitorMetricSpec `json:"spec"`
}

type MonitorMetricSpec struct {
	Expression   string `json:"expression,omitempty" norman:"required"`
	LegendFormat string `json:"legendFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

type QueryGraphInput struct {
	From         string            `json:"from,omitempty"`
	To           string            `json:"to,omitempty"`
	Interval     string            `json:"interval,omitempty"`
	MetricParams map[string]string `json:"metricParams,omitempty"`
	Filters      map[string]string `json:"filters,omitempty"`
	IsDetails    bool              `json:"isDetails,omitempty"`
}

type QueryClusterGraphOutput struct {
	Type string              `json:"type,omitempty"`
	Data []QueryClusterGraph `json:"data,omitempty"`
}

type QueryClusterGraph struct {
	GraphName string        `json:"graphID" norman:"type=reference[clusterMonitorGraph]"`
	Series    []*TimeSeries `json:"series" norman:"type=array[reference[timeSeries]]"`
}

type QueryProjectGraphOutput struct {
	Type string              `json:"type,omitempty"`
	Data []QueryProjectGraph `json:"data,omitempty"`
}

type QueryProjectGraph struct {
	GraphName string        `json:"graphID" norman:"type=reference[projectMonitorGraph]"`
	Series    []*TimeSeries `json:"series" norman:"type=array[reference[timeSeries]]"`
}

type QueryClusterMetricInput struct {
	ClusterName string `json:"clusterId" norman:"type=reference[cluster]"`
	CommonQueryMetricInput
}

func (q *QueryClusterMetricInput) ObjClusterName() string {
	return q.ClusterName
}

type QueryProjectMetricInput struct {
	ProjectName string `json:"projectId" norman:"type=reference[project]"`
	CommonQueryMetricInput
}

func (q *QueryProjectMetricInput) ObjClusterName() string {
	if parts := strings.SplitN(q.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type CommonQueryMetricInput struct {
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Interval string `json:"interval,omitempty"`
	Expr     string `json:"expr,omitempty" norman:"required"`
}

type QueryMetricOutput struct {
	Type   string        `json:"type,omitempty"`
	Series []*TimeSeries `json:"series" norman:"type=array[reference[timeSeries]]"`
}

type TimeSeries struct {
	Name   string      `json:"name"`
	Points [][]float64 `json:"points" norman:"type=array[array[float]]"`
}

type MetricNamesOutput struct {
	Type  string   `json:"type,omitempty"`
	Names []string `json:"names" norman:"type=array[string]"`
}

type ClusterMetricNamesInput struct {
	ClusterName string `json:"clusterId" norman:"type=reference[cluster]"`
}

func (c *ClusterMetricNamesInput) ObjClusterName() string {
	return c.ClusterName
}

type ProjectMetricNamesInput struct {
	ProjectName string `json:"projectId" norman:"type=reference[project]"`
}

func (p *ProjectMetricNamesInput) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}
