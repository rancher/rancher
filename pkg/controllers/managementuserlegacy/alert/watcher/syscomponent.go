package watcher

import (
	"context"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/ticker"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SysComponentWatcher struct {
	componentStatuses      v1.ComponentStatusInterface
	clusterAlertRuleLister v3.ClusterAlertRuleLister
	alertManager           *manager.AlertManager
	clusterName            string
	clusterLister          v3.ClusterLister
}

func StartSysComponentWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.AlertManager) {

	s := &SysComponentWatcher{
		componentStatuses:      cluster.Core.ComponentStatuses(""),
		clusterAlertRuleLister: cluster.Management.Management.ClusterAlertRules(cluster.ClusterName).Controller().Lister(),
		alertManager:           manager,
		clusterName:            cluster.ClusterName,
		clusterLister:          cluster.Management.Management.Clusters("").Controller().Lister(),
	}
	go s.watch(ctx, syncInterval)
}

func (w *SysComponentWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch system component, error: %v", err)
		}
	}
}

func (w *SysComponentWatcher) watchRule() error {
	if w.alertManager.IsDeploy == false {
		return nil
	}

	clusterAlerts, err := w.clusterAlertRuleLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	statuses, err := w.componentStatuses.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, rule := range clusterAlerts {
		if rule.Status.AlertState == "inactive" || rule.Spec.SystemServiceRule == nil {
			continue
		}
		if rule.Spec.SystemServiceRule != nil {
			w.checkComponentHealthy(statuses, rule)
		}
	}
	return nil
}

func (w *SysComponentWatcher) checkComponentHealthy(statuses *corev1.ComponentStatusList, alert *v3.ClusterAlertRule) {
	for _, cs := range statuses.Items {
		if strings.HasPrefix(cs.Name, alert.Spec.SystemServiceRule.Condition) {
			for _, cond := range cs.Conditions {
				if cond.Type == corev1.ComponentHealthy {
					if cond.Status == corev1.ConditionFalse {
						ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)

						clusterDisplayName := common.GetClusterDisplayName(w.clusterName, w.clusterLister)

						data := map[string]string{}
						data["rule_id"] = ruleID
						data["group_id"] = alert.Spec.GroupName
						data["alert_type"] = "systemService"
						data["alert_name"] = alert.Spec.DisplayName
						data["severity"] = alert.Spec.Severity
						data["cluster_name"] = clusterDisplayName
						data["component_name"] = alert.Spec.SystemServiceRule.Condition

						if cond.Message != "" {
							data["logs"] = cond.Message
						}
						if err := w.alertManager.SendAlert(data); err != nil {
							logrus.Errorf("Failed to send alert: %v", err)
						}
						return
					}
				}
			}
		}
	}

}
