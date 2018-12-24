package notifiers

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type Message struct {
	Title   string
	Content string
}

type wechatToken struct {
	AccessToken string `json:"access_token"`
}

type wechatResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func SendMessage(notifier *v3.Notifier, recipient string, msg *Message) error {
	if notifier.Spec.SlackConfig != nil {
		if recipient == "" {
			recipient = notifier.Spec.SlackConfig.DefaultRecipient
		}
		return TestSlack(notifier.Spec.SlackConfig.URL, recipient, msg.Content)
	}

	if notifier.Spec.SMTPConfig != nil {
		s := notifier.Spec.SMTPConfig
		if recipient == "" {
			recipient = s.DefaultRecipient
		}
		return TestEmail(s.Host, s.Password, s.Username, int(s.Port), s.TLS, msg.Title, msg.Content, recipient, s.Sender)
	}

	if notifier.Spec.PagerdutyConfig != nil {
		return TestPagerduty(notifier.Spec.PagerdutyConfig.ServiceKey, msg.Content)
	}

	if notifier.Spec.WechatConfig != nil {
		s := notifier.Spec.WechatConfig
		if recipient == "" {
			recipient = s.DefaultRecipient
		}
		return TestWechat(notifier.Spec.WechatConfig.Secret, notifier.Spec.WechatConfig.Agent, notifier.Spec.WechatConfig.Corp, notifier.Spec.WechatConfig.RecipientType, recipient, msg.Content)
	}

	if notifier.Spec.WebhookConfig != nil {
		return TestWebhook(notifier.Spec.WebhookConfig.URL, msg.Content)
	}

	return errors.New("Notifier not configured")
}

func TestPagerduty(key, msg string) error {
	if msg == "" {
		msg = "Pagerduty setting validated"
	}

	pd := &pagerDutyEvent{
		RoutingKey:  key,
		EventAction: "trigger",
		Payload: pagerDutyEventPayload{
			Summary:  msg,
			Source:   "rancher",
			Severity: "info",
			Group:    "Rancher alert testing",
		},
	}

	url := "https://events.pagerduty.com/v2/enqueue"

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(pd); err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("http status code is %d, not include in the 2xx success HTTP status codes", resp.StatusCode)
	}

	return nil
}

func TestWechat(secret, agent, corp, receiverType, receiver, msg string) error {
	if msg == "" {
		msg = "Wechat setting validated"
	}

	req, err := http.NewRequest(http.MethodGet, "https://qyapi.weixin.qq.com/cgi-bin/gettoken", nil)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Add("corpid", corp)
	q.Add("corpsecret", secret)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	var wechatToken wechatToken
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	requestBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(requestBytes, &wechatToken); err != nil {
		return err
	}

	if wechatToken.AccessToken == "" {
		return fmt.Errorf("Invalid APISecret for CorpID. %s", corp)
	}

	wc := &wechatEvent{
		AgentID: agent,
		MsgType: "text",
		Text: wechatEventPayload{
			Content: msg,
		},
	}

	switch receiverType {
	case "tag":
		wc.ToTag = receiver
	case "user":
		wc.ToUser = receiver
	default:
		wc.ToParty = receiver
	}

	url := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + wechatToken.AccessToken

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(wc); err != nil {
		return err
	}
	resp, err = http.Post(url, "application/json", &buf)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("http status code is %d, not include in the 2xx success HTTP status codes", resp.StatusCode)
	}

	requestBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var weResp wechatResponse
	if err := json.Unmarshal(requestBytes, &weResp); err != nil {
		return err
	}

	if weResp.Code != 0 {
		return fmt.Errorf("Failed to send Wechat message. %s", weResp.Error)
	}

	return nil
}

func TestWebhook(url, msg string) error {
	if msg == "" {
		msg = "Webhook setting validated"
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

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("http status code is %d, not include in the 2xx success HTTP status codes", resp.StatusCode)
	}

	return nil
}

func TestSlack(url, channel, msg string) error {
	if msg == "" {
		msg = "Slack setting validated"
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

func TestEmail(host, password, username string, port int, requireTLS bool, title, content, receiver, sender string) error {
	if content == "" {
		content = "Alert Name: Test SMTP setting"
	}
	c, err := smtpInit(host, port)
	if err != nil {
		return err
	}
	defer c.Quit()
	if err := smtpPrepare(c, host, password, username, port, requireTLS); err != nil {
		return err
	}

	return smtpSend(c, title, content, receiver, sender)
}

func smtpInit(host string, port int) (*smtp.Client, error) {
	var c *smtp.Client
	smartHost := host + ":" + strconv.Itoa(port)
	timeout := 15 * time.Second
	if port == 465 {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", smartHost, &tls.Config{ServerName: host})
		if err != nil {
			return nil, fmt.Errorf("Failed to connect smtp server: %v", err)
		}
		c, err = smtp.NewClient(conn, smartHost)
		if err != nil {
			return nil, fmt.Errorf("Failed to connect smtp server: %v", err)
		}
	} else {
		conn, err := net.DialTimeout("tcp", smartHost, timeout)
		if err != nil {
			return nil, fmt.Errorf("Failed to connect smtp server: %v", err)
		}
		c, err = smtp.NewClient(conn, smartHost)
		if err != nil {
			return nil, fmt.Errorf("Failed to connect smtp server: %v", err)
		}
	}
	return c, nil
}

func smtpPrepare(c *smtp.Client, host, password, username string, port int, requireTLS bool) error {
	smartHost := host + ":" + strconv.Itoa(port)
	if requireTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("Require TLS but %q does not advertise the STARTTLS extension", smartHost)
		}
		tlsConf := &tls.Config{ServerName: host}
		if err := c.StartTLS(tlsConf); err != nil {
			return fmt.Errorf("Starttls failed: %v", err)
		}
	}

	if ok, mech := c.Extension("AUTH"); ok {
		if password != "" && username != "" {
			auth, err := auth(mech, username, password)
			if err != nil {
				return fmt.Errorf("Authentication failed: %v", err)
			}
			if auth != nil {
				if err := c.Auth(auth); err != nil {
					return fmt.Errorf("Authentication failed: %v", err)
				}
			}
		}
	}
	return nil
}

func smtpSend(c *smtp.Client, title, content, receiver, sender string) error {
	if err := c.Mail(sender); err != nil {
		return fmt.Errorf("Failed to set sender: %v", err)
	}

	if err := c.Rcpt(receiver); err != nil {
		return fmt.Errorf("Failed to set recipient: %v", err)
	}

	wc, err := c.Data()
	if err != nil {
		return err
	}

	defer wc.Close()

	fmt.Fprintf(wc, "%s: %s\r\n", "From", sender)
	fmt.Fprintf(wc, "%s: %s\r\n", "To", receiver)
	fmt.Fprintf(wc, "%s: %s\r\n", "Subject", title)

	buffer := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(buffer)

	fmt.Fprintf(wc, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(wc, "Content-Type: multipart/alternative;  boundary=%s\r\n", multipartWriter.Boundary())
	fmt.Fprintf(wc, "MIME-Version: 1.0\r\n")

	fmt.Fprintf(wc, "\r\n")

	w, err := multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/html; charset=UTF-8"}})
	if err != nil {
		return fmt.Errorf("Failed to send test email: %s", err)
	}

	_, err = w.Write([]byte(content))
	if err != nil {
		return fmt.Errorf("Failed to send test email: %s", err)
	}

	multipartWriter.Close()
	_, err = wc.Write(buffer.Bytes())
	if err != nil {
		return fmt.Errorf("Failed to send test email: %s", err)
	}

	return nil
}

type pagerDutyEventPayload struct {
	Summary  string `json:"summary"`
	Source   string `json:"source"`
	Severity string `json:"severity"`
	Group    string `json:"group"`
}

type pagerDutyEvent struct {
	RoutingKey  string                `json:"routing_key"`
	EventAction string                `json:"event_action"`
	Payload     pagerDutyEventPayload `json:"payload"`
}

func hashKey(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

type wechatEventPayload struct {
	Content string `json:"content"`
}

type wechatEvent struct {
	ToParty string             `json:"toparty"`
	ToUser  string             `json:"touser"`
	ToTag   string             `json:"totag"`
	AgentID string             `json:"agentid"`
	MsgType string             `json:"msgtype"`
	Text    wechatEventPayload `json:"text"`
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
