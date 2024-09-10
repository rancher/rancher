package configsyncer

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator/assemblers"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	alertconfig "github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/config"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/deployer"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	notifierutil "github.com/rancher/rancher/pkg/notifiers"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	eventGroupInterval  = 1
	eventGroupWait      = 1
	eventRepeatInterval = 525600
	webhookReceiverURL  = "http://webhook-receiver.cattle-prometheus.svc:9094/"
	DingTalk            = "DINGTALK"
	MicrosoftTeams      = "MICROSOFT_TEAMS"
)

type WebhookReceiverConfig struct {
	Providers map[string]*Provider `json:"providers" yaml:"providers"`
	Receivers map[string]*Receiver `json:"receivers" yaml:"receivers"`
}

type Provider struct {
	Type       string `json:"type,omitempty" yaml:"type,omitempty"`
	WebHookURL string `json:"webhook_url,omitempty" yaml:"webhook_url,omitempty"`
	Secret     string `json:"secret,omitempty" yaml:"secret,omitempty"`
	ProxyURL   string `json:"proxy_url,omitempty" yaml:"proxy_url,omitempty"`
}

type Receiver struct {
	Provider string `yaml:"provider"`
}

func NewConfigSyncer(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, alertManager *manager.AlertManager, operatorCRDManager *manager.PromOperatorCRDManager) *ConfigSyncer {
	return &ConfigSyncer{
		apps:                    cluster.Management.Project.Apps(metav1.NamespaceAll),
		appLister:               cluster.Management.Project.Apps(metav1.NamespaceAll).Controller().Lister(),
		secretsGetter:           cluster.Core,
		secretLister:            mgmt.Core.Secrets("").Controller().Lister(),
		clusterAlertGroupLister: cluster.Management.Management.ClusterAlertGroups(cluster.ClusterName).Controller().Lister(),
		projectAlertGroupLister: cluster.Management.Management.ProjectAlertGroups("").Controller().Lister(),
		clusterAlertRuleLister:  cluster.Management.Management.ClusterAlertRules(cluster.ClusterName).Controller().Lister(),
		projectAlertRuleLister:  cluster.Management.Management.ProjectAlertRules("").Controller().Lister(),
		notifierLister:          cluster.Management.Management.Notifiers(cluster.ClusterName).Controller().Lister(),
		clusterLister:           cluster.Management.Management.Clusters(metav1.NamespaceAll).Controller().Lister(),
		projectLister:           cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		clusterName:             cluster.ClusterName,
		alertManager:            alertManager,
		operatorCRDManager:      operatorCRDManager,
	}
}

type ConfigSyncer struct {
	apps                    projectv3.AppInterface
	appLister               projectv3.AppLister
	secretsGetter           v1.SecretsGetter
	secretLister            v1.SecretLister
	projectAlertGroupLister v3.ProjectAlertGroupLister
	clusterAlertGroupLister v3.ClusterAlertGroupLister
	projectAlertRuleLister  v3.ProjectAlertRuleLister
	clusterAlertRuleLister  v3.ClusterAlertRuleLister
	notifierLister          v3.NotifierLister
	clusterLister           v3.ClusterLister
	projectLister           v3.ProjectLister
	clusterName             string
	alertManager            *manager.AlertManager
	operatorCRDManager      *manager.PromOperatorCRDManager
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

// sync: update the secret which store the configuration of alertmanager given the latest configured notifiers and alerts rules.
// For each alert, it will generate a route and a receiver in the alertmanager's configuration file, for metric rules it will update operator crd also.
func (d *ConfigSyncer) sync() error {
	project, err := project.GetSystemProject(d.clusterName, d.projectLister)
	if err != nil {
		return err
	}

	systemProjectName := project.Name
	isDeployed, webhookReceiverEnabled, err := d.isAppDeploy(systemProjectName)
	if err != nil {
		return err
	}

	if !isDeployed {
		return nil
	}

	clusterDisplayName := common.GetClusterDisplayName(d.clusterName, d.clusterLister)

	if _, err := d.alertManager.GetAlertManagerEndpoint(); err != nil {
		return err
	}
	notifiers, err := d.notifierLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List notifiers")
	}

	clusterAlertGroup, err := d.clusterAlertGroupLister.List(metav1.NamespaceAll, labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List cluster alert group")
	}

	cAlertGroupsMap := map[string]*v3.ClusterAlertGroup{}
	for _, v := range clusterAlertGroup {
		if len(v.Spec.Recipients) > 0 {
			groupID := common.GetGroupID(v.Namespace, v.Name)
			cAlertGroupsMap[groupID] = v
		}
	}

	projectAlertGroup, err := d.projectAlertGroupLister.List(metav1.NamespaceAll, labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List project alert group")
	}

	pAlertGroupsMap := map[string]*v3.ProjectAlertGroup{}
	for _, v := range projectAlertGroup {
		if len(v.Spec.Recipients) > 0 && controller.ObjectInCluster(d.clusterName, v) {
			groupID := common.GetGroupID(v.Namespace, v.Name)
			pAlertGroupsMap[groupID] = v
		}
	}

	clusterAlertRules, err := d.clusterAlertRuleLister.List(metav1.NamespaceAll, labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List cluster alert rules")
	}

	projectAlertRules, err := d.projectAlertRuleLister.List(metav1.NamespaceAll, labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List project alert rules")
	}

	cAlertsMap := map[string][]*v3.ClusterAlertRule{}
	cAlertsKey := []string{}
	for _, alert := range clusterAlertRules {
		if _, ok := cAlertGroupsMap[alert.Spec.GroupName]; ok && alert.Status.AlertState != "inactive" {
			cAlertsMap[alert.Spec.GroupName] = append(cAlertsMap[alert.Spec.GroupName], alert)
		}
	}

	for k := range cAlertsMap {
		cAlertsKey = append(cAlertsKey, k)
	}
	sort.Strings(cAlertsKey)

	pAlertsMap := map[string]map[string][]*v3.ProjectAlertRule{}
	pAlertsKey := []string{}
	for _, alert := range projectAlertRules {
		if controller.ObjectInCluster(d.clusterName, alert) {
			if _, ok := pAlertGroupsMap[alert.Spec.GroupName]; ok && alert.Status.AlertState != "inactive" {
				_, projectName := ref.Parse(alert.Spec.ProjectName)
				if _, ok := pAlertsMap[projectName]; !ok {
					pAlertsMap[projectName] = make(map[string][]*v3.ProjectAlertRule)
				}
				pAlertsMap[projectName][alert.Spec.GroupName] = append(pAlertsMap[projectName][alert.Spec.GroupName], alert)
			}
		}
	}
	for k := range pAlertsMap {
		pAlertsKey = append(pAlertsKey, k)
	}
	sort.Strings(pAlertsKey)

	if err := d.addClusterAlert2Operator(clusterDisplayName, cAlertsMap, cAlertsKey); err != nil {
		return err
	}

	if err := d.addProjectAlert2Operator(clusterDisplayName, pAlertsMap, pAlertsKey); err != nil {
		return err
	}

	config := manager.GetAlertManagerDefaultConfig()
	config.Global.PagerdutyURL = "https://events.pagerduty.com/v2/enqueue"

	if err = d.addClusterAlert2Config(config, cAlertsMap, cAlertsKey, cAlertGroupsMap, notifiers); err != nil {
		return err
	}

	if err = d.addProjectAlert2Config(config, pAlertsMap, pAlertsKey, pAlertGroupsMap, notifiers); err != nil {
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

	if string(configSecret.Data["alertmanager.yaml"]) != string(data) || string(configSecret.Data["notification.tmpl"]) != deployer.NotificationTmpl {
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

	if webhookReceiverEnabled {
		if err := d.syncWebhookConfig(notifiers, cAlertGroupsMap, pAlertGroupsMap); err != nil {
			return errors.Wrapf(err, "Update Webhook Receiver Config")
		}
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

func (d *ConfigSyncer) addProjectAlert2Operator(clusterDisplayName string, projectGroups map[string]map[string][]*v3.ProjectAlertRule, keys []string) error {
	for _, projectName := range keys {
		groupRules := projectGroups[projectName]
		_, namespace := monitorutil.ProjectMonitoringInfo(projectName)
		promRule := d.operatorCRDManager.GetDefaultPrometheusRule(namespace, projectName)

		projectID := fmt.Sprintf("%s:%s", d.clusterName, projectName)
		projectDisplayName := common.GetProjectDisplayName(projectID, d.projectLister)

		var groupIDs []string
		for k := range groupRules {
			groupIDs = append(groupIDs, k)
		}
		sort.Strings(groupIDs)

		for _, groupID := range groupIDs {
			alertRules := groupRules[groupID]
			ruleGroup := d.operatorCRDManager.GetRuleGroup(groupID)
			for _, alertRule := range alertRules {
				if alertRule.Spec.MetricRule != nil {
					ruleID := common.GetRuleID(alertRule.Spec.GroupName, alertRule.Name)
					promRule := manager.Metric2Rule(groupID, ruleID, alertRule.Spec.Severity, alertRule.Spec.DisplayName, clusterDisplayName, projectDisplayName, alertRule.Spec.MetricRule)
					d.operatorCRDManager.AddRule(ruleGroup, promRule)
				}
			}

			if len(ruleGroup.Rules) > 0 {
				d.operatorCRDManager.AddRuleGroup(promRule, *ruleGroup)
			}
		}

		if len(promRule.Spec.Groups) > 0 {
			if err := d.operatorCRDManager.SyncPrometheusRule(promRule); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *ConfigSyncer) addClusterAlert2Operator(clusterDisplayName string, groupRules map[string][]*v3.ClusterAlertRule, keys []string) error {
	_, namespace := monitorutil.ClusterMonitoringInfo()
	promRule := d.operatorCRDManager.GetDefaultPrometheusRule(namespace, d.clusterName)

	for _, groupID := range keys {
		ruleGroup := d.operatorCRDManager.GetRuleGroup(groupID)
		alertRules := groupRules[groupID]
		for _, alertRule := range alertRules {
			if alertRule.Spec.MetricRule != nil {
				ruleID := common.GetRuleID(alertRule.Spec.GroupName, alertRule.Name)
				promRule := manager.Metric2Rule(groupID, ruleID, alertRule.Spec.Severity, alertRule.Spec.DisplayName, clusterDisplayName, "", alertRule.Spec.MetricRule)
				d.operatorCRDManager.AddRule(ruleGroup, promRule)
			}
		}
		if len(ruleGroup.Rules) > 0 {
			d.operatorCRDManager.AddRuleGroup(promRule, *ruleGroup)
		}
	}

	if len(promRule.Spec.Groups) > 0 {
		return d.operatorCRDManager.SyncPrometheusRule(promRule)
	}

	return nil
}

func (d *ConfigSyncer) addProjectAlert2Config(config *alertconfig.Config, projectGroups map[string]map[string][]*v3.ProjectAlertRule, keys []string, alertGroups map[string]*v3.ProjectAlertGroup, notifiers []*v3.Notifier) error {
	for _, projectName := range keys {
		groups := projectGroups[projectName]
		var groupIDs []string
		for groupID := range groups {
			groupIDs = append(groupIDs, groupID)
		}
		sort.Strings(groupIDs)

		for _, groupID := range groupIDs {
			rules := groups[groupID]
			group, ok := alertGroups[groupID]
			if !ok {
				return fmt.Errorf("get project alert group %s failed", groupID)
			}

			receiver := &alertconfig.Receiver{Name: groupID}

			exist := d.addRecipients(notifiers, receiver, group.Spec.Recipients)

			if exist {
				config.Receivers = append(config.Receivers, receiver)
				r1 := d.newRoute(map[string]string{"group_id": groupID}, false, group.Spec.TimingField, []model.LabelName{"group_id"})

				for _, alert := range rules {
					if alert.Status.AlertState == "inactive" {
						continue
					}

					groupBy := getProjectAlertGroupBy(alert.Spec)

					if alert.Spec.PodRule != nil || alert.Spec.WorkloadRule != nil || alert.Spec.MetricRule != nil {
						ruleID := common.GetRuleID(groupID, alert.Name)
						d.addRule(ruleID, r1, alert.Spec.CommonRuleField, groupBy)
					}

				}
				d.appendRoute(config.Route, r1)
			}
		}
	}

	return nil
}

func (d *ConfigSyncer) addClusterAlert2Config(config *alertconfig.Config, alerts map[string][]*v3.ClusterAlertRule, keys []string, alertGroups map[string]*v3.ClusterAlertGroup, notifiers []*v3.Notifier) error {
	for _, groupID := range keys {
		groupRules := alerts[groupID]
		receiver := &alertconfig.Receiver{Name: groupID}

		group, ok := alertGroups[groupID]
		if !ok {
			return fmt.Errorf("get cluster alert group %s failed", groupID)
		}

		exist := d.addRecipients(notifiers, receiver, group.Spec.Recipients)

		if exist {
			config.Receivers = append(config.Receivers, receiver)
			r1 := d.newRoute(map[string]string{"group_id": groupID}, false, group.Spec.TimingField, []model.LabelName{"group_id"})
			for _, alert := range groupRules {
				if alert.Status.AlertState == "inactive" {
					continue
				}
				ruleID := common.GetRuleID(groupID, alert.Name)
				groupBy := getClusterAlertGroupBy(alert.Spec)

				if alert.Spec.EventRule != nil {
					timeFields := v32.TimingField{
						GroupWaitSeconds:      eventGroupWait,
						GroupIntervalSeconds:  eventGroupInterval,
						RepeatIntervalSeconds: eventRepeatInterval,
					}

					r2 := d.newRoute(map[string]string{"rule_id": ruleID}, false, timeFields, groupBy)
					d.appendRoute(r1, r2)
				}

				if alert.Spec.MetricRule != nil || alert.Spec.SystemServiceRule != nil || alert.Spec.NodeRule != nil {
					d.addRule(ruleID, r1, alert.Spec.CommonRuleField, groupBy)
				}

			}

			d.appendRoute(config.Route, r1)
		}
	}
	return nil
}

func (d *ConfigSyncer) addRule(ruleID string, route *alertconfig.Route, comm v32.CommonRuleField, groupBy []model.LabelName) {
	inherited := true
	if comm.Inherited != nil {
		inherited = *comm.Inherited
	}

	r2 := d.newRoute(map[string]string{"rule_id": ruleID}, inherited, comm.TimingField, groupBy)
	d.appendRoute(route, r2)
}

func (d *ConfigSyncer) newRoute(match map[string]string, inherited bool, timeFields v32.TimingField, groupBy []model.LabelName) *alertconfig.Route {
	route := &alertconfig.Route{
		Continue: true,
		Receiver: match["group_id"],
		Match:    match,
		GroupBy:  groupBy,
	}

	if !inherited {
		if timeFields.GroupWaitSeconds > 0 {
			gw := model.Duration(time.Duration(timeFields.GroupWaitSeconds) * time.Second)
			route.GroupWait = &gw
		}

		if timeFields.RepeatIntervalSeconds > 0 {
			ri := model.Duration(time.Duration(timeFields.RepeatIntervalSeconds) * time.Second)
			route.RepeatInterval = &ri
		}

		if timeFields.GroupIntervalSeconds > 0 {
			gi := model.Duration(time.Duration(timeFields.GroupIntervalSeconds) * time.Second)
			route.GroupInterval = &gi
		}
	}

	return route
}

func (d *ConfigSyncer) appendRoute(route *alertconfig.Route, subRoute *alertconfig.Route) {
	if route.Routes == nil {
		route.Routes = []*alertconfig.Route{}
	}
	route.Routes = append(route.Routes, subRoute)
}

func (d *ConfigSyncer) addRecipients(notifiers []*v3.Notifier, receiver *alertconfig.Receiver, recipients []v32.Recipient) bool {
	receiverExist := false
	for _, r := range recipients {
		if r.NotifierName != "" {
			notifier := d.getNotifier(r.NotifierName, notifiers)
			if notifier == nil {
				logrus.Debugf("Can not find the notifier %s", r.NotifierName)
				continue
			}
			commonNotifierConfig := alertconfig.NotifierConfig{
				VSendResolved: notifier.Spec.SendResolved,
			}
			if notifier.Spec.PagerdutyConfig != nil {
				pagerduty := &alertconfig.PagerdutyConfig{
					NotifierConfig: commonNotifierConfig,
					ServiceKey:     alertconfig.Secret(notifier.Spec.PagerdutyConfig.ServiceKey),
					Description:    `{{ template "rancher.title" . }}`,
				}

				if notifierutil.IsHTTPClientConfigSet(notifier.Spec.PagerdutyConfig.HTTPClientConfig) {
					url, err := toAlertManagerURL(notifier.Spec.PagerdutyConfig.HTTPClientConfig.ProxyURL)
					if err != nil {
						logrus.Errorf("Failed to parse pagerduty proxy url %s, %v", notifier.Spec.PagerdutyConfig.HTTPClientConfig.ProxyURL, err)
						continue
					}
					pagerduty.HTTPConfig = &alertconfig.HTTPClientConfig{
						ProxyURL: *url,
					}
				}
				if r.Recipient != "" {
					pagerduty.ServiceKey = alertconfig.Secret(r.Recipient)
				}
				receiver.PagerdutyConfigs = append(receiver.PagerdutyConfigs, pagerduty)
				receiverExist = true

			} else if notifier.Spec.WechatConfig != nil {
				notifierSpec, err := assemblers.AssembleWechatCredential(notifier, d.secretLister)
				if err != nil {
					logrus.Errorf("error getting Wechat credential: %v", err)
					continue
				}
				wechat := &alertconfig.WechatConfig{
					NotifierConfig: commonNotifierConfig,
					APISecret:      alertconfig.Secret(notifierSpec.WechatConfig.Secret),
					AgentID:        notifierSpec.WechatConfig.Agent,
					CorpID:         notifierSpec.WechatConfig.Corp,
					Message:        `{{ template "wechat.text" . }}`,
				}

				recipient := notifier.Spec.WechatConfig.DefaultRecipient
				if r.Recipient != "" {
					recipient = r.Recipient
				}

				switch notifier.Spec.WechatConfig.RecipientType {
				case "tag":
					wechat.ToTag = recipient
				case "user":
					wechat.ToUser = recipient
				default:
					wechat.ToParty = recipient
				}

				if notifierutil.IsHTTPClientConfigSet(notifier.Spec.WechatConfig.HTTPClientConfig) {
					url, err := toAlertManagerURL(notifier.Spec.WechatConfig.HTTPClientConfig.ProxyURL)
					if err != nil {
						logrus.Errorf("Failed to parse wechat proxy url %s, %v", notifier.Spec.WechatConfig.HTTPClientConfig.ProxyURL, err)
						continue
					}
					wechat.HTTPConfig = &alertconfig.HTTPClientConfig{
						ProxyURL: *url,
					}
				}

				receiver.WechatConfigs = append(receiver.WechatConfigs, wechat)
				receiverExist = true

			} else if notifier.Spec.DingtalkConfig != nil {
				webhookURL := webhookReceiverURL + r.NotifierName
				dingtalk := &alertconfig.WebhookConfig{
					NotifierConfig: commonNotifierConfig,
					URL:            webhookURL,
				}

				receiver.WebhookConfigs = append(receiver.WebhookConfigs, dingtalk)
				receiverExist = true

			} else if notifier.Spec.MSTeamsConfig != nil {
				webhookURL := webhookReceiverURL + r.NotifierName
				msTeams := &alertconfig.WebhookConfig{
					NotifierConfig: commonNotifierConfig,
					URL:            webhookURL,
				}

				receiver.WebhookConfigs = append(receiver.WebhookConfigs, msTeams)
				receiverExist = true

			} else if notifier.Spec.WebhookConfig != nil {
				webhook := &alertconfig.WebhookConfig{
					NotifierConfig: commonNotifierConfig,
					URL:            notifier.Spec.WebhookConfig.URL,
				}
				if r.Recipient != "" {
					webhook.URL = r.Recipient
				}

				if notifierutil.IsHTTPClientConfigSet(notifier.Spec.WebhookConfig.HTTPClientConfig) {
					url, err := toAlertManagerURL(notifier.Spec.WebhookConfig.HTTPClientConfig.ProxyURL)
					if err != nil {
						logrus.Errorf("Failed to parse webhook proxy url %s, %v", notifier.Spec.WebhookConfig.HTTPClientConfig.ProxyURL, err)
						continue
					}
					webhook.HTTPConfig = &alertconfig.HTTPClientConfig{
						ProxyURL: *url,
					}
				}

				receiver.WebhookConfigs = append(receiver.WebhookConfigs, webhook)
				receiverExist = true
			} else if notifier.Spec.SlackConfig != nil {
				slack := &alertconfig.SlackConfig{
					NotifierConfig: commonNotifierConfig,
					APIURL:         alertconfig.Secret(notifier.Spec.SlackConfig.URL),
					Channel:        notifier.Spec.SlackConfig.DefaultRecipient,
					Text:           `{{ template "slack.text" . }}`,
					Title:          `{{ template "rancher.title" . }}`,
					TitleLink:      "",
					Color:          `{{ if eq (index .Alerts 0).Labels.severity "critical" }}danger{{ else if eq (index .Alerts 0).Labels.severity "warning" }}warning{{ else }}good{{ end }}`,
				}
				if r.Recipient != "" {
					slack.Channel = r.Recipient
				}

				if notifierutil.IsHTTPClientConfigSet(notifier.Spec.SlackConfig.HTTPClientConfig) {
					url, err := toAlertManagerURL(notifier.Spec.SlackConfig.HTTPClientConfig.ProxyURL)
					if err != nil {
						logrus.Errorf("Failed to parse slack proxy url %s, %v", notifier.Spec.SlackConfig.HTTPClientConfig.ProxyURL, err)
						continue
					}
					slack.HTTPConfig = &alertconfig.HTTPClientConfig{
						ProxyURL: *url,
					}
				}
				receiver.SlackConfigs = append(receiver.SlackConfigs, slack)
				receiverExist = true

			} else if notifier.Spec.SMTPConfig != nil {
				header := map[string]string{}
				header["Subject"] = `{{ template "rancher.title" . }}`
				notifierSpec, err := assemblers.AssembleSMTPCredential(notifier, d.secretLister)
				if err != nil {
					logrus.Errorf("error getting SMTP credential: %v", err)
					continue
				}
				email := &alertconfig.EmailConfig{
					NotifierConfig: commonNotifierConfig,
					Smarthost:      notifierSpec.SMTPConfig.Host + ":" + strconv.Itoa(notifierSpec.SMTPConfig.Port),
					AuthPassword:   alertconfig.Secret(notifierSpec.SMTPConfig.Password),
					AuthUsername:   notifierSpec.SMTPConfig.Username,
					RequireTLS:     notifierSpec.SMTPConfig.TLS,
					To:             notifierSpec.SMTPConfig.DefaultRecipient,
					Headers:        header,
					From:           notifierSpec.SMTPConfig.Sender,
					HTML:           `{{ template "email.text" . }}`,
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

func (d *ConfigSyncer) isAppDeploy(appNamespace string) (bool, bool, error) {
	appName, _ := monitorutil.ClusterAlertManagerInfo()
	app, err := d.appLister.Get(appNamespace, appName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}

		return false, false, errors.Wrapf(err, "get app %s failed", appName)
	}

	if app.DeletionTimestamp != nil {
		return false, false, nil
	}

	if webhookReceiverEnabled, ok := app.Spec.Answers[deployer.WebhookReceiverEnable]; ok && webhookReceiverEnabled == "true" {
		return true, true, nil
	}

	return true, false, nil
}

func includeProjectMetrics(projectAlerts []*v3.ProjectAlertRule) bool {
	for _, v := range projectAlerts {
		if v.Spec.MetricRule != nil {
			return true
		}
	}
	return false
}

func getClusterAlertGroupBy(spec v32.ClusterAlertRuleSpec) []model.LabelName {
	if spec.EventRule != nil {
		return []model.LabelName{"rule_id", "resource_kind", "target_namespace", "target_name", "event_message"}
	} else if spec.SystemServiceRule != nil {
		return []model.LabelName{"rule_id", "component_name"}
	} else if spec.NodeRule != nil {
		return []model.LabelName{"rule_id", "node_name", "alert_type"}
	} else if spec.MetricRule != nil {
		return []model.LabelName{"rule_id"}
	}

	return nil
}

func getProjectAlertGroupBy(spec v32.ProjectAlertRuleSpec) []model.LabelName {
	if spec.PodRule != nil {
		return []model.LabelName{"rule_id", "namespace", "pod_name", "alert_type"}
	} else if spec.WorkloadRule != nil {
		return []model.LabelName{"rule_id", "workload_namespace", "workload_name", "workload_kind"}
	} else if spec.MetricRule != nil {
		return []model.LabelName{"rule_id"}
	}

	return nil
}

func toAlertManagerURL(urlStr string) (*alertconfig.URL, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return &alertconfig.URL{URL: url}, nil
}

func (d *ConfigSyncer) syncWebhookConfig(notifiers []*v3.Notifier, cAlertGroupsMap map[string]*v3.ClusterAlertGroup, pAlertGroupsMap map[string]*v3.ProjectAlertGroup) error {
	var recipients []v32.Recipient
	for _, group := range cAlertGroupsMap {
		recipients = append(recipients, group.Spec.Recipients...)
	}

	for _, group := range pAlertGroupsMap {
		recipients = append(recipients, group.Spec.Recipients...)
	}

	webhookSecreteName, altermanagerAppNamespace := monitorutil.SecretWebhook()
	secretClient := d.secretsGetter.Secrets(altermanagerAppNamespace)
	configSecret, err := secretClient.Get(webhookSecreteName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "Get secret")
	}

	oldConfig := configSecret.Data["config.yaml"]

	providers := make(map[string]*Provider)
	receivers := make(map[string]*Receiver)
	for _, r := range recipients {
		if r.NotifierName != "" {
			notifier := d.getNotifier(r.NotifierName, notifiers)
			if notifier == nil {
				logrus.Debugf("Can not find the notifier %s", r.NotifierName)
				continue
			}
			if notifier.Spec.DingtalkConfig != nil {
				provider := &Provider{
					Type:       DingTalk,
					WebHookURL: notifier.Spec.DingtalkConfig.URL,
				}
				notifierSpec, err := assemblers.AssembleDingtalkCredential(notifier, d.secretLister)
				if err != nil {
					return err
				}
				if notifierSpec.DingtalkConfig.Secret != "" {
					provider.Secret = notifierSpec.DingtalkConfig.Secret
				}
				if notifierutil.IsHTTPClientConfigSet(notifier.Spec.DingtalkConfig.HTTPClientConfig) {
					provider.ProxyURL = notifier.Spec.DingtalkConfig.HTTPClientConfig.ProxyURL
				}
				receiver := &Receiver{
					Provider: r.NotifierName,
				}
				providers[r.NotifierName] = provider
				receivers[r.NotifierName] = receiver
			} else if notifier.Spec.MSTeamsConfig != nil {
				provider := &Provider{
					Type:       MicrosoftTeams,
					WebHookURL: notifier.Spec.MSTeamsConfig.URL,
				}
				if notifierutil.IsHTTPClientConfigSet(notifier.Spec.MSTeamsConfig.HTTPClientConfig) {
					provider.ProxyURL = notifier.Spec.MSTeamsConfig.HTTPClientConfig.ProxyURL
				}
				receiver := &Receiver{
					Provider: r.NotifierName,
				}
				providers[r.NotifierName] = provider
				receivers[r.NotifierName] = receiver
			}
		}
	}

	config := WebhookReceiverConfig{
		Providers: providers,
		Receivers: receivers,
	}

	newConfig, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrapf(err, "Marshal secrets")
	}
	if !bytes.Equal(oldConfig, newConfig) {
		configSecret.Data["config.yaml"] = newConfig
		if _, err = secretClient.Update(configSecret); err != nil {
			return errors.Wrapf(err, "Update secret")
		}
	}

	return nil
}
