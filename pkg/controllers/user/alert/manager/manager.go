package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/common/model"
	alertconfig "github.com/rancher/rancher/pkg/controllers/user/alert/config"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	"github.com/sirupsen/logrus"
)

type AlertState string

const (
	AlertStateUnprocessed AlertState = "unprocessed"
	AlertStateActive                 = "active"
	AlertStateSuppressed             = "suppressed"
)

type AlertStatus struct {
	State       AlertState `json:"state"`
	SilencedBy  []string   `json:"silencedBy"`
	InhibitedBy []string   `json:"inhibitedBy"`
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

type Manager struct {
	svcLister   v1.ServiceLister
	dialer      dialer.Factory
	IsDeploy    bool
	clusterName string
	client      *http.Client
}

func NewManager(cluster *config.UserContext) *Manager {

	dial, err := cluster.Management.Dialer.ClusterDialer(cluster.ClusterName)
	if err != nil {
		logrus.Warnf("Failed to get cluster dialer: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			Dial: dial,
		},
		Timeout: 15 * time.Second,
	}

	return &Manager{
		svcLister:   cluster.Core.Services("").Controller().Lister(),
		client:      client,
		IsDeploy:    false,
		clusterName: cluster.ClusterName,
	}
}

func (m *Manager) getAlertManagerEndpoint() (string, error) {

	svc, err := m.svcLister.Get("cattle-alerting", "alertmanager-svc")
	if err != nil {
		return "", fmt.Errorf("Failed to get service for alertmanager")
	}

	ip := svc.Spec.ClusterIP
	port := svc.Spec.Ports[0].Port
	url := "http://" + ip + ":" + strconv.Itoa(int(port))

	return url, nil
}

func (m *Manager) GetDefaultConfig() *alertconfig.Config {
	config := alertconfig.Config{}

	resolveTimeout, _ := model.ParseDuration("5m")
	config.Global = &alertconfig.GlobalConfig{
		SlackAPIURL:    "slack_api_url",
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

	config.Templates = []string{"/etc/alertmanager/notification.tmpl"}

	return &config
}

func (m *Manager) GetAlertList() ([]*APIAlert, error) {

	url, err := m.getAlertManagerEndpoint()
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

func (m *Manager) GetState(alertID string, apiAlerts []*APIAlert) string {

	for _, a := range apiAlerts {
		if string(a.Labels["alert_id"]) == alertID {
			if a.Status.State == "suppressed" {
				return "muted"
			}
			return "alerting"
		}
	}

	return "active"
}

func (m *Manager) AddSilenceRule(alertID string) error {

	url, err := m.getAlertManagerEndpoint()
	if err != nil {
		return err
	}

	matchers := []*model.Matcher{}
	m1 := &model.Matcher{
		Name:    "alert_id",
		Value:   alertID,
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

func (m *Manager) RemoveSilenceRule(alertID string) error {
	url, err := m.getAlertManagerEndpoint()
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
	q.Add("filter", fmt.Sprintf("{%s}", "alert_id="+alertID))
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

func (m *Manager) SendAlert(labels map[string]string) error {
	url, err := m.getAlertManagerEndpoint()
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
