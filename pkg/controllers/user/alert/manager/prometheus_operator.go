package manager

import (
	"context"
	"fmt"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rmonitoringv1 "github.com/rancher/types/apis/monitoring.coreos.com/v1"
	"github.com/rancher/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	ComparisonEqual          = "equal"
	ComparisonNotEqual       = "not-equal"
	ComparisonGreaterThan    = "greater-than"
	ComparisonLessThan       = "less-than"
	ComparisonGreaterOrEqual = "greater-or-equal"
	ComparisonLessOrEqual    = "less-or-equal"
)

var (
	comparisonMap = map[string]string{
		ComparisonEqual:          "==",
		ComparisonNotEqual:       "!=",
		ComparisonGreaterThan:    ">",
		ComparisonLessThan:       "<",
		ComparisonGreaterOrEqual: ">=",
		ComparisonGreaterOrEqual: "<=",
	}
)

type PromOperatorCRDManager struct {
	clusterName        string
	NodeLister         v3.NodeLister
	PrometheusRules    rmonitoringv1.PrometheusRuleInterface
	podLister          v1.PodLister
	workloadController workload.CommonController
}

type ResourceQueryCondition interface {
	GetQueryCondition(nameSelector string, labelSelector map[string]string) (interface{}, error)
}

type NodeQueryCondition struct {
	clusterName string
	NodeLister  v3.NodeLister
}

type ClusterQueryCondition struct{}

type WorkloadQueryCondition struct {
	podLister v1.PodLister
}

func NewPrometheusCRDManager(ctx context.Context, cluster *config.UserContext) *PromOperatorCRDManager {
	return &PromOperatorCRDManager{
		clusterName:        cluster.ClusterName,
		NodeLister:         cluster.Management.Management.Nodes(cluster.ClusterName).Controller().Lister(),
		PrometheusRules:    cluster.Monitoring.PrometheusRules(metav1.NamespaceAll),
		workloadController: workload.NewWorkloadController(ctx, cluster.UserOnlyContext(), nil),
	}
}

func GetDefaultPrometheusRule(namespace, name string) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				monitorutil.CattlePrometheusRuleLabelKey: monitorutil.CattleAlertingPrometheusRuleLabelValue,
			},
		},
	}
}

func AddRuleGroup(promRule *monitoringv1.PrometheusRule, ruleGroup monitoringv1.RuleGroup) {
	if promRule.Spec.Groups == nil {
		promRule.Spec.Groups = []monitoringv1.RuleGroup{ruleGroup}
		return
	}

	for _, v := range promRule.Spec.Groups {
		if v.Name == ruleGroup.Name {
			v = ruleGroup
			return
		}
	}
	promRule.Spec.Groups = append(promRule.Spec.Groups, ruleGroup)
}

func (c *PromOperatorCRDManager) GetRuleGroup(name string) *monitoringv1.RuleGroup {
	return &monitoringv1.RuleGroup{
		Name: name,
	}
}

func (c *PromOperatorCRDManager) AddRule(ruleGroup *monitoringv1.RuleGroup, groupID, ruleID, displayName, serverity string, metric *v3.MetricRule) {
	r := Metric2Rule(groupID, ruleID, displayName, serverity, c.clusterName, metric)
	ruleGroup.Rules = append(ruleGroup.Rules, r)
}

func Metric2Rule(groupID, ruleID, serverity, displayName, clusterName string, metric *v3.MetricRule) monitoringv1.Rule {
	expr := getExpr(metric.Expression, metric.Comparison, metric.ThresholdValue)
	labels := map[string]string{
		"alert_type":   "metric",
		"group_id":     groupID,
		"cluster_name": clusterName,
		"rule_id":      ruleID,
		"severity":     serverity,
		"duration":     metric.Duration,
		"expression":   expr,
	}

	return monitoringv1.Rule{
		Alert:  displayName,
		Expr:   intstr.FromString(expr),
		For:    metric.Duration,
		Labels: labels,
	}
}

func getExpr(expr, comparison string, thresholdValue float64) string {
	if comparison != "" {
		return fmt.Sprintf("%s%s%v", expr, comparisonMap[comparison], thresholdValue)
	}
	return expr
}
