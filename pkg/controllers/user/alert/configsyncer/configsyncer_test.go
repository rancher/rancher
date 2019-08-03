package configsyncer

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	alertconfig "github.com/rancher/rancher/pkg/controllers/user/alert/config"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	clusterAlertTests = []struct {
		caseName     string
		in           map[string][]*v3.ClusterAlertRule
		outGroupBy   []model.LabelName
		outTimeField v3.TimingField
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
		outTimeField v3.TimingField
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

func TestAddRecipients(t *testing.T) {
	var (
		addRecipientsTests = []struct {
			caseName   string
			notifiers  []*v3.Notifier
			recipients []v3.Recipient
			verify     func(*testing.T, *alertconfig.Receiver)
		}{
			{
				caseName:   "slack recipient",
				notifiers:  []*v3.Notifier{slackNotifier},
				recipients: []v3.Recipient{slackRecipient},
				verify: func(t *testing.T, receiver *alertconfig.Receiver) {
					if len(receiver.SlackConfigs) != 1 {
						t.Errorf("slackConfigs: expected 1 - actual %v", len(receiver.SlackConfigs))
					}
				},
			},
			{
				caseName:   "opsgenie recipient",
				notifiers:  []*v3.Notifier{opsgenieNotifier},
				recipients: []v3.Recipient{opsgenieRecipient},
				verify: func(t *testing.T, receiver *alertconfig.Receiver) {
					if len(receiver.OpsgenieConfigs) != 1 {
						t.Errorf("opsgenieConfigs: expected 1 - actual %v", len(receiver.OpsgenieConfigs))
					}
					opsgenieConfig := receiver.OpsgenieConfigs[0]
					if opsgenieConfig.APIURL != "https://api.opsgenie.com/" {
						t.Errorf("opsgenieConfigs: expected https://api.opsgenie.com/ - actual %v", opsgenieConfig.APIURL)
					}
				},
			},
			{
				caseName:   "opsgenie EU recipient",
				notifiers:  []*v3.Notifier{opsgenieEUNotifier},
				recipients: []v3.Recipient{opsgenieRecipient},
				verify: func(t *testing.T, receiver *alertconfig.Receiver) {
					if len(receiver.OpsgenieConfigs) != 1 {
						t.Errorf("opsgenieConfigs: expected 1 - actual %v", len(receiver.OpsgenieConfigs))
					}
					opsgenieConfig := receiver.OpsgenieConfigs[0]
					if opsgenieConfig.APIURL != "https://api.eu.opsgenie.com/" {
						t.Errorf("opsgenieConfigs: expected https://api.eu.opsgenie.com/ - actual %v", opsgenieConfig.APIURL)
					}
				},
			},
		}
	)
	for _, tt := range addRecipientsTests {
		t.Run(tt.caseName, func(t *testing.T) {
			receiver := &alertconfig.Receiver{Name: groupID}
			configSyncer := ConfigSyncer{
				clusterName: clusterName,
			}
			configSyncer.addRecipients(tt.notifiers, receiver, tt.recipients)
			tt.verify(t, receiver)
		})
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

	defaultTimingField = v3.TimingField{
		GroupWaitSeconds:      30,
		GroupIntervalSeconds:  180,
		RepeatIntervalSeconds: 3600,
	}

	commonRuleField = v3.CommonRuleField{
		DisplayName: displayName,
		Severity:    serverity,
		Inherited:   &inherited,
		TimingField: defaultTimingField,
	}

	alertStatus = v3.AlertStatus{
		AlertState: "active",
	}
)

var (
	commonGroupField = v3.CommonGroupField{
		DisplayName: displayName,
		TimingField: defaultTimingField,
	}

	clusterGroupMap = map[string]*v3.ClusterAlertGroup{
		groupID: &v3.ClusterAlertGroup{
			Spec: v3.ClusterGroupSpec{
				ClusterName:      clusterName,
				CommonGroupField: commonGroupField,
				Recipients:       recipients,
			},
			Status: alertStatus,
		},
	}

	projectGroupMap = map[string]*v3.ProjectAlertGroup{
		groupID: &v3.ProjectAlertGroup{
			Spec: v3.ProjectGroupSpec{
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
	opsgenie       = "opsgenie"
	namespace      = "cattle-alerting"
	slackNotifier  = &v3.Notifier{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slack,
			Namespace: namespace,
		},
		Spec: v3.NotifierSpec{
			ClusterName: clusterName,
			DisplayName: displayName,
			SlackConfig: &v3.SlackConfig{
				DefaultRecipient: defaultChannel,
				URL:              "www.slack.com",
			},
		},
	}
	opsgenieNotifier = &v3.Notifier{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsgenie,
			Namespace: namespace,
		},
		Spec: v3.NotifierSpec{
			ClusterName: clusterName,
			DisplayName: displayName,
			OpsgenieConfig: &v3.OpsgenieConfig{
				APIKey:           "test-api-key",
				DefaultRecipient: "test-team",
			},
		},
	}
	opsgenieEUNotifier = &v3.Notifier{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsgenie,
			Namespace: namespace,
		},
		Spec: v3.NotifierSpec{
			ClusterName: clusterName,
			DisplayName: displayName,
			OpsgenieConfig: &v3.OpsgenieConfig{
				APIKey:           "test-api-key",
				Region:           "eu",
				DefaultRecipient: "test-team",
			},
		},
	}
	notifiers = []*v3.Notifier{slackNotifier}

	slackRecipient = v3.Recipient{
		Recipient:    defaultChannel,
		NotifierName: clusterName + ":" + slack,
		NotifierType: slack,
	}
	opsgenieRecipient = v3.Recipient{
		Recipient:    defaultChannel,
		NotifierName: clusterName + ":" + opsgenie,
		NotifierType: opsgenie,
	}
	recipients = []v3.Recipient{slackRecipient}
)

// event
var (
	name = "eventRule"

	podEventRule = v3.EventRule{
		EventType:    "normal",
		ResourceKind: "Pod",
	}

	eventTimingField = v3.TimingField{
		GroupWaitSeconds:      eventGroupWait,
		GroupIntervalSeconds:  eventGroupInterval,
		RepeatIntervalSeconds: eventRepeatInterval,
	}

	eventCommonRuleField = v3.CommonRuleField{
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
		Spec: v3.ClusterAlertRuleSpec{
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
	nodeRule = v3.NodeRule{
		NodeName:  "node1",
		Condition: "notready",
	}

	nodeAlert = v3.ClusterAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nodeRule",
			Namespace: namespace,
		},
		Spec: v3.ClusterAlertRuleSpec{
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
	systemServiceRule = v3.SystemServiceRule{
		Condition: "etcd",
	}

	systemServiceAlert = v3.ClusterAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "systemServiceRule",
			Namespace: namespace,
		},
		Spec: v3.ClusterAlertRuleSpec{
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
	metricRule = v3.MetricRule{
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
		Spec: v3.ClusterAlertRuleSpec{
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
	podRule = v3.PodRule{
		PodName:   "pod1",
		Condition: "notrunning",
	}

	podAlert = v3.ProjectAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podRule",
			Namespace: projectName,
		},
		Spec: v3.ProjectAlertRuleSpec{
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
	workloadRule = v3.WorkloadRule{
		WorkloadID:          "workloadID1",
		AvailablePercentage: 50,
	}

	workloadAlert = v3.ProjectAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workloadRule",
			Namespace: projectName,
		},
		Spec: v3.ProjectAlertRuleSpec{
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
	projectMetricRule = v3.MetricRule{
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
		Spec: v3.ProjectAlertRuleSpec{
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
