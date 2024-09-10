package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	"github.com/prometheus/common/model"
	alertconfig "github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/config"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/sirupsen/logrus"
)

type State string

const (
	AlertStateUnprocessed State = "unprocessed"
	AlertStateActive            = "active"
	AlertStateSuppressed        = "suppressed"
)

type AlertStatus struct {
	State       State    `json:"state"`
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
}

type APIAlert struct {
	*model.Alert
	Status      AlertStatus `json:"status"`
	Receivers   []string    `json:"receivers"`
	Fingerprint string      `json:"fingerprint"`
}

type Matchers []*Matcher

type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`

	regex *regexp.Regexp
}

type Silence struct {
	ID string `json:"id"`

	Matchers Matchers `json:"matchers"`

	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`

	UpdatedAt time.Time `json:"updatedAt"`

	CreatedBy string `json:"createdBy"`
	Comment   string `json:"comment,omitempty"`

	now func() time.Time

	Status SilenceStatus `json:"status"`
}

type SilenceStatus struct {
	State SilenceState `json:"state"`
}

type SilenceState string

const (
	SilenceStateExpired SilenceState = "expired"
	SilenceStateActive  SilenceState = "active"
	SilenceStatePending SilenceState = "pending"
)

type AlertManager struct {
	svcLister   v1.ServiceLister
	dialer      dialer.Factory
	clusterName string
	client      *http.Client
	IsDeploy    bool
}

func NewAlertManager(cluster *config.UserContext) *AlertManager {
	dial, err := cluster.Management.Dialer.ClusterDialer(cluster.ClusterName, true)
	if err != nil {
		logrus.Warnf("Failed to get cluster dialer: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dial,
		},
		Timeout: 15 * time.Second,
	}

	return &AlertManager{
		svcLister:   cluster.Core.Services("").Controller().Lister(),
		client:      client,
		clusterName: cluster.ClusterName,
	}
}

func (m *AlertManager) GetAlertManagerEndpoint() (string, error) {
	name, namespace, port := monitorutil.ClusterAlertManagerEndpoint()

	svc, err := m.svcLister.Get(namespace, name)
	if err != nil {
		return "", fmt.Errorf("Failed to get service for alertmanager, %v", err)
	}

	url := "http://" + svc.Name + "." + svc.Namespace + ".svc:" + port
	return url, nil
}

func GetAlertManagerDefaultConfig() *alertconfig.Config {
	config := alertconfig.Config{}

	resolveTimeout, _ := model.ParseDuration("5m")
	config.Global = &alertconfig.GlobalConfig{
		SlackAPIURL:    "https://api.slack.com",
		ResolveTimeout: resolveTimeout,
		SMTPRequireTLS: false,
	}

	slackConfigs := []*alertconfig.SlackConfig{}
	initSlackConfig := &alertconfig.SlackConfig{
		Channel: "#alert",
	}
	slackConfigs = append(slackConfigs, initSlackConfig)

	receivers := []*alertconfig.Receiver{}
	initReceiver := &alertconfig.Receiver{
		Name:         "rancherlabs",
		SlackConfigs: slackConfigs,
	}
	receivers = append(receivers, initReceiver)

	config.Receivers = receivers

	groupWait, _ := model.ParseDuration("1m")
	groupInterval, _ := model.ParseDuration("10s")
	repeatInterval, _ := model.ParseDuration("1h")

	config.Route = &alertconfig.Route{
		Receiver:       "rancherlabs",
		GroupWait:      &groupWait,
		GroupInterval:  &groupInterval,
		RepeatInterval: &repeatInterval,
	}

	config.Templates = []string{"/etc/alertmanager/config/notification.tmpl"}

	return &config
}

func (m *AlertManager) GetAlertList() ([]*APIAlert, error) {

	url, err := m.GetAlertManagerEndpoint()
	if err != nil {
		return nil, err
	}
	res := struct {
		Data   []*APIAlert `json:"data"`
		Status string      `json:"status"`
	}{}

	req, err := http.NewRequest(http.MethodGet, url+"/api/v1/alerts", nil)
	if err != nil {
		return nil, err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	requestBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(requestBytes, &res); err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (m *AlertManager) GetState(matcherName, matcherValue string, apiAlerts []*APIAlert) string {

	for _, a := range apiAlerts {
		if string(a.Labels[model.LabelName(matcherName)]) == matcherValue {
			if a.Status.State == "suppressed" {
				return "muted"
			}
			return "alerting"
		}
	}

	return "active"
}

func (m *AlertManager) AddSilenceRule(matcherName, matcherValue string) error {

	url, err := m.GetAlertManagerEndpoint()
	if err != nil {
		return err
	}

	matchers := []*model.Matcher{}
	m1 := &model.Matcher{
		Name:    model.LabelName(matcherName),
		Value:   matcherValue,
		IsRegex: false,
	}
	matchers = append(matchers, m1)

	now := time.Now()
	endsAt := now.AddDate(100, 0, 0)
	silence := model.Silence{
		Matchers:  matchers,
		StartsAt:  now,
		EndsAt:    endsAt,
		CreatedAt: now,
		CreatedBy: "rancherlabs",
		Comment:   "silence",
	}

	silenceData, err := json.Marshal(silence)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url+"/api/v1/silences", bytes.NewBuffer(silenceData))
	if err != nil {
		return err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil

}

func (m *AlertManager) RemoveSilenceRule(matcherName, matcherValue string) error {
	url, err := m.GetAlertManagerEndpoint()
	if err != nil {
		return err
	}
	res := struct {
		Data   []*Silence `json:"data"`
		Status string     `json:"status"`
	}{}

	req, err := http.NewRequest(http.MethodGet, url+"/api/v1/silences", nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	q.Add("filter", fmt.Sprintf("{%s=%s}", matcherName, matcherValue))
	req.URL.RawQuery = q.Encode()

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	requestBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(requestBytes, &res); err != nil {
		return err
	}

	if res.Status != "success" {
		return fmt.Errorf("Failed to get silence rules for alert")
	}

	for _, s := range res.Data {
		if s.Status.State == SilenceStateActive {
			delReq, err := http.NewRequest(http.MethodDelete, url+"/api/v1/silence/"+s.ID, nil)
			if err != nil {
				return err
			}

			delResp, err := m.client.Do(delReq)
			if err != nil {
				return err
			}
			defer delResp.Body.Close()

			_, err = ioutil.ReadAll(delResp.Body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *AlertManager) SendAlert(labels map[string]string) error {
	url, err := m.GetAlertManagerEndpoint()
	if err != nil {
		return err
	}

	alertList := model.Alerts{}
	a := &model.Alert{}
	a.Labels = map[model.LabelName]model.LabelValue{}
	for k, v := range labels {
		a.Labels[model.LabelName(k)] = model.LabelValue(v)
	}

	alertList = append(alertList, a)

	alertData, err := json.Marshal(alertList)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url+"/api/v1/alerts", bytes.NewBuffer(alertData))
	if err != nil {
		return err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alertmanager response is %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
