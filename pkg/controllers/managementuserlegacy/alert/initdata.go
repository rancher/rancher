package alert

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	nodeAlertGroupName = "node-alert"

	// cluster scan related
	clusterScanAlertGroupName                    = "cluster-scan-alert"
	clusterScanManualAllAlertRuleName            = "cluster-scan-manual-all"
	clusterScanManualFailureOnlyAlertRuleName    = "cluster-scan-manual-failure-only"
	clusterScanScheduledAllAlertRuleName         = "cluster-scan-scheduled-all"
	clusterScanScheduledFailureOnlyAlertRuleName = "cluster-scan-scheduled--failure-only"
)

var (
	windowNodeLabel = labels.Set(map[string]string{"kubernetes.io/os": "windows"}).AsSelector()
	inherited       = false
)

type initClusterAlerts struct {
	clusterAlertGroups      v3.ClusterAlertGroupInterface
	clusterAlertGroupLister v3.ClusterAlertGroupLister
	clusterAlertRules       v3.ClusterAlertRuleInterface
	clusterAlertRuleLister  v3.ClusterAlertRuleLister
}

type entry struct {
	group v3.ClusterAlertGroup
	rules []v3.ClusterAlertRule
}

// cluster alerting groups and rules for those groups
var entries = []entry{
	{
		group: v3.ClusterAlertGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "etcd-alert",
			},
			Spec: v32.ClusterGroupSpec{
				CommonGroupField: v32.CommonGroupField{
					Description: "Alert for etcd leader existence, db size",
					DisplayName: "A set of alerts for etcd",
					TimingField: defaultTimingField,
				},
			},
			Status: v32.AlertStatus{
				AlertState: "active",
			},
		},
		rules: []v3.ClusterAlertRule{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-leader",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityCritical,
						DisplayName: "Etcd member has no leader",
						TimingField: defaultTimingField,
					},
					MetricRule: &v32.MetricRule{
						Description:    "Etcd member has no leader",
						Expression:     `etcd_server_has_leader`,
						Comparison:     manager.ComparisonNotEqual,
						Duration:       "3m",
						ThresholdValue: 1,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "high-number-of-leader-changes",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityWarning,
						DisplayName: "A high number of leader changes within the etcd cluster are happening",
						TimingField: defaultTimingField,
					},
					MetricRule: &v32.MetricRule{
						Description:    "Etcd instance has seen high number of leader changes within the last hour",
						Expression:     `increase(etcd_server_leader_changes_seen_total[1h])`,
						Comparison:     manager.ComparisonGreaterThan,
						Duration:       "3m",
						ThresholdValue: 3,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "db-over-size",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityWarning,
						DisplayName: "Database usage close to the quota 500M",
						TimingField: defaultTimingField,
					},
					MetricRule: &v32.MetricRule{
						Description:    "Shows the etcd database size including free space waiting for defragmentation close to the quota",
						Expression:     `sum(etcd_debugging_mvcc_db_total_size_in_bytes)`,
						Comparison:     manager.ComparisonGreaterThan,
						Duration:       "3m",
						ThresholdValue: 524288000,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "etcd-system-service",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityCritical,
						DisplayName: "Etcd is unavailable",
						Inherited:   &inherited,
						TimingField: v32.TimingField{
							GroupWaitSeconds:      600,
							GroupIntervalSeconds:  180,
							RepeatIntervalSeconds: 3600,
						},
					},
					SystemServiceRule: &v32.SystemServiceRule{
						Condition: "etcd",
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
		},
	},
	{
		group: v3.ClusterAlertGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-components-alert",
			},
			Spec: v32.ClusterGroupSpec{
				CommonGroupField: v32.CommonGroupField{
					Description: "Alert for kube components api server, scheduler, controller manager",
					DisplayName: "A set of alerts for kube components",
					TimingField: defaultTimingField,
				},
			},
			Status: v32.AlertStatus{
				AlertState: "active",
			},
		},
		rules: []v3.ClusterAlertRule{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scheduler-system-service",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityCritical,
						DisplayName: "Scheduler is unavailable",
						TimingField: defaultTimingField,
					},
					SystemServiceRule: &v32.SystemServiceRule{
						Condition: "scheduler",
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "controllermanager-system-service",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityCritical,
						DisplayName: "Controller Manager is unavailable",
						TimingField: defaultTimingField,
					},
					SystemServiceRule: &v32.SystemServiceRule{
						Condition: "controller-manager",
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
		},
	},
	{
		group: v3.ClusterAlertGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-alert",
			},
			Spec: v32.ClusterGroupSpec{
				CommonGroupField: v32.CommonGroupField{
					Description: "Alert for Node Memory, CPU, Disk Usage",
					DisplayName: "A set of alerts for node",
					TimingField: defaultTimingField,
				},
			},
			Status: v32.AlertStatus{
				AlertState: "active",
			},
		},
		rules: []v3.ClusterAlertRule{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-disk-running-full",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityCritical,
						DisplayName: "Node disk is running full within 24 hours",
						TimingField: defaultTimingField,
					},
					MetricRule: &v32.MetricRule{
						Description: "Device on node is running full within the next 24 hours",
						Expression:  `predict_linear(node_filesystem_free_bytes{mountpoint!~"^/etc/(?:resolv.conf|hosts|hostname)$"}[6h], 3600 * 24) < 0`,
						Comparison:  manager.ComparisonHasValue,
						Duration:    "10m",
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "high-memmory",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityWarning,
						DisplayName: "High node memory utilization",
						TimingField: defaultTimingField,
					},
					MetricRule: &v32.MetricRule{
						Description:    "Node memory utilization is over 80%",
						Expression:     `(1 - sum(node_memory_MemAvailable_bytes) by (instance) / sum(node_memory_MemTotal_bytes) by (instance)) * 100`,
						Comparison:     manager.ComparisonGreaterOrEqual,
						Duration:       "3m",
						ThresholdValue: 80,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "high-cpu-load",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityWarning,
						DisplayName: "High cpu load",
						TimingField: defaultTimingField,
					},
					MetricRule: &v32.MetricRule{
						Description:    "The cpu load is higher than 100",
						Expression:     `sum(node_load1) by (node)  / sum(kube_node_status_capacity{resource="cpu"}) by (node) * 100`,
						Comparison:     manager.ComparisonGreaterThan,
						Duration:       "3m",
						ThresholdValue: 100,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
		},
	},
	{
		group: v3.ClusterAlertGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "event-alert",
			},
			Spec: v32.ClusterGroupSpec{
				CommonGroupField: v32.CommonGroupField{
					Description: "Alert for receiving resource event",
					DisplayName: "A set of alerts when event happened",
					TimingField: defaultTimingField,
				},
			},
			Status: v32.AlertStatus{
				AlertState: "active",
			},
		},
		rules: []v3.ClusterAlertRule{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deployment-event-alert",
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityWarning,
						DisplayName: "Get warning deployment event",
						TimingField: defaultTimingField,
					},
					EventRule: &v32.EventRule{
						EventType:    "Warning",
						ResourceKind: "Deployment",
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
		},
	},
	{

		group: v3.ClusterAlertGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterScanAlertGroupName,
			},
			Spec: v32.ClusterGroupSpec{
				CommonGroupField: v32.CommonGroupField{
					Description: "Alert for Cluster Scans, both manual and scheduled",
					DisplayName: "A set of alerts for cluster scans",
					TimingField: defaultTimingField,
				},
			},
			Status: v32.AlertStatus{
				AlertState: "active",
			},
		},
		rules: []v3.ClusterAlertRule{
			v3.ClusterAlertRule{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterScanManualAllAlertRuleName,
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityInfo,
						DisplayName: "Manual Cluster Scan Completed",
						TimingField: defaultTimingField,
					},
					ClusterScanRule: &v32.ClusterScanRule{
						ScanRunType:  v32.ClusterScanRunTypeManual,
						FailuresOnly: false,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "inactive",
				},
			},
			v3.ClusterAlertRule{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterScanManualFailureOnlyAlertRuleName,
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityWarning,
						DisplayName: "Manual Cluster Scan has Failures",
						TimingField: defaultTimingField,
					},
					ClusterScanRule: &v32.ClusterScanRule{
						ScanRunType:  v32.ClusterScanRunTypeManual,
						FailuresOnly: true,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "inactive",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterScanScheduledAllAlertRuleName,
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityInfo,
						DisplayName: "Scheduled Cluster Scan Completed",
						TimingField: defaultTimingField,
					},
					ClusterScanRule: &v32.ClusterScanRule{
						ScanRunType:  v32.ClusterScanRunTypeScheduled,
						FailuresOnly: false,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "inactive",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterScanScheduledFailureOnlyAlertRuleName,
				},
				Spec: v32.ClusterAlertRuleSpec{
					CommonRuleField: v32.CommonRuleField{
						Severity:    SeverityWarning,
						DisplayName: "Scheduled Cluster Scan has Failures",
						TimingField: defaultTimingField,
					},
					ClusterScanRule: &v32.ClusterScanRule{
						ScanRunType:  v32.ClusterScanRunTypeScheduled,
						FailuresOnly: true,
					},
				},
				Status: v32.AlertStatus{
					AlertState: "active",
				},
			},
		},
	},
}

func initClusterPreCanAlerts(initAlerts *initClusterAlerts, clusterName string) {
	for _, entry := range entries {
		group := entry.group

		_, err := initAlerts.clusterAlertGroupLister.Get(clusterName, group.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logrus.Warnf("Failed to get precan alert group %v: %v", group.Name, err)
			} else {
				group.Spec.ClusterName = clusterName
				if _, err := initAlerts.clusterAlertGroups.Create(&group); err != nil && !apierrors.IsAlreadyExists(err) {
					logrus.Warnf("Failed to create precan alert group %v: %v", group.Name, err)
				}

			}
		}

		for _, rule := range entry.rules {
			_, err := initAlerts.clusterAlertRuleLister.Get(clusterName, rule.Name)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					logrus.Warnf("Failed to get precan alert rule %v: %v", rule.Name, err)
				} else {
					rule.Spec.ClusterName = clusterName
					rule.Spec.GroupName = common.GetGroupID(clusterName, group.Name)
					if _, err := initAlerts.clusterAlertRules.Create(&rule); err != nil && !apierrors.IsAlreadyExists(err) {
						logrus.Warnf("Failed to create precan alert rule %v: %v", rule.Name, err)
					}

				}
			}
		}

	}

}

type ProjectLifecycle struct {
	projectAlertGroups v3.ProjectAlertGroupInterface
	projectAlertRules  v3.ProjectAlertRuleInterface
	clusterName        string
}

// Create built-in project alerts
func (l *ProjectLifecycle) Create(obj *v3.Project) (runtime.Object, error) {
	name := "projectalert-workload-alert"
	projectName := obj.Name
	projectID := fmt.Sprintf("%s:%s", obj.Namespace, obj.Name)
	group := &v3.ProjectAlertGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: projectName,
		},
		Spec: v32.ProjectGroupSpec{
			ProjectName: projectID,
			CommonGroupField: v32.CommonGroupField{
				DisplayName: "A set of alerts for workload, pod, container",
				Description: "Alert for cpu, memory, disk, network",
				TimingField: defaultTimingField,
			},
		},
		Status: v32.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := l.projectAlertGroups.Create(group); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create built-in rules for deployment: %v", err)
	}

	name = "less-than-half-workload-available"
	rule := &v3.ProjectAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: projectName,
		},
		Spec: v32.ProjectAlertRuleSpec{
			ProjectName: projectID,
			GroupName:   common.GetGroupID(projectName, group.Name),
			CommonRuleField: v32.CommonRuleField{
				Severity:    SeverityCritical,
				DisplayName: "Less than half workload available",
				TimingField: defaultTimingField,
			},
			WorkloadRule: &v32.WorkloadRule{
				Selector: map[string]string{
					"app": "workload",
				},
				AvailablePercentage: 50,
			},
		},
		Status: v32.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := l.projectAlertRules.Create(rule); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create precan rules for %s: %v", name, err)
	}

	name = "memory-close-to-resource-limited"
	rule = &v3.ProjectAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: projectName,
		},
		Spec: v32.ProjectAlertRuleSpec{
			ProjectName: projectID,
			GroupName:   common.GetGroupID(projectName, group.Name),
			CommonRuleField: v32.CommonRuleField{
				Severity:    SeverityWarning,
				DisplayName: "Memory usage close to the quota",
				TimingField: defaultTimingField,
			},
			MetricRule: &v32.MetricRule{
				Description:    "Container using memory close to the quota",
				Expression:     `sum(container_memory_working_set_bytes) by (pod_name, container_name) / sum(label_join(label_join(kube_pod_container_resource_limits_memory_bytes,"pod_name", "", "pod"),"container_name", "", "container")) by (pod_name, container_name)`,
				Comparison:     manager.ComparisonGreaterThan,
				Duration:       "3m",
				ThresholdValue: 1,
			},
		},
		Status: v32.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := l.projectAlertRules.Create(rule); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Warnf("Failed to create precan rules for %s: %v", name, err)
	}
	return obj, nil
}

func (l *ProjectLifecycle) Updated(obj *v3.Project) (runtime.Object, error) {
	return obj, nil
}

func (l *ProjectLifecycle) Remove(obj *v3.Project) (runtime.Object, error) {
	return obj, nil
}

type windowsNodeSyner struct {
	clusterName       string
	clusterAlertRules v3.ClusterAlertRuleInterface
	nodeLister        v1.NodeLister
}

func (l *windowsNodeSyner) Sync(key string, obj *corev1.Node) (runtime.Object, error) {
	windowsNodes, err := l.nodeLister.List(metav1.NamespaceAll, windowNodeLabel)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list nodes")
	}

	if len(windowsNodes) != 0 {
		return obj, l.initClusterWindowsAlert()
	}

	return obj, nil
}

func (l *windowsNodeSyner) initClusterWindowsAlert() error {
	groupName := common.GetGroupID(l.clusterName, nodeAlertGroupName)
	name := "windows-node-disk-running-full"

	exsitWindowsAlert, err := l.clusterAlertRules.Controller().Lister().Get(l.clusterName, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "Failed to get alert rules %s", name)
	}

	if exsitWindowsAlert != nil {
		return nil
	}

	rule := &v3.ClusterAlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v32.ClusterAlertRuleSpec{
			ClusterName: l.clusterName,
			GroupName:   groupName,
			CommonRuleField: v32.CommonRuleField{
				Severity:    SeverityCritical,
				DisplayName: "Windows node disk is running full within 24 hours",
				TimingField: defaultTimingField,
			},
			MetricRule: &v32.MetricRule{
				Description: "Device on node is running full within the next 24 hours",
				Expression:  `predict_linear(node_filesystem_free_bytes{job=~"expose-node-metrics-windows", device!~"HarddiskVolume.+"}[6h], 3600 * 24) < 0`,
				Comparison:  manager.ComparisonHasValue,
				Duration:    "10m",
			},
		},
		Status: v32.AlertStatus{
			AlertState: "active",
		},
	}

	if _, err := l.clusterAlertRules.Create(rule); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Failed to create precan rules %s", name)
	}
	return nil
}
