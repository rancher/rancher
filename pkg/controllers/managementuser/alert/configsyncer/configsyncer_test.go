package configsyncer

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/alert/manager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	clusterAlertTests = []struct {
		caseName     string
		in           map[string][]*v3.ClusterAlertRule
		outGroupBy   []model.LabelName
		outTimeField v32.TimingField
	}{
		{"event alert", eventRulesMap, eventGroupBy, eventTimingField},
		{"node alert", nodeRulesMap, nodeGroupBy, defaultTimingField},
		{"system service alert", systemServiceRulesMap, systemServiceGroupBy, defaultTimingField},
		{"metric alert", metricRulesMap, metricGroupBy, defaultTimingField},
	}
)

func TestAddClusterAlert2Config(t *testing.T) {

	for _, tt := range clusterAlertTests {
		keys := []string{groupID}
		config := manager.GetAlertManagerDefaultConfig()

		configSyncer := ConfigSyncer{
			clusterName: clusterName,
		}

		if err := configSyncer.addClusterAlert2Config(config, tt.in, keys, clusterGroupMap, notifiers); err != nil {
			t.Error(err)
			return
		}

		if len(config.Route.Routes) == 0 {
			t.Errorf("test %s failed, routes is empty", tt.caseName)
		}

		if len(config.Route.Routes[0].Routes) == 0 {
			t.Errorf("test %s failed, sub routes is empty", tt.caseName)
		}

		subRoute := config.Route.Routes[0].Routes[0]
		if !reflect.DeepEqual(subRoute.GroupBy, tt.outGroupBy) {
			t.Errorf("test %s failed, expect group by %v, actual %v", tt.caseName, tt.outGroupBy, subRoute.GroupBy)
		}

		if *subRoute.GroupWait != model.Duration(time.Duration(tt.outTimeField.GroupWaitSeconds)*time.Second) {
			t.Errorf("test %s failed, expect group wait %v, actual %v", tt.caseName, tt.outTimeField.GroupWaitSeconds, subRoute.GroupWait)
		}

		if *subRoute.GroupInterval != model.Duration(time.Duration(tt.outTimeField.GroupIntervalSeconds)*time.Second) {
			t.Errorf("test %s failed, expect group interval %v, actual %v", tt.caseName, tt.outTimeField.GroupIntervalSeconds, subRoute.GroupInterval)
		}

		if *subRoute.RepeatInterval != model.Duration(time.Duration(tt.outTimeField.RepeatIntervalSeconds)*time.Second) {
			t.Errorf("test %s failed, expect repeat interval %v, actual %v", tt.caseName, tt.outTimeField.RepeatIntervalSeconds, subRoute.RepeatInterval)
		}
	}

}

var (
	projectAlertTests = []struct {
		caseName     string
		in           map[string]map[string][]*v3.ProjectAlertRule
		outGroupBy   []model.LabelName
		outTimeField v32.TimingField
	}{
		{"pod alert", podRulesMap, podGroupBy, defaultTimingField},
		{"workload alert", workloadRulesMap, workloadGroupBy, defaultTimingField},
		{"metric alert", projectMetricRulesMap, projectMetricGroupBy, defaultTimingField},
	}
)

func TestAddProjectAlert2Config(t *testing.T) {

	for _, tt := range projectAlertTests {
		keys := []string{projectID}
		config := manager.GetAlertManagerDefaultConfig()

		configSyncer := ConfigSyncer{
			clusterName: clusterName,
		}

		if err := configSyncer.addProjectAlert2Config(config, tt.in, keys, projectGroupMap, notifiers); err != nil {
			t.Error(err)
			return
		}

		fmt.Println(config.String())

		if len(config.Route.Routes) == 0 {
			t.Errorf("test %s failed, routes is empty", tt.caseName)
		}

		if len(config.Route.Routes[0].Routes) == 0 {
			t.Errorf("test %s failed, sub routes is empty", tt.caseName)
		}

		subRoute := config.Route.Routes[0].Routes[0]
		if !reflect.DeepEqual(subRoute.GroupBy, tt.outGroupBy) {
			t.Errorf("test %s failed, expect group by %v, actual %v", tt.caseName, tt.outGroupBy, subRoute.GroupBy)
		}

		if *subRoute.GroupWait != model.Duration(time.Duration(tt.outTimeField.GroupWaitSeconds)*time.Second) {
			t.Errorf("test %s failed, expect group wait %v, actual %v", tt.caseName, tt.outTimeField.GroupWaitSeconds, subRoute.GroupWait)
		}

		if *subRoute.GroupInterval != model.Duration(time.Duration(tt.outTimeField.GroupIntervalSeconds)*time.Second) {
			t.Errorf("test %s failed, expect group interval %v, actual %v", tt.caseName, tt.outTimeField.GroupIntervalSeconds, subRoute.GroupInterval)
		}

		if *subRoute.RepeatInterval != model.Duration(time.Duration(tt.outTimeField.RepeatIntervalSeconds)*time.Second) {
			t.Errorf("test %s failed, expect repeat interval %v, actual %v", tt.caseName, tt.outTimeField.RepeatIntervalSeconds, subRoute.RepeatInterval)
		}
	}

}

var (
	clusterName = "testCluster"
	projectName = "testProject"
	projectID   = clusterName + ":" + projectName
	groupID     = "testcluster:testGroup"
	displayName = "test"
	serverity   = "critical"
	inherited   = false

	defaultTimingField = v32.TimingField{
		GroupWaitSeconds:      30,
		GroupIntervalSeconds:  180,
		RepeatIntervalSeconds: 3600,
	}

	commonRuleField = v32.CommonRuleField{
		DisplayName: displayName,
		Severity:    serverity,
		Inherited:   &inherited,
		TimingField: defaultTimingField,
	}

	alertStatus = v32.AlertStatus{
		AlertState: "active",
	}
)

var (
	commonGroupField = v32.CommonGroupField{
		DisplayName: displayName,
		TimingField: defaultTimingField,
	}

	clusterGroupMap = map[string]*v3.ClusterAlertGroup{
		groupID: &v3.ClusterAlertGroup{
			Spec: v32.ClusterGroupSpec{
				ClusterName:      clusterName,
				CommonGroupField: commonGroupField,
				Recipients:       recipients,
			},
			Status: alertStatus,
		},
	}

	projectGroupMap = map[string]*v3.ProjectAlertGroup{
		groupID: &v3.ProjectAlertGroup{
			Spec: v32.ProjectGroupSpec{
				ProjectName:      projectID,
				CommonGroupField: commonGroupField,
				Recipients:       recipients,
			},
			Status: alertStatus,
		},
	}
)

var (
	defaultChannel = "testChannel"
	slack          = "slack"
	namespace      = "cattle-alerting"
	notifiers      = []*v3.Notifier{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      slack,
				Namespace: namespace,
			},
			Spec: v32.NotifierSpec{
				ClusterName: clusterName,
				DisplayName: displayName,
				SlackConfig: &v32.SlackConfig{
					DefaultRecipient: defaultChannel,
					URL:              "www.slack.com",
				},
			},
		},
	}

	recipients = []v32.Recipient{
		{
			Recipient:    defaultChannel,
			NotifierName: clusterName + ":" + slack,
			NotifierType: slack,
		},
	}
)

// event
var (
	name = "eventRule"

	podEventRule = v32.EventRule{
		EventType:    "normal",
		ResourceKind: "Pod",
	}

	eventTimingField = v32.TimingField{
		GroupWaitSeconds:      eventGroupWait,
		GroupIntervalSeconds:  eventGroupInterval,
		RepeatIntervalSeconds: eventRepeatInterval,
	}

	eventCommonRuleField = v32.CommonRuleField{
		DisplayName: displayName,
		Severity:    serverity,
		Inherited:   &inherited,
		TimingField: eventTimingField,
	}

	eventAlert = v3.ClusterAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v32.ClusterAlertRuleSpec{
			ClusterName:     clusterName,
			GroupName:       groupID,
			CommonRuleField: eventCommonRuleField,
			EventRule:       &podEventRule,
		},
		Status: alertStatus,
	}

	eventRulesMap = map[string][]*v3.ClusterAlertRule{
		groupID: {
			&eventAlert,
		},
	}

	eventGroupBy = getClusterAlertGroupBy(eventAlert.Spec)
)

//node
var (
	nodeRule = v32.NodeRule{
		NodeName:  "node1",
		Condition: "notready",
	}

	nodeAlert = v3.ClusterAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nodeRule",
			Namespace: namespace,
		},
		Spec: v32.ClusterAlertRuleSpec{
			ClusterName:     clusterName,
			GroupName:       groupID,
			CommonRuleField: commonRuleField,
			NodeRule:        &nodeRule,
		},
		Status: alertStatus,
	}

	nodeRulesMap = map[string][]*v3.ClusterAlertRule{
		groupID: {
			&nodeAlert,
		},
	}

	nodeGroupBy = getClusterAlertGroupBy(nodeAlert.Spec)
)

//system service
var (
	systemServiceRule = v32.SystemServiceRule{
		Condition: "etcd",
	}

	systemServiceAlert = v3.ClusterAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "systemServiceRule",
			Namespace: namespace,
		},
		Spec: v32.ClusterAlertRuleSpec{
			ClusterName:       clusterName,
			GroupName:         groupID,
			CommonRuleField:   commonRuleField,
			SystemServiceRule: &systemServiceRule,
		},
		Status: alertStatus,
	}

	systemServiceRulesMap = map[string][]*v3.ClusterAlertRule{
		groupID: {
			&systemServiceAlert,
		},
	}

	systemServiceGroupBy = getClusterAlertGroupBy(systemServiceAlert.Spec)
)

//metric
var (
	metricRule = v32.MetricRule{
		Expression:     `sum(node_load5) by (instance) / count(node_cpu_seconds_total{mode="system"})`,
		Duration:       "1m",
		Comparison:     "equal",
		ThresholdValue: 1,
	}

	metricAlert = v3.ClusterAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metricRule",
			Namespace: namespace,
		},
		Spec: v32.ClusterAlertRuleSpec{
			ClusterName:     clusterName,
			GroupName:       groupID,
			CommonRuleField: commonRuleField,
			MetricRule:      &metricRule,
		},
		Status: alertStatus,
	}

	metricRulesMap = map[string][]*v3.ClusterAlertRule{
		groupID: {
			&metricAlert,
		},
	}

	metricGroupBy = getClusterAlertGroupBy(metricAlert.Spec)
)

//pod
var (
	podRule = v32.PodRule{
		PodName:   "pod1",
		Condition: "notrunning",
	}

	podAlert = v3.ProjectAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podRule",
			Namespace: projectName,
		},
		Spec: v32.ProjectAlertRuleSpec{
			ProjectName:     projectID,
			GroupName:       groupID,
			CommonRuleField: commonRuleField,
			PodRule:         &podRule,
		},
		Status: alertStatus,
	}

	podRulesMap = map[string]map[string][]*v3.ProjectAlertRule{
		projectID: {
			groupID: {
				&podAlert,
			},
		},
	}

	podGroupBy = getProjectAlertGroupBy(podAlert.Spec)
)

//workload
var (
	workloadRule = v32.WorkloadRule{
		WorkloadID:          "workloadID1",
		AvailablePercentage: 50,
	}

	workloadAlert = v3.ProjectAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workloadRule",
			Namespace: projectName,
		},
		Spec: v32.ProjectAlertRuleSpec{
			ProjectName:     projectID,
			GroupName:       groupID,
			CommonRuleField: commonRuleField,
			WorkloadRule:    &workloadRule,
		},
		Status: alertStatus,
	}

	workloadRulesMap = map[string]map[string][]*v3.ProjectAlertRule{
		projectID: {
			groupID: {
				&workloadAlert,
			},
		},
	}

	workloadGroupBy = getProjectAlertGroupBy(workloadAlert.Spec)
)

//metric
var (
	projectMetricRule = v32.MetricRule{
		Expression: `sum(rate(container_cpu_user_seconds_total{container_name!="POD",namespace=~"cattle-alerting",pod_name=~"alert-manager",
			container_name!=""}[5m])) by (container_name)`,
		Duration:       "1m",
		Comparison:     "equal",
		ThresholdValue: 1,
	}

	projectMetricAlert = v3.ProjectAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metricRule",
			Namespace: projectName,
		},
		Spec: v32.ProjectAlertRuleSpec{
			ProjectName:     projectID,
			GroupName:       groupID,
			CommonRuleField: commonRuleField,
			MetricRule:      &projectMetricRule,
		},
		Status: alertStatus,
	}

	projectMetricRulesMap = map[string]map[string][]*v3.ProjectAlertRule{
		projectID: {
			groupID: {
				&projectMetricAlert,
			},
		},
	}

	projectMetricGroupBy = getProjectAlertGroupBy(projectMetricAlert.Spec)
)
