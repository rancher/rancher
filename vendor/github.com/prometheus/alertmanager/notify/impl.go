// Copyright 2015 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package notify

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"net/textproto"
)

type notifierConfig interface {
	SendResolved() bool
}

// A Notifier notifies about alerts under constraints of the given context.
// It returns an error if unsuccessful and a flag whether the error is
// recoverable. This information is useful for a retry logic.
type Notifier interface {
	Notify(context.Context, ...*types.Alert) (bool, error)
}

// An Integration wraps a notifier and its config to be uniquely identified by
// name and index from its origin in the configuration.
type Integration struct {
	notifier Notifier
	conf     notifierConfig
	name     string
	idx      int
}

// Notify implements the Notifier interface.
func (i *Integration) Notify(ctx context.Context, alerts ...*types.Alert) (bool, error) {
	var res []*types.Alert

	// Resolved alerts have to be filtered only at this point, because they need
	// to end up unfiltered in the SetNotifiesStage.
	if i.conf.SendResolved() {
		res = alerts
	} else {
		for _, a := range alerts {
			if a.Status() != model.AlertResolved {
				res = append(res, a)
			}
		}
	}
	if len(res) == 0 {
		return false, nil
	}

	return i.notifier.Notify(ctx, res...)
}

// BuildReceiverIntegrations builds a list of integration notifiers off of a
// receivers config.
func BuildReceiverIntegrations(nc *config.Receiver, tmpl *template.Template) []Integration {
	var (
		integrations []Integration
		add          = func(name string, i int, n Notifier, nc notifierConfig) {
			integrations = append(integrations, Integration{
				notifier: n,
				conf:     nc,
				name:     name,
				idx:      i,
			})
		}
	)

	for i, c := range nc.WebhookConfigs {
		n := NewWebhook(c, tmpl)
		add("webhook", i, n, c)
	}
	for i, c := range nc.EmailConfigs {
		n := NewEmail(c, tmpl)
		add("email", i, n, c)
	}
	for i, c := range nc.PagerdutyConfigs {
		n := NewPagerDuty(c, tmpl)
		add("pagerduty", i, n, c)
	}
	for i, c := range nc.OpsGenieConfigs {
		n := NewOpsGenie(c, tmpl)
		add("opsgenie", i, n, c)
	}
	for i, c := range nc.SlackConfigs {
		n := NewSlack(c, tmpl)
		add("slack", i, n, c)
	}
	for i, c := range nc.HipchatConfigs {
		n := NewHipchat(c, tmpl)
		add("hipchat", i, n, c)
	}
	for i, c := range nc.VictorOpsConfigs {
		n := NewVictorOps(c, tmpl)
		add("victorops", i, n, c)
	}
	for i, c := range nc.PushoverConfigs {
		n := NewPushover(c, tmpl)
		add("pushover", i, n, c)
	}
	return integrations
}

const contentTypeJSON = "application/json"

var userAgentHeader = fmt.Sprintf("Alertmanager/%s", version.Version)

// Webhook implements a Notifier for generic webhooks.
type Webhook struct {
	// The URL to which notifications are sent.
	URL  string
	tmpl *template.Template
}

// NewWebhook returns a new Webhook.
func NewWebhook(conf *config.WebhookConfig, t *template.Template) *Webhook {
	return &Webhook{URL: conf.URL, tmpl: t}
}

// WebhookMessage defines the JSON object send to webhook endpoints.
type WebhookMessage struct {
	*template.Data

	// The protocol version.
	Version  string `json:"version"`
	GroupKey string `json:"groupKey"`
}

// Notify implements the Notifier interface.
func (w *Webhook) Notify(ctx context.Context, alerts ...*types.Alert) (bool, error) {
	data := w.tmpl.Data(receiverName(ctx), groupLabels(ctx), alerts...)

	groupKey, ok := GroupKey(ctx)
	if !ok {
		log.Errorf("group key missing")
	}

	msg := &WebhookMessage{
		Version:  "4",
		Data:     data,
		GroupKey: groupKey,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(msg); err != nil {
		return false, err
	}

	req, err := http.NewRequest("POST", w.URL, &buf)
	if err != nil {
		return true, err
	}
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("User-Agent", userAgentHeader)

	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)
	if err != nil {
		return true, err
	}
	resp.Body.Close()

	return w.retry(resp.StatusCode)
}

func (w *Webhook) retry(statusCode int) (bool, error) {
	// Webhooks are assumed to respond with 2xx response codes on a successful
	// request and 5xx response codes are assumed to be recoverable.
	if statusCode/100 != 2 {
		return (statusCode/100 == 5), fmt.Errorf("unexpected status code %v from %s", statusCode, w.URL)
	}

	return false, nil
}

// Email implements a Notifier for email notifications.
type Email struct {
	conf *config.EmailConfig
	tmpl *template.Template
}

// NewEmail returns a new Email notifier.
func NewEmail(c *config.EmailConfig, t *template.Template) *Email {
	if _, ok := c.Headers["Subject"]; !ok {
		c.Headers["Subject"] = config.DefaultEmailSubject
	}
	if _, ok := c.Headers["To"]; !ok {
		c.Headers["To"] = c.To
	}
	if _, ok := c.Headers["From"]; !ok {
		c.Headers["From"] = c.From
	}
	return &Email{conf: c, tmpl: t}
}

// auth resolves a string of authentication mechanisms.
func (n *Email) auth(mechs string) (smtp.Auth, error) {
	username := n.conf.AuthUsername

	for _, mech := range strings.Split(mechs, " ") {
		switch mech {
		case "CRAM-MD5":
			secret := string(n.conf.AuthSecret)
			if secret == "" {
				continue
			}
			return smtp.CRAMMD5Auth(username, secret), nil

		case "PLAIN":
			password := string(n.conf.AuthPassword)
			if password == "" {
				continue
			}
			identity := n.conf.AuthIdentity

			// We need to know the hostname for both auth and TLS.
			host, _, err := net.SplitHostPort(n.conf.Smarthost)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %s", err)
			}
			return smtp.PlainAuth(identity, username, password, host), nil
		case "LOGIN":
			password := string(n.conf.AuthPassword)
			if password == "" {
				continue
			}
			return LoginAuth(username, password), nil
		}
	}
	return nil, nil
}

// Notify implements the Notifier interface.
func (n *Email) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	// We need to know the hostname for both auth and TLS.
	var c *smtp.Client
	host, port, err := net.SplitHostPort(n.conf.Smarthost)
	if err != nil {
		return false, fmt.Errorf("invalid address: %s", err)
	}

	if port == "465" {
		conn, err := tls.Dial("tcp", n.conf.Smarthost, &tls.Config{ServerName: host})
		if err != nil {
			return true, err
		}
		c, err = smtp.NewClient(conn, n.conf.Smarthost)
		if err != nil {
			return true, err
		}

	} else {
		// Connect to the SMTP smarthost.
		c, err = smtp.Dial(n.conf.Smarthost)
		if err != nil {
			return true, err
		}
	}
	defer c.Quit()

	if n.conf.Hello != "" {
		err := c.Hello(n.conf.Hello)
		if err != nil {
			return true, err
		}
	}

	// Global Config guarantees RequireTLS is not nil
	if *n.conf.RequireTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return true, fmt.Errorf("require_tls: true (default), but %q does not advertise the STARTTLS extension", n.conf.Smarthost)
		}
		tlsConf := &tls.Config{ServerName: host}
		if err := c.StartTLS(tlsConf); err != nil {
			return true, fmt.Errorf("starttls failed: %s", err)
		}
	}

	if ok, mech := c.Extension("AUTH"); ok {
		auth, err := n.auth(mech)
		if err != nil {
			return true, err
		}
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return true, fmt.Errorf("%T failed: %s", auth, err)
			}
		}
	}

	var (
		data = n.tmpl.Data(receiverName(ctx), groupLabels(ctx), as...)
		tmpl = tmplText(n.tmpl, data, &err)
		from = tmpl(n.conf.From)
		to   = tmpl(n.conf.To)
	)
	if err != nil {
		return false, err
	}

	addrs, err := mail.ParseAddressList(from)
	if err != nil {
		return false, fmt.Errorf("parsing from addresses: %s", err)
	}
	if len(addrs) != 1 {
		return false, fmt.Errorf("must be exactly one from address")
	}
	if err := c.Mail(addrs[0].Address); err != nil {
		return true, fmt.Errorf("sending mail from: %s", err)
	}
	addrs, err = mail.ParseAddressList(to)
	if err != nil {
		return false, fmt.Errorf("parsing to addresses: %s", err)
	}
	for _, addr := range addrs {
		if err := c.Rcpt(addr.Address); err != nil {
			return true, fmt.Errorf("sending rcpt to: %s", err)
		}
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return true, err
	}
	defer wc.Close()

	for header, t := range n.conf.Headers {
		value, err := n.tmpl.ExecuteTextString(t, data)
		if err != nil {
			return false, fmt.Errorf("executing %q header template: %s", header, err)
		}
		fmt.Fprintf(wc, "%s: %s\r\n", header, mime.QEncoding.Encode("utf-8", value))
	}

	buffer := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(buffer)

	fmt.Fprintf(wc, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(wc, "Content-Type: multipart/alternative;  boundary=%s\r\n", multipartWriter.Boundary())
	fmt.Fprintf(wc, "MIME-Version: 1.0\r\n")

	// TODO: Add some useful headers here, such as URL of the alertmanager
	// and active/resolved.
	fmt.Fprintf(wc, "\r\n")

	if len(n.conf.Text) > 0 {
		// Text template
		w, err := multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}})
		if err != nil {
			return false, fmt.Errorf("creating part for text template: %s", err)
		}
		body, err := n.tmpl.ExecuteTextString(n.conf.Text, data)
		if err != nil {
			return false, fmt.Errorf("executing email text template: %s", err)
		}
		_, err = w.Write([]byte(body))
		if err != nil {
			return true, err
		}
	}

	if len(n.conf.HTML) > 0 {
		// Html template
		// Preferred alternative placed last per section 5.1.4 of RFC 2046
		// https://www.ietf.org/rfc/rfc2046.txt
		w, err := multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/html; charset=UTF-8"}})
		if err != nil {
			return false, fmt.Errorf("creating part for html template: %s", err)
		}
		body, err := n.tmpl.ExecuteHTMLString(n.conf.HTML, data)
		if err != nil {
			return false, fmt.Errorf("executing email html template: %s", err)
		}
		_, err = w.Write([]byte(body))
		if err != nil {
			return true, err
		}
	}

	multipartWriter.Close()
	wc.Write(buffer.Bytes())

	return false, nil
}

// PagerDuty implements a Notifier for PagerDuty notifications.
type PagerDuty struct {
	conf *config.PagerdutyConfig
	tmpl *template.Template
}

// NewPagerDuty returns a new PagerDuty notifier.
func NewPagerDuty(c *config.PagerdutyConfig, t *template.Template) *PagerDuty {
	return &PagerDuty{conf: c, tmpl: t}
}

const (
	pagerDutyEventTrigger = "trigger"
	pagerDutyEventResolve = "resolve"
)

type pagerDutyMessage struct {
	ServiceKey  string            `json:"service_key"`
	IncidentKey string            `json:"incident_key"`
	EventType   string            `json:"event_type"`
	Description string            `json:"description"`
	Client      string            `json:"client,omitempty"`
	ClientURL   string            `json:"client_url,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
}

// Notify implements the Notifier interface.
//
// http://developer.pagerduty.com/documentation/integration/events/trigger
func (n *PagerDuty) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	key, ok := GroupKey(ctx)
	if !ok {
		return false, fmt.Errorf("group key missing")
	}

	var err error
	var (
		alerts    = types.Alerts(as...)
		data      = n.tmpl.Data(receiverName(ctx), groupLabels(ctx), as...)
		tmpl      = tmplText(n.tmpl, data, &err)
		eventType = pagerDutyEventTrigger
	)
	if alerts.Status() == model.AlertResolved {
		eventType = pagerDutyEventResolve
	}

	log.With("incident", key).With("eventType", eventType).Debugln("notifying PagerDuty")

	details := make(map[string]string, len(n.conf.Details))
	for k, v := range n.conf.Details {
		details[k] = tmpl(v)
	}

	msg := &pagerDutyMessage{
		ServiceKey:  tmpl(string(n.conf.ServiceKey)),
		EventType:   eventType,
		IncidentKey: hashKey(key),
		Description: tmpl(n.conf.Description),
		Details:     details,
	}
	if eventType == pagerDutyEventTrigger {
		msg.Client = tmpl(n.conf.Client)
		msg.ClientURL = tmpl(n.conf.ClientURL)
	}
	if err != nil {
		return false, err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(msg); err != nil {
		return false, err
	}

	resp, err := ctxhttp.Post(ctx, http.DefaultClient, n.conf.URL, contentTypeJSON, &buf)
	if err != nil {
		return true, err
	}
	resp.Body.Close()

	return n.retry(resp.StatusCode)
}

func (n *PagerDuty) retry(statusCode int) (bool, error) {
	// Retrying can solve the issue on 403 (rate limiting) and 5xx response codes.
	// 2xx response codes indicate a successful request.
	// https://v2.developer.pagerduty.com/docs/trigger-events
	if statusCode/100 != 2 {
		return (statusCode == 403 || statusCode/100 == 5), fmt.Errorf("unexpected status code %v", statusCode)
	}

	return false, nil
}

// Slack implements a Notifier for Slack notifications.
type Slack struct {
	conf *config.SlackConfig
	tmpl *template.Template
}

// NewSlack returns a new Slack notification handler.
func NewSlack(conf *config.SlackConfig, tmpl *template.Template) *Slack {
	return &Slack{
		conf: conf,
		tmpl: tmpl,
	}
}

// slackReq is the request for sending a slack notification.
type slackReq struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	LinkNames   bool              `json:"link_names,omitempty"`
	Attachments []slackAttachment `json:"attachments"`
}

// slackAttachment is used to display a richly-formatted message block.
type slackAttachment struct {
	Title     string `json:"title,omitempty"`
	TitleLink string `json:"title_link,omitempty"`
	Pretext   string `json:"pretext,omitempty"`
	Text      string `json:"text"`
	Fallback  string `json:"fallback"`

	Color    string   `json:"color,omitempty"`
	MrkdwnIn []string `json:"mrkdwn_in,omitempty"`
}

// slackAttachmentField is displayed in a table inside the message attachment.
type slackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short,omitempty"`
}

// Notify implements the Notifier interface.
func (n *Slack) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var err error
	var (
		data     = n.tmpl.Data(receiverName(ctx), groupLabels(ctx), as...)
		tmplText = tmplText(n.tmpl, data, &err)
	)

	attachment := &slackAttachment{
		Title:     tmplText(n.conf.Title),
		TitleLink: tmplText(n.conf.TitleLink),
		Pretext:   tmplText(n.conf.Pretext),
		Text:      tmplText(n.conf.Text),
		Fallback:  tmplText(n.conf.Fallback),
		Color:     tmplText(n.conf.Color),
		MrkdwnIn:  []string{"fallback", "pretext", "text"},
	}
	req := &slackReq{
		Channel:     tmplText(n.conf.Channel),
		Username:    tmplText(n.conf.Username),
		IconEmoji:   tmplText(n.conf.IconEmoji),
		IconURL:     tmplText(n.conf.IconURL),
		LinkNames:   n.conf.LinkNames,
		Attachments: []slackAttachment{*attachment},
	}
	if err != nil {
		return false, err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		return false, err
	}

	resp, err := ctxhttp.Post(ctx, http.DefaultClient, string(n.conf.APIURL), contentTypeJSON, &buf)
	if err != nil {
		return true, err
	}
	resp.Body.Close()

	return n.retry(resp.StatusCode)
}

func (n *Slack) retry(statusCode int) (bool, error) {
	// Only 5xx response codes are recoverable and 2xx codes are successful.
	// https://api.slack.com/incoming-webhooks#handling_errors
	// https://api.slack.com/changelog/2016-05-17-changes-to-errors-for-incoming-webhooks
	if statusCode/100 != 2 {
		return (statusCode/100 == 5), fmt.Errorf("unexpected status code %v", statusCode)
	}

	return false, nil
}

// Hipchat implements a Notifier for Hipchat notifications.
type Hipchat struct {
	conf *config.HipchatConfig
	tmpl *template.Template
}

// NewHipchat returns a new Hipchat notification handler.
func NewHipchat(conf *config.HipchatConfig, tmpl *template.Template) *Hipchat {
	return &Hipchat{
		conf: conf,
		tmpl: tmpl,
	}
}

type hipchatReq struct {
	From          string `json:"from"`
	Notify        bool   `json:"notify"`
	Message       string `json:"message"`
	MessageFormat string `json:"message_format"`
	Color         string `json:"color"`
}

// Notify implements the Notifier interface.
func (n *Hipchat) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var err error
	var msg string
	var (
		data     = n.tmpl.Data(receiverName(ctx), groupLabels(ctx), as...)
		tmplText = tmplText(n.tmpl, data, &err)
		tmplHTML = tmplHTML(n.tmpl, data, &err)
		url      = fmt.Sprintf("%sv2/room/%s/notification?auth_token=%s", n.conf.APIURL, n.conf.RoomID, n.conf.AuthToken)
	)

	if n.conf.MessageFormat == "html" {
		msg = tmplHTML(n.conf.Message)
	} else {
		msg = tmplText(n.conf.Message)
	}

	req := &hipchatReq{
		From:          tmplText(n.conf.From),
		Notify:        n.conf.Notify,
		Message:       msg,
		MessageFormat: n.conf.MessageFormat,
		Color:         tmplText(n.conf.Color),
	}
	if err != nil {
		return false, err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		return false, err
	}

	resp, err := ctxhttp.Post(ctx, http.DefaultClient, url, contentTypeJSON, &buf)
	if err != nil {
		return true, err
	}

	defer resp.Body.Close()

	return n.retry(resp.StatusCode)
}

func (n *Hipchat) retry(statusCode int) (bool, error) {
	// Response codes 429 (rate limiting) and 5xx can potentially recover. 2xx
	// responce codes indicate successful requests.
	// https://developer.atlassian.com/hipchat/guide/hipchat-rest-api/api-response-codes
	if statusCode/100 != 2 {
		return (statusCode == 429 || statusCode/100 == 5), fmt.Errorf("unexpected status code %v", statusCode)
	}

	return false, nil
}

// OpsGenie implements a Notifier for OpsGenie notifications.
type OpsGenie struct {
	conf *config.OpsGenieConfig
	tmpl *template.Template
}

// NewOpsGenie returns a new OpsGenie notifier.
func NewOpsGenie(c *config.OpsGenieConfig, t *template.Template) *OpsGenie {
	return &OpsGenie{conf: c, tmpl: t}
}

type opsGenieMessage struct {
	APIKey string `json:"apiKey"`
	Alias  string `json:"alias"`
}

type opsGenieCreateMessage struct {
	*opsGenieMessage `json:",inline"`

	Message     string            `json:"message"`
	Description string            `json:"description,omitempty"`
	Details     map[string]string `json:"details"`
	Source      string            `json:"source"`
	Teams       string            `json:"teams,omitempty"`
	Tags        string            `json:"tags,omitempty"`
	Note        string            `json:"note,omitempty"`
}

type opsGenieCloseMessage struct {
	*opsGenieMessage `json:",inline"`
}

type opsGenieErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

// Notify implements the Notifier interface.
func (n *OpsGenie) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	key, ok := GroupKey(ctx)
	if !ok {
		return false, fmt.Errorf("group key missing")
	}
	data := n.tmpl.Data(receiverName(ctx), groupLabels(ctx), as...)

	log.With("incident", key).Debugln("notifying OpsGenie")

	var err error
	tmpl := tmplText(n.tmpl, data, &err)

	details := make(map[string]string, len(n.conf.Details))
	for k, v := range n.conf.Details {
		details[k] = tmpl(v)
	}

	var (
		msg    interface{}
		apiURL string

		apiMsg = opsGenieMessage{
			APIKey: string(n.conf.APIKey),
			Alias:  hashKey(key),
		}
		alerts = types.Alerts(as...)
	)
	switch alerts.Status() {
	case model.AlertResolved:
		apiURL = n.conf.APIHost + "v1/json/alert/close"
		msg = &opsGenieCloseMessage{&apiMsg}
	default:
		apiURL = n.conf.APIHost + "v1/json/alert"
		msg = &opsGenieCreateMessage{
			opsGenieMessage: &apiMsg,
			Message:         tmpl(n.conf.Message),
			Description:     tmpl(n.conf.Description),
			Details:         details,
			Source:          tmpl(n.conf.Source),
			Teams:           tmpl(n.conf.Teams),
			Tags:            tmpl(n.conf.Tags),
			Note:            tmpl(n.conf.Note),
		}
	}
	if err != nil {
		return false, fmt.Errorf("templating error: %s", err)
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(msg); err != nil {
		return false, err
	}

	resp, err := ctxhttp.Post(ctx, http.DefaultClient, apiURL, contentTypeJSON, &buf)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()

	// Missing documentation therefore assuming only 5xx response codes are
	// recoverable.
	if resp.StatusCode/100 == 5 {
		return true, fmt.Errorf("unexpected status code %v", resp.StatusCode)
	} else if resp.StatusCode == 400 && alerts.Status() == model.AlertResolved {
		body, _ := ioutil.ReadAll(resp.Body)

		var responseMessage opsGenieErrorResponse
		if err := json.Unmarshal(body, &responseMessage); err != nil {
			return false, fmt.Errorf("could not parse error response %q", body)
		}
		const alreadyClosedError = 5
		if responseMessage.Code == alreadyClosedError {
			return false, nil
		}
		return false, fmt.Errorf("error when closing alert: code %d, error %q",
			responseMessage.Code, responseMessage.Error)
	} else if resp.StatusCode/100 == 4 {
		return false, fmt.Errorf("unexpected status code %v", resp.StatusCode)
	} else if resp.StatusCode/100 != 2 {
		body, _ := ioutil.ReadAll(resp.Body)
		log.With("incident", key).Debugf("unexpected OpsGenie response from %s (POSTed %s), %s: %s",
			apiURL, msg, resp.Status, body)
		return false, fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}
	return false, nil
}

// VictorOps implements a Notifier for VictorOps notifications.
type VictorOps struct {
	conf *config.VictorOpsConfig
	tmpl *template.Template
}

// NewVictorOps returns a new VictorOps notifier.
func NewVictorOps(c *config.VictorOpsConfig, t *template.Template) *VictorOps {
	return &VictorOps{
		conf: c,
		tmpl: t,
	}
}

const (
	victorOpsEventTrigger = "CRITICAL"
	victorOpsEventResolve = "RECOVERY"
)

type victorOpsMessage struct {
	MessageType       string `json:"message_type"`
	EntityID          string `json:"entity_id"`
	EntityDisplayName string `json:"entity_display_name"`
	StateMessage      string `json:"state_message"`
	MonitoringTool    string `json:"monitoring_tool"`
}

type victorOpsErrorResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

// Notify implements the Notifier interface.
func (n *VictorOps) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	victorOpsAllowedEvents := map[string]bool{
		"INFO":     true,
		"WARNING":  true,
		"CRITICAL": true,
	}

	key, ok := GroupKey(ctx)
	if !ok {
		return false, fmt.Errorf("group key missing")
	}

	var err error
	var (
		alerts       = types.Alerts(as...)
		data         = n.tmpl.Data(receiverName(ctx), groupLabels(ctx), as...)
		tmpl         = tmplText(n.tmpl, data, &err)
		apiURL       = fmt.Sprintf("%s%s/%s", n.conf.APIURL, n.conf.APIKey, n.conf.RoutingKey)
		messageType  = n.conf.MessageType
		stateMessage = tmpl(n.conf.StateMessage)
	)

	if alerts.Status() == model.AlertFiring && !victorOpsAllowedEvents[messageType] {
		messageType = victorOpsEventTrigger
	}

	if alerts.Status() == model.AlertResolved {
		messageType = victorOpsEventResolve
	}

	if len(stateMessage) > 20480 {
		stateMessage = stateMessage[0:20475] + "\n..."
	}

	msg := &victorOpsMessage{
		MessageType:       messageType,
		EntityID:          hashKey(key),
		EntityDisplayName: tmpl(n.conf.EntityDisplayName),
		StateMessage:      stateMessage,
		MonitoringTool:    tmpl(n.conf.MonitoringTool),
	}

	if err != nil {
		return false, fmt.Errorf("templating error: %s", err)
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(msg); err != nil {
		return false, err
	}

	resp, err := ctxhttp.Post(ctx, http.DefaultClient, apiURL, contentTypeJSON, &buf)
	if err != nil {
		return true, err
	}

	defer resp.Body.Close()

	// Missing documentation therefore assuming only 5xx response codes are
	// recoverable.
	if resp.StatusCode/100 == 5 {
		return true, fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}

	if resp.StatusCode/100 != 2 {
		body, _ := ioutil.ReadAll(resp.Body)

		var responseMessage victorOpsErrorResponse
		if err := json.Unmarshal(body, &responseMessage); err != nil {
			return false, fmt.Errorf("could not parse error response %q", body)
		}

		log.With("incident", key).Debugf("unexpected VictorOps response from %s (POSTed %s), %s: %s", apiURL, msg, resp.Status, body)

		return false, fmt.Errorf("error when posting alert: result %q, message %q",
			responseMessage.Result, responseMessage.Message)
	}

	return false, nil
}

// Pushover implements a Notifier for Pushover notifications.
type Pushover struct {
	conf *config.PushoverConfig
	tmpl *template.Template
}

// NewPushover returns a new Pushover notifier.
func NewPushover(c *config.PushoverConfig, t *template.Template) *Pushover {
	return &Pushover{conf: c, tmpl: t}
}

// Notify implements the Notifier interface.
func (n *Pushover) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	key, ok := GroupKey(ctx)
	if !ok {
		return false, fmt.Errorf("group key missing")
	}
	data := n.tmpl.Data(receiverName(ctx), groupLabels(ctx), as...)

	log.With("incident", key).Debugln("notifying Pushover")

	var err error
	tmpl := tmplText(n.tmpl, data, &err)

	parameters := url.Values{}
	parameters.Add("token", tmpl(string(n.conf.Token)))
	parameters.Add("user", tmpl(string(n.conf.UserKey)))
	title := tmpl(n.conf.Title)
	message := tmpl(n.conf.Message)
	parameters.Add("title", title)
	if len(title) > 512 {
		title = title[:512]
		log.With("incident", key).Debugf("Truncated title to %q due to Pushover message limit", title)
	}
	if len(title)+len(message) > 512 {
		message = message[:512-len(title)]
		log.With("incident", key).Debugf("Truncated message to %q due to Pushover message limit", message)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		// Pushover rejects empty messages.
		message = "(no details)"
	}
	parameters.Add("message", message)
	parameters.Add("url", tmpl(n.conf.URL))
	parameters.Add("priority", tmpl(n.conf.Priority))
	parameters.Add("retry", fmt.Sprintf("%d", int64(time.Duration(n.conf.Retry).Seconds())))
	parameters.Add("expire", fmt.Sprintf("%d", int64(time.Duration(n.conf.Expire).Seconds())))
	if err != nil {
		return false, err
	}

	apiURL := "https://api.pushover.net/1/messages.json"
	u, err := url.Parse(apiURL)
	if err != nil {
		return false, err
	}
	u.RawQuery = parameters.Encode()
	log.With("incident", key).Debugf("Pushover URL = %q", u.String())

	resp, err := ctxhttp.Post(ctx, http.DefaultClient, u.String(), "text/plain", nil)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()

	// Only documented behaviour is that 2xx response codes are successful and
	// 4xx are unsuccessful, therefore assuming only 5xx are recoverable.
	// https://pushover.net/api#response
	if resp.StatusCode/100 == 5 {
		return true, fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}

	if resp.StatusCode/100 != 2 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("unexpected status code %v (body: %s)", resp.StatusCode, string(body))
	}

	return false, nil
}

func tmplText(tmpl *template.Template, data *template.Data, err *error) func(string) string {
	return func(name string) (s string) {
		if *err != nil {
			return
		}
		s, *err = tmpl.ExecuteTextString(name, data)
		return s
	}
}

func tmplHTML(tmpl *template.Template, data *template.Data, err *error) func(string) string {
	return func(name string) (s string) {
		if *err != nil {
			return
		}
		s, *err = tmpl.ExecuteHTMLString(name, data)
		return s
	}
}

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
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

// hashKey returns the sha256 for a group key as integrations may have
// maximum length requirements on deduplication keys.
func hashKey(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}
