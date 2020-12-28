package manager

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	rmonitoringv1 "github.com/rancher/rancher/pkg/generated/norman/monitoring.coreos.com/v1"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	ComparisonHasValue       = "has-value"
	ComparisonEqual          = "equal"
	ComparisonNotEqual       = "not-equal"
	ComparisonGreaterThan    = "greater-than"
	ComparisonLessThan       = "less-than"
	ComparisonGreaterOrEqual = "greater-or-equal"
	ComparisonLessOrEqual    = "less-or-equal"
)

var (
	comparisonMap = map[string]string{
		ComparisonHasValue:       "",
		ComparisonEqual:          "==",
		ComparisonNotEqual:       "!=",
		ComparisonGreaterThan:    ">",
		ComparisonLessThan:       "<",
		ComparisonGreaterOrEqual: ">=",
		ComparisonLessOrEqual:    "<=",
	}
)

type PromOperatorCRDManager struct {
	clusterName     string
	namespaces      v1.NamespaceInterface
	prometheusRules rmonitoringv1.PrometheusRuleInterface
}

func NewPrometheusCRDManager(ctx context.Context, cluster *config.UserContext) *PromOperatorCRDManager {
	return &PromOperatorCRDManager{
		clusterName:     cluster.ClusterName,
		namespaces:      cluster.Core.Namespaces(metav1.NamespaceAll),
		prometheusRules: cluster.Monitoring.PrometheusRules(metav1.NamespaceAll),
	}
}

func (c *PromOperatorCRDManager) GetDefaultPrometheusRule(namespace, name string) *monitoringv1.PrometheusRule {
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

func (c *PromOperatorCRDManager) DeletePrometheusRule(namespace, name string) error {
	if err := c.prometheusRules.DeleteNamespaced(namespace, name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete prometheus rule %s:%s failed, %v", namespace, name, err)
	}
	return nil
}

func (c *PromOperatorCRDManager) SyncPrometheusRule(promRule *monitoringv1.PrometheusRule) error {
	if len(promRule.Spec.Groups) == 0 {
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: promRule.Namespace,
		},
	}
	if _, err := c.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("get namespace %s for prometheus rule %s:%s failed, %v", ns.Name, promRule.Namespace, promRule.Name, err)
	}

	old, err := c.prometheusRules.GetNamespaced(promRule.Namespace, promRule.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get prometheus rule %s:%s failed, %v", promRule.Namespace, promRule.Name, err)
		}

		if _, err = c.prometheusRules.Create(promRule); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create prometheus rule %s:%s failed, %v", promRule.Namespace, promRule.Name, err)
		}
		return nil
	}

	sortedNewGroups := promRule.Spec.Groups
	sortedOldGroups := old.Spec.Groups
	sortGroups(sortedNewGroups)
	sortGroups(sortedOldGroups)

	if !reflect.DeepEqual(sortedOldGroups, sortedNewGroups) {

		updated := old.DeepCopy()
		updated.Spec.Groups = sortedNewGroups

		if _, err = c.prometheusRules.Update(updated); err != nil {
			return fmt.Errorf("update prometheus rule %s:%s failed, %v", updated.Namespace, updated.Name, err)
		}
	}
	return nil
}

func (c *PromOperatorCRDManager) AddRuleGroup(promRule *monitoringv1.PrometheusRule, ruleGroup monitoringv1.RuleGroup) {
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

func (c *PromOperatorCRDManager) AddRule(ruleGroup *monitoringv1.RuleGroup, rule monitoringv1.Rule) {
	ruleGroup.Rules = append(ruleGroup.Rules, rule)
}

func Metric2Rule(groupID, ruleID, serverity, displayName, clusterName, projectName string, metric *v32.MetricRule) monitoringv1.Rule {
	expr := getExpr(metric.Expression, metric.Comparison, metric.ThresholdValue)
	comp := strings.Replace(metric.Comparison, "-", " ", -1)
	labels := map[string]string{
		"alert_type":      "metric",
		"alert_name":      displayName,
		"group_id":        groupID,
		"cluster_name":    clusterName,
		"rule_id":         ruleID,
		"severity":        serverity,
		"duration":        metric.Duration,
		"expression":      expr,
		"threshold_value": fmt.Sprintf("%v", metric.ThresholdValue),
		"comparison":      comp,
	}

	annotation := map[string]string{
		"current_value": "{{ .Value }}",
	}

	if projectName != "" {
		labels["project_name"] = projectName
		labels["level"] = "project"
	}

	return monitoringv1.Rule{
		Alert:       displayName,
		Expr:        intstr.FromString(expr),
		For:         metric.Duration,
		Labels:      labels,
		Annotations: annotation,
	}
}

func getExpr(expr, comparison string, thresholdValue float64) string {
	if comparison != ComparisonHasValue {
		return fmt.Sprintf("%s%s%v", expr, comparisonMap[comparison], thresholdValue)
	}
	return expr
}

func sortGroups(groups []monitoringv1.RuleGroup) {
	for _, v := range groups {
		sortedRules := v.Rules
		sort.Slice(sortedRules, func(k, l int) bool {
			return sortedRules[k].Labels["rule_id"] < sortedRules[l].Labels["rule_id"]
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
}
