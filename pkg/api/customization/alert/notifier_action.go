package alert

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NotifierCollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, "send")
}

func NotifierFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "send")
}

func (h *Handler) NotifierActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "send":
		return h.testNotifier(actionName, action, apiContext)
	}

	return httperror.NewAPIError(httperror.InvalidAction, "invalid action: "+actionName)

}

func (h *Handler) testNotifier(actionName string, action *types.Action, apiContext *types.APIContext) error {
	actionInput, err := parse.ReadBody(apiContext.Request)
	if err != nil {
		return err
	}

	msg := ""
	msgInf, exist := actionInput["message"]
	if exist {
		message, ok := msgInf.(string)
		if ok {
			msg = message
		}
	}

	if apiContext.ID != "" {
		parts := strings.Split(apiContext.ID, ":")
		ns := parts[0]
		id := parts[1]
		notifier, err := h.Notifiers.GetNamespaced(ns, id, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if notifier.Spec.SlackConfig != nil {
			return testSlack(notifier.Spec.SlackConfig.URL, notifier.Spec.SlackConfig.DefaultRecipient, msg)
		}

		if notifier.Spec.SMTPConfig != nil {
			s := notifier.Spec.SMTPConfig
			return testEmail(s.Host, s.Password, s.Username, int(s.Port), s.TLS, msg, s.DefaultRecipient)
		}

		if notifier.Spec.PagerdutyConfig != nil {
			return testPagerduty(notifier.Spec.PagerdutyConfig.ServiceKey, msg)
		}

		if notifier.Spec.WebhookConfig != nil {
			return testWebhook(notifier.Spec.WebhookConfig.URL, msg)
		}

	} else {

		slackConfigInterface, exist := actionInput["slackConfig"]
		if exist {
			slackConfig := convert.ToMapInterface(slackConfigInterface)
			url, ok := slackConfig["url"].(string)
			if ok {
				channel := slackConfig["defaultRecipient"].(string)
				return testSlack(url, channel, msg)
			}
		}

		smtpConfigInterface, exist := actionInput["smtpConfig"]
		if exist {
			smtpConfig := convert.ToMapInterface(smtpConfigInterface)
			host, ok := smtpConfig["host"].(string)
			if ok {
				port, _ := smtpConfig["port"].(json.Number).Int64()
				password := smtpConfig["password"].(string)
				username := smtpConfig["username"].(string)
				receiver := smtpConfig["defaultRecipient"].(string)
				tls := smtpConfig["tls"].(bool)
				return testEmail(host, password, username, int(port), tls, msg, receiver)
			}
		}

		webhookConfigInterface, exist := actionInput["webhookConfig"]
		if exist {
			webhookConfig := convert.ToMapInterface(webhookConfigInterface)
			url, ok := webhookConfig["url"].(string)
			if ok {
				return testWebhook(url, msg)
			}
		}

		pagerdutyConfigInterface, exist := actionInput["pagerdutyConfig"]
		if exist {
			pagerdutyConfig := convert.ToMapInterface(pagerdutyConfigInterface)
			key, ok := pagerdutyConfig["serviceKey"].(string)
			if ok {
				return testPagerduty(key, msg)
			}
		}

		return httperror.NewAPIError(httperror.ErrorCode{Status: 400}, "Notifier not configured")
	}

	return nil
}

type pagerDutyMessage struct {
	RoutingKey  string `json:"routing_key,omitempty"`
	ServiceKey  string `json:"service_key,omitempty"`
	DedupKey    string `json:"dedup_key,omitempty"`
	IncidentKey string `json:"incident_key,omitempty"`
	EventType   string `json:"event_type,omitempty"`
	Description string `json:"description,omitempty"`
}

func hashKey(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func testPagerduty(key, msg string) error {
	if msg == "" {
		msg = "test pagerduty service key"
	}

	pd := &pagerDutyMessage{
		ServiceKey:  key,
		EventType:   "trigger",
		IncidentKey: hashKey("key"),
		Description: msg,
	}

	url := "https://events.pagerduty.com/generic/2010-04-15/create_event.json"

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(pd); err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status code is not 200")
	}

	return nil
}

func testWebhook(url, msg string) error {
	if msg == "" {
		msg = "test webhook"
	}
	alertList := model.Alerts{
		&model.Alert{
			Labels: map[model.LabelName]model.LabelValue{
				model.LabelName("test_msg"): model.LabelValue(msg),
			},
		},
	}

	alertData, err := json.Marshal(alertList)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(alertData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status code is not 200")
	}

	return nil
}

func testSlack(url, channel, msg string) error {
	if msg == "" {
		msg = "test slack webhook"
	}
	req := struct {
		Text    string `json:"text"`
		Channel string `json:"channel"`
	}{}

	req.Text = msg
	req.Channel = channel

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status code is not 200")
	}

	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(res) != "ok" {
		return fmt.Errorf("http response is not ok")
	}

	return nil
}

func testEmail(host, password, username string, port int, requireTLS bool, msg, receiver string) error {
	var c *smtp.Client
	smartHost := host + ":" + strconv.Itoa(port)

	timeout := 15 * time.Second
	if requireTLS {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", smartHost, &tls.Config{ServerName: host})
		if err != nil {
			return err
		}
		c, err = smtp.NewClient(conn, smartHost)
		if err != nil {
			return err
		}

	} else {
		conn, err := net.DialTimeout("tcp", smartHost, timeout)
		if err != nil {
			return err
		}
		c, err = smtp.NewClient(conn, smartHost)
		if err != nil {
			return err
		}
	}
	defer c.Quit()

	if ok, mech := c.Extension("AUTH"); ok {
		auth, err := auth(mech, username, password)
		if err != nil {
			return err
		}
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("%T failed: %s", auth, err)
			}
		}
	}

	if msg != "" {
		if err := c.Mail(username); err != nil {
			return fmt.Errorf("fail to set sender: %v", err)
		}

		if err := c.Rcpt(receiver); err != nil {
			return fmt.Errorf("fail to set recipient: %v", err)
		}

		// Data
		w, err := c.Data()
		if err != nil {
			return err
		}
		defer w.Close()

		_, err = w.Write([]byte(msg))
		if err != nil {
			return err
		}
	}

	return nil
}

func auth(mechs string, username, password string) (smtp.Auth, error) {

	for _, mech := range strings.Split(mechs, " ") {
		switch mech {
		case "LOGIN":
			if password == "" {
				continue
			}

			return &loginAuth{username, password}, nil
		}
	}
	return nil, fmt.Errorf("smtp server does not support login auth")
}

type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

// Used for AUTH LOGIN. (Maybe password should be encrypted)
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch strings.ToLower(string(fromServer)) {
		case "username:":
			return []byte(a.username), nil
		case "password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("unexpected server challenge")
		}
	}
	return nil, nil
}
