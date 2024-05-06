package statesyncer

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/ticker"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func StartStateSyncer(ctx context.Context, cluster *config.UserContext, manager *manager.AlertManager) {
	s := &StateSyncer{
		clusterAlertRules: cluster.Management.Management.ClusterAlertRules(cluster.ClusterName),
		projectAlertRules: cluster.Management.Management.ProjectAlertRules(""),
		alertManager:      manager,
		clusterName:       cluster.ClusterName,
	}
	go s.watch(ctx, 10*time.Second)
}

func (s *StateSyncer) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		s.syncState()
	}
}

type StateSyncer struct {
	clusterAlertRules v3.ClusterAlertRuleInterface
	projectAlertRules v3.ProjectAlertRuleInterface
	alertManager      *manager.AlertManager
	clusterName       string
}

// synchronize the state between alert CRD and alertmanager.
func (s *StateSyncer) syncState() error {

	if s.alertManager.IsDeploy == false {
		return nil
	}

	apiAlerts, err := s.alertManager.GetAlertList()
	if err == nil {
		clusterAlerts, err := s.clusterAlertRules.Controller().Lister().List("", labels.NewSelector())
		if err != nil {
			return err
		}

		projectAlerts, err := s.projectAlertRules.Controller().Lister().List("", labels.NewSelector())
		if err != nil {
			return err
		}

		pAlerts := []*v3.ProjectAlertRule{}
		for _, alert := range projectAlerts {
			if controller.ObjectInCluster(s.clusterName, alert) {
				pAlerts = append(pAlerts, alert)
			}
		}

		for _, alert := range clusterAlerts {
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)
			state := s.alertManager.GetState("rule_id", ruleID, apiAlerts)
			ruleNeedUpdate := s.doSync("rule_id", ruleID, alert.Status.AlertState, state)
			if ruleNeedUpdate {
				old, err := s.clusterAlertRules.Get(alert.Name, metav1.GetOptions{})
				if err != nil {
					logrus.Errorf("Error occurred while get alert %s:%s, %v", alert.Namespace, alert.Name, err)
					continue
				}
				new := old.DeepCopy()
				new.Status.AlertState = state
				_, err = s.clusterAlertRules.Update(new)
				if err != nil {
					logrus.Errorf("Error occurred while updating %s:%s, alert state, %v", alert.Namespace, alert.Name, err)
				}
			}
		}

		for _, alert := range pAlerts {
			ruleID := common.GetRuleID(alert.Spec.GroupName, alert.Name)
			state := s.alertManager.GetState("rule_id", ruleID, apiAlerts)
			ruleNeedUpdate := s.doSync("rule_id", ruleID, alert.Status.AlertState, state)
			if ruleNeedUpdate {
				old, err := s.projectAlertRules.GetNamespaced(alert.Namespace, alert.Name, metav1.GetOptions{})
				if err != nil {
					logrus.Errorf("Error occurred while get alert %s:%s, %v", alert.Namespace, alert.Name, err)
					continue
				}
				new := old.DeepCopy()
				new.Status.AlertState = state
				_, err = s.projectAlertRules.Update(new)
				if err != nil {
					logrus.Errorf("Error occurred while updating %s:%s alert state, %v", alert.Namespace, alert.Name, err)
				}
			}
		}
	}

	return err

}

// The curState is the state in the CRD status,
// The newState is the state in alert manager side
func (s *StateSyncer) doSync(matcherName, matcherValue, curState, newState string) (needUpdate bool) {
	if curState == "inactive" {
		return false
	}

	//only take ation when the state is not the same
	if newState != curState {

		//the alert is muted by user (curState == muted), but it already went away in alertmanager side (newState == active)
		//then we need to remove the silence rule and update the state in CRD
		if curState == "muted" && newState == "active" {
			err := s.alertManager.RemoveSilenceRule(matcherName, matcherValue)
			if err != nil {
				logrus.Errorf("Error occurred while remove silence : %v", err)
			}
			return true
		}
		//the alert is unmuted by user, but it is still muted in alertmanager side
		//need to remove the silence rule, but do not have to update the CRD
		if curState == "alerting" && newState == "muted" {
			err := s.alertManager.RemoveSilenceRule(matcherName, matcherValue)
			if err != nil {
				logrus.Errorf("Error occurred while remove silence : %v", err)
			}
			return false
		}

		//the alert is muted by user, but it is still alerting in alertmanager side
		//need to add silence rule to alertmanager
		if curState == "muted" && newState == "alerting" {
			err := s.alertManager.AddSilenceRule(matcherName, matcherValue)
			if err != nil {
				logrus.Errorf("Error occurred while remove silence : %v", err)
			}
			return false
		}
		return true
	}

	return false

}
