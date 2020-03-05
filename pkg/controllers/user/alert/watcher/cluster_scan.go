package watcher

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/rancher/pkg/controllers/user/alert/common"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	MsgAlertManagerNotDeployed = "alert manager is currently not deployed"
	MsgNoMatchingAlertRule     = "no matching alert rule"
)

type ClusterScanWatcher struct {
	clusterName            string
	clusterLister          v3.ClusterLister
	clusterScanLister      v3.ClusterScanLister
	clusterAlertRuleLister v3.ClusterAlertRuleLister
	alertManager           *manager.AlertManager
}

func StartClusterScanWatcher(ctx context.Context, cluster *config.UserContext, alertManager *manager.AlertManager) {

	clusterLister := cluster.Management.Management.Clusters("").Controller().Lister()
	clusterScans := cluster.Management.Management.ClusterScans(cluster.ClusterName)
	clusterScanLister := clusterScans.Controller().Lister()
	clusterAlertRuleLister := cluster.Management.Management.ClusterAlertRules(cluster.ClusterName).Controller().Lister()

	clusterScanWatcher := &ClusterScanWatcher{
		clusterName:            cluster.ClusterName,
		clusterLister:          clusterLister,
		clusterScanLister:      clusterScanLister,
		clusterAlertRuleLister: clusterAlertRuleLister,
		alertManager:           alertManager,
	}
	clusterScans.AddClusterScopedHandler(ctx, "cluster-scan-watcher", cluster.ClusterName, clusterScanWatcher.Sync)
}

func (csw *ClusterScanWatcher) Sync(_ string, cs *v3.ClusterScan) (runtime.Object, error) {
	if cs == nil {
		return nil, nil
	}
	if cs.DeletionTimestamp != nil {
		return cs, nil
	}
	// Start with Unknown, True if there is/are a matching alert rule(s), else False
	if !(v3.ClusterScanConditionAlerted.IsUnknown(cs) &&
		v3.ClusterScanConditionCompleted.IsTrue(cs)) {
		return cs, nil
	}
	var err error
	if csw.alertManager.IsDeploy == false {
		logrus.Infof("ClusterScanWatcher: Sync: alert manager not deployed")
		v3.ClusterScanConditionAlerted.False(cs)
		v3.ClusterScanConditionAlerted.Message(cs, MsgAlertManagerNotDeployed)
		return cs, nil
	}
	clusterAlertRules, err := csw.clusterAlertRuleLister.List("", labels.NewSelector())
	if err != nil {
		return cs, fmt.Errorf("ClusterScanWatcher: Sync: error listing cluster alert rules: %v", err)
	}

	match := false
	var matchingAlertRules []*v3.ClusterAlertRule
	for _, alertRule := range clusterAlertRules {
		if alertRule.Status.AlertState == "inactive" || alertRule.Spec.ClusterScanRule == nil {
			continue
		}
		if csw.isAlertRuleMatching(cs, alertRule) {
			match = true
			matchingAlertRules = append(matchingAlertRules, alertRule)
		}
	}
	logrus.Debugf("ClusterScanWatcher: Sync: matchingAlertRules: %+v", matchingAlertRules)
	if !match {
		v3.ClusterScanConditionAlerted.False(cs)
		v3.ClusterScanConditionAlerted.Message(cs, MsgNoMatchingAlertRule)
		return cs, nil
	}

	alertSuccessful := true
	for _, alertRule := range matchingAlertRules {
		if e := csw.sendAlert(cs, alertRule); e != nil {
			alertSuccessful = false
			logrus.Errorf("ClusterScanWatcher: Sync: error sending alert: %v", e)
			err = multierror.Append(err, e)
		}
	}
	if !alertSuccessful {
		return cs, err
	}
	v3.ClusterScanConditionAlerted.True(cs)
	return cs, nil
}

func (csw *ClusterScanWatcher) isAlertRuleMatching(cs *v3.ClusterScan, alertRule *v3.ClusterAlertRule) bool {
	if alertRule.Spec.ClusterScanRule.ScanRunType != cs.Spec.RunType {
		return false
	}
	if alertRule.Spec.ClusterScanRule.FailuresOnly && !(cs.Status.CisScanStatus.Fail > 0) {
		return false
	}
	return true
}

func (csw *ClusterScanWatcher) sendAlert(cs *v3.ClusterScan, alertRule *v3.ClusterAlertRule) error {
	ruleID := common.GetRuleID(alertRule.Spec.GroupName, alertRule.Name)
	clusterDisplayName := common.GetClusterDisplayName(csw.clusterName, csw.clusterLister)

	data := map[string]string{}
	data["rule_id"] = ruleID
	data["group_id"] = alertRule.Spec.GroupName
	data["alert_type"] = "clusterScan"
	data["alert_name"] = alertRule.Spec.DisplayName
	data["severity"] = alertRule.Spec.Severity
	data["cluster_name"] = clusterDisplayName
	data["component_name"] = cs.Name
	data["logs"] = csw.getAlertMessage(cs, alertRule)

	if err := csw.alertManager.SendAlert(data); err != nil {
		return err
	}
	return nil
}

func (csw *ClusterScanWatcher) getAlertMessage(cs *v3.ClusterScan, alertRule *v3.ClusterAlertRule) string {
	var msg string
	if alertRule.Spec.ClusterScanRule.FailuresOnly {
		msg = fmt.Sprintf("Cluster Scan reported %v failures", cs.Status.CisScanStatus.Fail)
	} else {
		total := cs.Status.CisScanStatus.Total
		pass := cs.Status.CisScanStatus.Pass
		fail := cs.Status.CisScanStatus.Fail
		skip := cs.Status.CisScanStatus.Skip
		na := cs.Status.CisScanStatus.NotApplicable
		msg = fmt.Sprintf("Cluster Scan Results: %v/%v pass, %v/%v fail, %v/%v skip, %v/%v not applicable",
			pass, total, fail, total, skip, total, na, total)
	}
	return msg
}
