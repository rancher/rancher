package configsyner

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/rancher/norman/controller"
	alertconfig "github.com/rancher/rancher/pkg/controllers/user/alert/config"
	"github.com/rancher/rancher/pkg/controllers/user/alert/deploy"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func NewConfigSyncer(ctx context.Context, cluster *config.UserContext, manager *manager.Manager) *ConfigSyncer {
	return &ConfigSyncer{
		secrets:            cluster.Core.Secrets("cattle-alerting"),
		clusterAlertLister: cluster.Management.Management.ClusterAlerts(cluster.ClusterName).Controller().Lister(),
		projectAlertLister: cluster.Management.Management.ProjectAlerts("").Controller().Lister(),
		notifierLister:     cluster.Management.Management.Notifiers(cluster.ClusterName).Controller().Lister(),
		clusterName:        cluster.ClusterName,
		alertManager:       manager,
	}
}

type ConfigSyncer struct {
	secrets            v1.SecretInterface
	projectAlertLister v3.ProjectAlertLister
	clusterAlertLister v3.ClusterAlertLister
	notifierLister     v3.NotifierLister
	clusterName        string
	alertManager       *manager.Manager
}

func (d *ConfigSyncer) ProjectSync(key string, alert *v3.ProjectAlert) error {
	return d.sync()
}

func (d *ConfigSyncer) ClusterSync(key string, alert *v3.ClusterAlert) error {
	return d.sync()
}

func (d *ConfigSyncer) NotifierSync(key string, alert *v3.Notifier) error {
	return d.sync()
}

//sync: update the secret which store the configuration of alertmanager given the latest configured notifiers and alerts rules.
//For each alert, it will generate a route and a receiver in the alertmanager's configuration file.
func (d *ConfigSyncer) sync() error {

	if d.alertManager.IsDeploy == false {
		return nil
	}

	notifiers, err := d.notifierLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List notifiers")
	}

	clusterAlerts, err := d.clusterAlertLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List cluster alerts")
	}

	projectAlerts, err := d.projectAlertLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List project alerts")
	}

	pAlerts := []*v3.ProjectAlert{}
	for _, alert := range projectAlerts {
		if controller.ObjectInCluster(d.clusterName, alert) {
			pAlerts = append(pAlerts, alert)
		}
	}

	config := d.alertManager.GetDefaultConfig()
	config.Global.PagerdutyURL = "https://events.pagerduty.com/generic/2010-04-15/create_event.json"

	d.addClusterAlert2Config(config, clusterAlerts, notifiers)
	d.addProjectAlert2Config(config, pAlerts, notifiers)

	data, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrapf(err, "Marshal secrets")
	}

	//TODO: check why lister does not work
	configSecret, err := d.secrets.Get("alertmanager", metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "Get secrets")
	}

	if string(configSecret.Data["config.yml"]) != string(data) {
		configSecret.Data["config.yml"] = data
		configSecret.Data["email.tmpl"] = []byte(deploy.EmailTmlp)

		_, err = d.secrets.Update(configSecret)
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

func (d *ConfigSyncer) addProjectAlert2Config(config *alertconfig.Config, alerts []*v3.ProjectAlert, notifiers []*v3.Notifier) {
	for _, alert := range alerts {
		if alert.Status.AlertState == "inactive" {
			continue
		}

		id := alert.Namespace + "-" + alert.Name

		receiver := &alertconfig.Receiver{Name: id}
		exist := d.addRecipients(notifiers, receiver, alert.Spec.Recipients)
		if exist {
			config.Receivers = append(config.Receivers, receiver)
			d.addRoute(config, id, alert.Spec.InitialWaitSeconds, alert.Spec.RepeatIntervalSeconds)

		}
	}
}

func (d *ConfigSyncer) addClusterAlert2Config(config *alertconfig.Config, alerts []*v3.ClusterAlert, notifiers []*v3.Notifier) {
	for _, alert := range alerts {
		if alert.Status.AlertState == "inactive" {
			continue
		}

		id := alert.Namespace + "-" + alert.Name

		receiver := &alertconfig.Receiver{Name: id}
		exist := d.addRecipients(notifiers, receiver, alert.Spec.Recipients)
		if exist {
			config.Receivers = append(config.Receivers, receiver)
			d.addRoute(config, id, alert.Spec.InitialWaitSeconds, alert.Spec.RepeatIntervalSeconds)
		}

	}
}

func (d *ConfigSyncer) addRoute(config *alertconfig.Config, id string, initalWait, repeatInterval int) {
	routes := config.Route.Routes
	if routes == nil {
		routes = []*alertconfig.Route{}
	}

	match := map[string]string{}
	match["alert_id"] = id
	route := &alertconfig.Route{
		Receiver: id,
		Match:    match,
	}

	gw := model.Duration(time.Duration(initalWait) * time.Second)
	route.GroupWait = &gw
	ri := model.Duration(time.Duration(repeatInterval) * time.Second)
	route.RepeatInterval = &ri

	routes = append(routes, route)
	config.Route.Routes = routes
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
					Description: "{{ (index .Alerts 0).Labels.description}}",
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
					APIURL:  alertconfig.Secret(notifier.Spec.SlackConfig.URL),
					Channel: notifier.Spec.SlackConfig.DefaultRecipient,
					Text:    "{{ (index .Alerts 0).Labels.text}}\n",
					Title:   "{{ (index .Alerts 0).Labels.title}}\n",
					//Pretext: "Alert From Rancher",
					Color: `{{ if eq (index .Alerts 0).Labels.severity "critical" }}danger{{ else if eq (index .Alerts 0).Labels.severity "warning" }}warning{{ else }}good{{ end }}`,
				}
				if r.Recipient != "" {
					slack.Channel = r.Recipient
				}
				receiver.SlackConfigs = append(receiver.SlackConfigs, slack)
				receiverExist = true

			} else if notifier.Spec.SMTPConfig != nil {
				header := map[string]string{}
				header["Subject"] = "Alert from Rancher: {{ (index .Alerts 0).Labels.title}}"
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
