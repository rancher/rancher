package configsyncer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/user/alert/common"
	alertconfig "github.com/rancher/rancher/pkg/controllers/user/alert/config"
	"github.com/rancher/rancher/pkg/controllers/user/alert/deployer"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/ref"
	"k8s.io/apimachinery/pkg/runtime"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	defaultGroupInterval  = 10
	eventGroupInterval    = 1
	defaultGroupWait      = 10
	defaultRepeatInterval = 10
)

func NewConfigSyncer(ctx context.Context, cluster *config.UserContext, alertManager *manager.AlertManager, operatorCRDManager *manager.PromOperatorCRDManager) *ConfigSyncer {
	return &ConfigSyncer{
		secretsGetter:           cluster.Core,
		clusterAlertGroupLister: cluster.Management.Management.ClusterAlertGroups(cluster.ClusterName).Controller().Lister(),
		projectAlertGroupLister: cluster.Management.Management.ProjectAlertGroups("").Controller().Lister(),
		clusterAlertRuleLister:  cluster.Management.Management.ClusterAlertRules(cluster.ClusterName).Controller().Lister(),
		projectAlertRuleLister:  cluster.Management.Management.ProjectAlertRules("").Controller().Lister(),
		notifierLister:          cluster.Management.Management.Notifiers(cluster.ClusterName).Controller().Lister(),
		clusterName:             cluster.ClusterName,
		alertManager:            alertManager,
		operatorCRDManager:      operatorCRDManager,
		namespaces:              cluster.Core.Namespaces(metav1.NamespaceAll),
	}
}

type ConfigSyncer struct {
	secretsGetter           v1.SecretsGetter
	projectAlertGroupLister v3.ProjectAlertGroupLister
	clusterAlertGroupLister v3.ClusterAlertGroupLister
	projectAlertRuleLister  v3.ProjectAlertRuleLister
	clusterAlertRuleLister  v3.ClusterAlertRuleLister
	notifierLister          v3.NotifierLister
	clusterName             string
	alertManager            *manager.AlertManager
	operatorCRDManager      *manager.PromOperatorCRDManager
	namespaces              v1.NamespaceInterface
}

func (d *ConfigSyncer) ProjectGroupSync(key string, alert *v3.ProjectAlertGroup) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *ConfigSyncer) ClusterGroupSync(key string, alert *v3.ClusterAlertGroup) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *ConfigSyncer) ProjectRuleSync(key string, alert *v3.ProjectAlertRule) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *ConfigSyncer) ClusterRuleSync(key string, alert *v3.ClusterAlertRule) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *ConfigSyncer) NotifierSync(key string, alert *v3.Notifier) (runtime.Object, error) {
	return nil, d.sync()
}

//sync: update the secret which store the configuration of alertmanager given the latest configured notifiers and alerts rules.
//For each alert, it will generate a route and a receiver in the alertmanager's configuration file, for metric rules it will update operator crd also.
func (d *ConfigSyncer) sync() error {
	if d.alertManager.IsDeploy == false {
		return nil
	}

	if _, err := d.alertManager.GetAlertManagerEndpoint(); err != nil {
		return err
	}
	notifiers, err := d.notifierLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List notifiers")
	}

	clusterAlertRules, err := d.clusterAlertRuleLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List cluster alert rules")
	}

	projectAlertRules, err := d.projectAlertRuleLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List project alert rules")
	}

	var haveClusterMetrics bool
	cAlertsAlertmanagerMap, cAlertsOperatorMap := map[string][]*v3.ClusterAlertRule{}, map[string][]*v3.ClusterAlertRule{}
	for _, alert := range clusterAlertRules {
		if alert.Status.AlertState != "inactive" {
			cAlertsAlertmanagerMap[alert.Spec.GroupName] = append(cAlertsAlertmanagerMap[alert.Spec.GroupName], alert)

			if alert.Spec.MetricRule != nil {
				haveClusterMetrics = true

				cAlertsOperatorMap[alert.Spec.GroupName] = append(cAlertsOperatorMap[alert.Spec.GroupName], alert)
			}
		}
	}

	var haveProjectMetrics bool
	pAlertsAlermanagerMap, pAlertsOperatorMap := make(map[string]map[string][]*v3.ProjectAlertRule), make(map[string]map[string][]*v3.ProjectAlertRule)
	for _, alert := range projectAlertRules {
		if controller.ObjectInCluster(d.clusterName, alert) {

			if alert.Status.AlertState != "inactive" {

				_, projectName := ref.Parse(alert.Spec.ProjectName)
				if _, ok := pAlertsAlermanagerMap[projectName]; !ok {
					pAlertsAlermanagerMap[projectName] = make(map[string][]*v3.ProjectAlertRule)
				}
				pAlertsAlermanagerMap[projectName][alert.Spec.GroupName] = append(pAlertsAlermanagerMap[projectName][alert.Spec.GroupName], alert)

				if alert.Spec.MetricRule != nil {
					haveProjectMetrics = true

					if _, ok := pAlertsOperatorMap[projectName]; !ok {
						pAlertsOperatorMap[projectName] = make(map[string][]*v3.ProjectAlertRule)
					}
					pAlertsOperatorMap[projectName][alert.Spec.GroupName] = append(pAlertsOperatorMap[projectName][alert.Spec.GroupName], alert)
				}
			}
		}
	}

	if haveClusterMetrics {
		_, prometheusNamespace := monitorutil.ClusterMonitoringInfo()
		promRule := manager.GetDefaultPrometheusRule(prometheusNamespace, d.clusterName)
		d.addClusterAlert2Operator(promRule, cAlertsOperatorMap)
		old, err := d.operatorCRDManager.PrometheusRules.GetNamespaced(prometheusNamespace, d.clusterName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				if _, err = d.operatorCRDManager.PrometheusRules.Create(promRule); err != nil && !apierrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "create prometheus rule %s failed", d.clusterName)
				}
			} else {
				return errors.Wrapf(err, "get prometheus rule %s failed", d.clusterName)
			}
		}

		updated := old.DeepCopy()
		updated.Spec = promRule.Spec
		if _, err = d.operatorCRDManager.PrometheusRules.Update(updated); err != nil {
			return errors.Wrapf(err, "update prometheus rule %s failed", d.clusterName)
		}
	}

	if haveProjectMetrics {
		for projectName, groupRules := range pAlertsOperatorMap {
			_, ns := monitorutil.ProjectMonitoringInfo(projectName)
			if err := d.createNamespace(ns); err != nil {
				return err
			}
			promRule := manager.GetDefaultPrometheusRule(ns, projectName)

			d.addProjectAlert2Operator(promRule, groupRules)

			old, err := d.operatorCRDManager.PrometheusRules.GetNamespaced(ns, projectName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					if _, err = d.operatorCRDManager.PrometheusRules.Create(promRule); err != nil && !apierrors.IsAlreadyExists(err) {
						return err
					}
				} else {
					return errors.Wrapf(err, "get prometheus rule %s failed", ns)
				}
			}

			updated := old.DeepCopy()
			updated.Spec = promRule.Spec
			if _, err = d.operatorCRDManager.PrometheusRules.Update(updated); err != nil {
				return err
			}
		}
	}

	config := manager.GetAlertManagerDefaultConfig()
	config.Global.PagerdutyURL = "https://events.pagerduty.com/generic/2010-04-15/create_event.json"

	if err = d.addClusterAlert2Config(config, cAlertsAlertmanagerMap, notifiers); err != nil {
		return err
	}

	if err = d.addProjectAlert2Config(config, pAlertsAlermanagerMap, notifiers); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrapf(err, "Marshal secrets")
	}

	altermanagerAppName, altermanagerAppNamespace := monitorutil.ClusterAlertManagerInfo()
	secretClient := d.secretsGetter.Secrets(altermanagerAppNamespace)
	secretName := common.GetAlertManagerSecretName(altermanagerAppName)
	configSecret, err := secretClient.Get(secretName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "Get secrets")
	}

	if string(configSecret.Data["alertmanager.yaml"]) != string(data) {
		newConfigSecret := configSecret.DeepCopy()
		newConfigSecret.Data["alertmanager.yaml"] = data
		newConfigSecret.Data["notification.tmpl"] = []byte(deployer.NotificationTmpl)

		_, err = secretClient.Update(newConfigSecret)
		if err != nil {
			return errors.Wrapf(err, "Update secrets")
		}

	} else {
		logrus.Debug("The config stay the same, will not update the secret")
	}

	return nil
}

func (d *ConfigSyncer) getNotifier(id string, notifiers []*v3.Notifier) *v3.Notifier {

	for _, n := range notifiers {
		if d.clusterName+":"+n.Name == id {
			return n
		}
	}

	return nil
}

func (d *ConfigSyncer) addProjectAlert2Operator(promRule *monitoringv1.PrometheusRule, groupRules map[string][]*v3.ProjectAlertRule) {
	for k, rules := range groupRules {
		ruleGroup := d.operatorCRDManager.GetRuleGroup(k)
		for _, rule := range rules {
			if rule.Spec.MetricRule != nil {
				ruleID := common.GetRuleID(rule.Spec.GroupName, rule.Name)
				d.operatorCRDManager.AddRule(ruleGroup, rule.Spec.GroupName, ruleID, rule.Spec.DisplayName, rule.Spec.Severity, rule.Spec.MetricRule)
			}
		}
		manager.AddRuleGroup(promRule, *ruleGroup)
	}
}

func (d *ConfigSyncer) addClusterAlert2Operator(promRule *monitoringv1.PrometheusRule, groupRules map[string][]*v3.ClusterAlertRule) {
	for k, rules := range groupRules {
		ruleGroup := d.operatorCRDManager.GetRuleGroup(k)
		for _, rule := range rules {
			ruleID := common.GetRuleID(rule.Spec.GroupName, rule.Name)
			d.operatorCRDManager.AddRule(ruleGroup, rule.Spec.GroupName, ruleID, rule.Spec.DisplayName, rule.Spec.Severity, rule.Spec.MetricRule)
		}
		manager.AddRuleGroup(promRule, *ruleGroup)
	}
}

func (d *ConfigSyncer) addProjectAlert2Config(config *alertconfig.Config, projectGroups map[string]map[string][]*v3.ProjectAlertRule, notifiers []*v3.Notifier) error {
	for projectName, groupedRules := range projectGroups {

		for groupID, rules := range groupedRules {

			_, groupName := ref.Parse(groupID)

			group, err := d.projectAlertGroupLister.Get(projectName, groupName)
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("get project alert group %s:%s failed, %v", projectName, groupName, err)
			}

			if group == nil {
				continue
			}

			receiver := &alertconfig.Receiver{Name: groupID}

			exist := d.addRecipients(notifiers, receiver, group.Spec.Recipients)

			if exist {
				config.Receivers = append(config.Receivers, receiver)
				r1 := d.newRoute(map[string]string{"group_id": groupID}, defaultGroupWait, defaultRepeatInterval, defaultGroupInterval)

				for _, alert := range rules {
					if alert.Status.AlertState == "inactive" {
						continue
					}

					if alert.Spec.PodRule != nil || alert.Spec.WorkloadRule != nil || alert.Spec.MetricRule != nil {
						ruleID := common.GetRuleID(groupID, alert.Name)
						d.addRule(ruleID, r1, alert.Spec.CommonRuleField)
					}

				}
				d.appendRoute(config.Route, r1)
			}
		}
	}

	return nil
}

func (d *ConfigSyncer) addClusterAlert2Config(config *alertconfig.Config, alerts map[string][]*v3.ClusterAlertRule, notifiers []*v3.Notifier) error {
	for groupID, groupRules := range alerts {
		_, groupName := ref.Parse(groupID)

		receiver := &alertconfig.Receiver{Name: groupID}

		group, err := d.clusterAlertGroupLister.Get(d.clusterName, groupName)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("get cluster alert group %s:%s failed, %v", d.clusterName, groupName, err)
		}

		if group == nil {
			continue
		}

		exist := d.addRecipients(notifiers, receiver, group.Spec.Recipients)

		if exist {
			config.Receivers = append(config.Receivers, receiver)
			r1 := d.newRoute(map[string]string{"group_id": groupID}, defaultGroupWait, defaultRepeatInterval, defaultGroupInterval)
			for _, alert := range groupRules {
				if alert.Status.AlertState == "inactive" {
					continue
				}
				ruleID := common.GetRuleID(groupID, alert.Name)

				if alert.Spec.EventRule != nil {
					r2 := d.newRoute(map[string]string{"alert_type": "event", "rule_id": ruleID}, defaultGroupWait, defaultRepeatInterval, eventGroupInterval)
					d.appendRoute(r1, r2) //todo: better not overwrite interval for each, if the interval is same as above, should not add interval field
				}

				if alert.Spec.MetricRule != nil || alert.Spec.SystemServiceRule != nil || alert.Spec.NodeRule != nil {
					d.addRule(ruleID, r1, alert.Spec.CommonRuleField)
				}

			}

			d.appendRoute(config.Route, r1)
		}
	}
	return nil
}

func (d *ConfigSyncer) addRule(ruleID string, route *alertconfig.Route, comm v3.CommonRuleField) {
	r2 := d.newRoute(map[string]string{"rule_id": ruleID}, comm.GroupWaitSeconds, comm.GroupIntervalSeconds, comm.RepeatIntervalSeconds)
	d.appendRoute(route, r2)
}

func (d *ConfigSyncer) newRoute(match map[string]string, groupWait, groupInterval, repeatInterval int) *alertconfig.Route {
	route := &alertconfig.Route{
		Receiver: match["group_id"],
		Match:    match,
	}

	gw := model.Duration(time.Duration(groupWait) * time.Second)
	route.GroupWait = &gw
	ri := model.Duration(time.Duration(repeatInterval) * time.Second)
	route.RepeatInterval = &ri

	if groupInterval != defaultGroupInterval {
		gi := model.Duration(time.Duration(groupInterval) * time.Second)
		route.GroupInterval = &gi
	}
	return route
}

func (d *ConfigSyncer) appendRoute(route *alertconfig.Route, subRoute *alertconfig.Route) {
	if route.Routes == nil {
		route.Routes = []*alertconfig.Route{}
	}
	route.Routes = append(route.Routes, subRoute)
}

func (d *ConfigSyncer) addRecipients(notifiers []*v3.Notifier, receiver *alertconfig.Receiver, recipients []v3.Recipient) bool {
	receiverExist := false
	for _, r := range recipients {
		if r.NotifierName != "" {
			notifier := d.getNotifier(r.NotifierName, notifiers)
			if notifier == nil {
				logrus.Debugf("Can not find the notifier %s", r.NotifierName)
				continue
			}

			if notifier.Spec.PagerdutyConfig != nil {
				pagerduty := &alertconfig.PagerdutyConfig{
					ServiceKey:  alertconfig.Secret(notifier.Spec.PagerdutyConfig.ServiceKey),
					Description: `{{ template "rancher.title" . }}`,
				}
				if r.Recipient != "" {
					pagerduty.ServiceKey = alertconfig.Secret(r.Recipient)
				}
				receiver.PagerdutyConfigs = append(receiver.PagerdutyConfigs, pagerduty)
				receiverExist = true

			} else if notifier.Spec.WebhookConfig != nil {
				webhook := &alertconfig.WebhookConfig{
					URL: notifier.Spec.WebhookConfig.URL,
				}
				if r.Recipient != "" {
					webhook.URL = r.Recipient
				}
				receiver.WebhookConfigs = append(receiver.WebhookConfigs, webhook)
				receiverExist = true
			} else if notifier.Spec.SlackConfig != nil {
				slack := &alertconfig.SlackConfig{
					APIURL:    alertconfig.Secret(notifier.Spec.SlackConfig.URL),
					Channel:   notifier.Spec.SlackConfig.DefaultRecipient,
					Text:      `{{ template "slack.text" . }}`,
					Title:     `{{ template "rancher.title" . }}`,
					TitleLink: "",
					Color:     `{{ if eq (index .Alerts 0).Labels.severity "critical" }}danger{{ else if eq (index .Alerts 0).Labels.severity "warning" }}warning{{ else }}good{{ end }}`,
				}
				if r.Recipient != "" {
					slack.Channel = r.Recipient
				}
				receiver.SlackConfigs = append(receiver.SlackConfigs, slack)
				receiverExist = true

			} else if notifier.Spec.SMTPConfig != nil {
				header := map[string]string{}
				header["Subject"] = `{{ template "rancher.title" . }}`
				email := &alertconfig.EmailConfig{
					Smarthost:    notifier.Spec.SMTPConfig.Host + ":" + strconv.Itoa(notifier.Spec.SMTPConfig.Port),
					AuthPassword: alertconfig.Secret(notifier.Spec.SMTPConfig.Password),
					AuthUsername: notifier.Spec.SMTPConfig.Username,
					RequireTLS:   &notifier.Spec.SMTPConfig.TLS,
					To:           notifier.Spec.SMTPConfig.DefaultRecipient,
					Headers:      header,
					From:         notifier.Spec.SMTPConfig.Sender,
					HTML:         `{{ template "email.text" . }}`,
				}
				if r.Recipient != "" {
					email.To = r.Recipient
				}
				receiver.EmailConfigs = append(receiver.EmailConfigs, email)
				receiverExist = true
			}

		}
	}

	return receiverExist

}

func (d *ConfigSyncer) createNamespace(name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if _, err := d.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Creating ns")
	}
	return nil
}

func includeProjectMetrics(projectAlerts []*v3.ProjectAlertRule) bool {
	for _, v := range projectAlerts {
		if v.Spec.MetricRule != nil {
			return true
		}
	}
	return false
}
