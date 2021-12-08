package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	// DefaultWebhookConfig defines default values for Webhook configurations.
	DefaultWebhookConfig = WebhookConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: true,
		},
	}

	// DefaultEmailConfig defines default values for Email configurations.
	DefaultEmailConfig = EmailConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: false,
		},
		HTML: `{{ template "email.default.html" . }}`,
		Text: ``,
	}

	// DefaultEmailSubject defines the default Subject header of an Email.
	DefaultEmailSubject = `{{ template "email.default.subject" . }}`

	// DefaultPagerdutyConfig defines default values for PagerDuty configurations.
	DefaultPagerdutyConfig = PagerdutyConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: true,
		},
		Description: `{{ template "pagerduty.default.description" .}}`,
		Client:      `{{ template "pagerduty.default.client" . }}`,
		ClientURL:   `{{ template "pagerduty.default.clientURL" . }}`,
		Details: map[string]string{
			"firing":       `{{ template "pagerduty.default.instances" .Alerts.Firing }}`,
			"resolved":     `{{ template "pagerduty.default.instances" .Alerts.Resolved }}`,
			"num_firing":   `{{ .Alerts.Firing | len }}`,
			"num_resolved": `{{ .Alerts.Resolved | len }}`,
		},
	}

	// DefaultWechatConfig defines default values for Wechat configurations.
	DefaultWechatConfig = WechatConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: true,
		},
		Message: `{{ template "wechat.default.message" . }}`,
	}

	// DefaultSlackConfig defines default values for Slack configurations.
	DefaultSlackConfig = SlackConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: false,
		},
		Color:     `{{ if eq .Status "firing" }}danger{{ else }}good{{ end }}`,
		Username:  `{{ template "slack.default.username" . }}`,
		Title:     `{{ template "slack.default.title" . }}`,
		TitleLink: `{{ template "slack.default.titlelink" . }}`,
		IconEmoji: `{{ template "slack.default.iconemoji" . }}`,
		IconURL:   `{{ template "slack.default.iconurl" . }}`,
		Pretext:   `{{ template "slack.default.pretext" . }}`,
		Text:      `{{ template "slack.default.text" . }}`,
		Fallback:  `{{ template "slack.default.fallback" . }}`,
	}

	// DefaultHipchatConfig defines default values for Hipchat configurations.
	DefaultHipchatConfig = HipchatConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: false,
		},
		Color:         `{{ if eq .Status "firing" }}red{{ else }}green{{ end }}`,
		From:          `{{ template "hipchat.default.from" . }}`,
		Notify:        false,
		Message:       `{{ template "hipchat.default.message" . }}`,
		MessageFormat: `text`,
	}

	// DefaultOpsGenieConfig defines default values for OpsGenie configurations.
	DefaultOpsGenieConfig = OpsGenieConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: true,
		},
		Message:     `{{ template "opsgenie.default.message" . }}`,
		Description: `{{ template "opsgenie.default.description" . }}`,
		Source:      `{{ template "opsgenie.default.source" . }}`,
		// TODO: Add a details field with all the alerts.
	}

	// DefaultVictorOpsConfig defines default values for VictorOps configurations.
	DefaultVictorOpsConfig = VictorOpsConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: true,
		},
		MessageType:       `CRITICAL`,
		StateMessage:      `{{ template "victorops.default.state_message" . }}`,
		EntityDisplayName: `{{ template "victorops.default.entity_display_name" . }}`,
		MonitoringTool:    `{{ template "victorops.default.monitoring_tool" . }}`,
	}

	// DefaultPushoverConfig defines default values for Pushover configurations.
	DefaultPushoverConfig = PushoverConfig{
		NotifierConfig: NotifierConfig{
			VSendResolved: true,
		},
		Title:    `{{ template "pushover.default.title" . }}`,
		Message:  `{{ template "pushover.default.message" . }}`,
		URL:      `{{ template "pushover.default.url" . }}`,
		Priority: `{{ if eq .Status "firing" }}2{{ else }}0{{ end }}`, // emergency (firing) or normal
		Retry:    duration(1 * time.Minute),
		Expire:   duration(1 * time.Hour),
	}
)

// NotifierConfig contains base options common across all notifier configurations.
type NotifierConfig struct {
	VSendResolved bool `yaml:"send_resolved" json:"send_resolved"`
}

func (nc *NotifierConfig) SendResolved() bool {
	return nc.VSendResolved
}

// EmailConfig configures notifications via mail.
type EmailConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	// Email address to notify.
	To           string            `yaml:"to,omitempty" json:"to,omitempty"`
	From         string            `yaml:"from,omitempty" json:"from,omitempty"`
	Hello        string            `yaml:"hello,omitempty" json:"hello,omitempty"`
	Smarthost    string            `yaml:"smarthost,omitempty" json:"smarthost,omitempty"`
	AuthUsername string            `yaml:"auth_username,omitempty" json:"auth_username,omitempty"`
	AuthPassword Secret            `yaml:"auth_password,omitempty" json:"auth_password,omitempty"`
	AuthSecret   Secret            `yaml:"auth_secret,omitempty" json:"auth_secret,omitempty"`
	AuthIdentity string            `yaml:"auth_identity,omitempty" json:"auth_identity,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	HTML         string            `yaml:"html,omitempty" json:"html,omitempty"`
	Text         string            `yaml:"text,omitempty" json:"text,omitempty"`
	RequireTLS   *bool             `yaml:"require_tls,omitempty" json:"require_tls,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *EmailConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultEmailConfig
	type plain EmailConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.To == "" {
		return fmt.Errorf("missing to address in email config")
	}
	// Header names are case-insensitive, check for collisions.
	normalizedHeaders := map[string]string{}
	for h, v := range c.Headers {
		normalized := strings.Title(h)
		if _, ok := normalizedHeaders[normalized]; ok {
			return fmt.Errorf("duplicate header %q in email config", normalized)
		}
		normalizedHeaders[normalized] = v
	}
	c.Headers = normalizedHeaders

	return checkOverflow(c.XXX, "email config")
}

// PagerdutyConfig configures notifications via PagerDuty.
type PagerdutyConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	HTTPConfig *HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`

	ServiceKey  Secret            `yaml:"service_key,omitempty" json:"service_key,omitempty"`
	URL         string            `yaml:"url,omitempty" json:"url,omitempty"`
	Client      string            `yaml:"client,omitempty" json:"client,omitempty"`
	ClientURL   string            `yaml:"client_url,omitempty" json:"client_url,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Details     map[string]string `yaml:"details,omitempty" json:"details,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *PagerdutyConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultPagerdutyConfig
	type plain PagerdutyConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.ServiceKey == "" {
		return fmt.Errorf("missing service key in PagerDuty config")
	}
	return checkOverflow(c.XXX, "pagerduty config")
}

type WechatConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	HTTPConfig *HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`

	APISecret Secret `yaml:"api_secret,omitempty" json:"api_secret,omitempty"`
	APIURL    string `yaml:"api_url,omitempty" json:"api_url,omitempty"`
	CorpID    string `yaml:"corp_id,omitempty" json:"corp_id,omitempty"`
	Message   string `yaml:"message,omitempty" json:"message,omitempty"`
	AgentID   string `yaml:"agent_id,omitempty" json:"agent_id,omitempty"`
	ToParty   string `yaml:"to_party,omitempty" json:"to_party,omitempty"`
	ToTag     string `yaml:"to_tag,omitempty" json:"to_tag,omitempty"`
	ToUser    string `yaml:"to_user,omitempty" json:"to_user,omitempty"`
	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *WechatConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultWechatConfig
	type plain WechatConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.APISecret == "" {
		return fmt.Errorf("missing api secret in Wechat config")
	}
	if c.APIURL == "" {
		return fmt.Errorf("missing api url in Wechat config")
	}
	if c.CorpID == "" {
		return fmt.Errorf("missing crop id in Wechat config")
	}
	if c.AgentID == "" {
		return fmt.Errorf("missing agent id in Wechat config")
	}
	if c.ToParty == "" && c.ToTag == "" && c.ToUser == "" {
		return fmt.Errorf("missing target id in Wechat config")
	}
	return checkOverflow(c.XXX, "wechat config")
}

// SlackConfig configures notifications via Slack.
type SlackConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	HTTPConfig *HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`

	APIURL Secret `yaml:"api_url,omitempty" json:"api_url,omitempty"`

	// Slack channel override, (like #other-channel or @username).
	Channel  string `yaml:"channel,omitempty" json:"channel,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Color    string `yaml:"color,omitempty" json:"color,omitempty"`

	Title     string `yaml:"title,omitempty" json:"title,omitempty"`
	TitleLink string `yaml:"title_link" json:"title_link"`
	Pretext   string `yaml:"pretext,omitempty" json:"pretext,omitempty"`
	Text      string `yaml:"text,omitempty" json:"text,omitempty"`
	Fallback  string `yaml:"fallback,omitempty" json:"fallback,omitempty"`
	IconEmoji string `yaml:"icon_emoji,omitempty" json:"icon_emoji,omitempty"`
	IconURL   string `yaml:"icon_url,omitempty" json:"icon_url,omitempty"`
	LinkNames bool   `yaml:"link_names,omitempty" json:"link_names,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *SlackConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultSlackConfig
	type plain SlackConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	return checkOverflow(c.XXX, "slack config")
}

// HipchatConfig configures notifications via Hipchat.
type HipchatConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	APIURL        string `yaml:"api_url,omitempty" json:"api_url,omitempty"`
	AuthToken     Secret `yaml:"auth_token,omitempty" json:"auth_token,omitempty"`
	RoomID        string `yaml:"room_id,omitempty" json:"room_id,omitempty"`
	From          string `yaml:"from,omitempty" json:"from,omitempty"`
	Notify        bool   `yaml:"notify,omitempty" json:"notify,omitempty"`
	Message       string `yaml:"message,omitempty" json:"message,omitempty"`
	MessageFormat string `yaml:"message_format,omitempty" json:"message_format,omitempty"`
	Color         string `yaml:"color,omitempty" json:"color,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" ,json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *HipchatConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultHipchatConfig
	type plain HipchatConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.RoomID == "" {
		return fmt.Errorf("missing room id in Hipchat config")
	}

	return checkOverflow(c.XXX, "hipchat config")
}

// WebhookConfig configures notifications via a generic webhook.
type WebhookConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	HTTPConfig *HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`

	// URL to send POST request to.
	URL string `yaml:"url" json:"url"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *WebhookConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultWebhookConfig
	type plain WebhookConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.URL == "" {
		return fmt.Errorf("missing URL in webhook config")
	}
	return checkOverflow(c.XXX, "webhook config")
}

// OpsGenieConfig configures notifications via OpsGenie.
type OpsGenieConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	HTTPConfig *HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`

	APIKey      Secret            `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	APIHost     string            `yaml:"api_host,omitempty" json:"api_host,omitempty"`
	Message     string            `yaml:"message,omitempty" json:"message,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Source      string            `yaml:"source,omitempty" json:"source,omitempty"`
	Details     map[string]string `yaml:"details,omitempty" json:"details,omitempty"`
	Teams       string            `yaml:"teams,omitempty" json:"teams,omitempty"`
	Tags        string            `yaml:"tags,omitempty" json:"tags,omitempty"`
	Note        string            `yaml:"note,omitempty" json:"note,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *OpsGenieConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultOpsGenieConfig
	type plain OpsGenieConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.APIKey == "" {
		return fmt.Errorf("missing API key in OpsGenie config")
	}
	return checkOverflow(c.XXX, "opsgenie config")
}

// VictorOpsConfig configures notifications via VictorOps.
type VictorOpsConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	HTTPConfig *HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`

	APIKey            Secret `yaml:"api_key" json:"api_key"`
	APIURL            string `yaml:"api_url" json:"api_url"`
	RoutingKey        string `yaml:"routing_key" json:"routing_key"`
	MessageType       string `yaml:"message_type" json:"message_type"`
	StateMessage      string `yaml:"state_message" json:"state_message"`
	EntityDisplayName string `yaml:"entity_display_name" json:"entity_display_name"`
	MonitoringTool    string `yaml:"monitoring_tool" json:"monitoring_tool"`

	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *VictorOpsConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultVictorOpsConfig
	type plain VictorOpsConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.RoutingKey == "" {
		return fmt.Errorf("missing Routing key in VictorOps config")
	}
	return checkOverflow(c.XXX, "victorops config")
}

type duration time.Duration

func (d *duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err == nil {
		*d = duration(parsed)
	}
	return err
}

func (d duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

type PushoverConfig struct {
	NotifierConfig `yaml:",inline" json:",inline"`

	HTTPConfig *HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`

	UserKey  Secret   `yaml:"user_key,omitempty" json:"user_key,omitempty"`
	Token    Secret   `yaml:"token,omitempty" json:"token,omitempty"`
	Title    string   `yaml:"title,omitempty" json:"title,omitempty"`
	Message  string   `yaml:"message,omitempty" json:"message,omitempty"`
	URL      string   `yaml:"url,omitempty" json:"url,omitempty"`
	Priority string   `yaml:"priority,omitempty" json:"priority,omitempty"`
	Retry    duration `yaml:"retry,omitempty" json:"retry,omitempty"`
	Expire   duration `yaml:"expire,omitempty" json:"expire,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *PushoverConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultPushoverConfig
	type plain PushoverConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.UserKey == "" {
		return fmt.Errorf("missing user key in Pushover config")
	}
	if c.Token == "" {
		return fmt.Errorf("missing token in Pushover config")
	}
	return checkOverflow(c.XXX, "pushover config")
}

// HTTPClientConfig configures an HTTP client.
type HTTPClientConfig struct {
	// The HTTP basic authentication credentials for the targets.
	BasicAuth *BasicAuth `yaml:"basic_auth,omitempty"`
	// The bearer token for the targets.
	BearerToken Secret `yaml:"bearer_token,omitempty"`
	// The bearer token file for the targets.
	BearerTokenFile string `yaml:"bearer_token_file,omitempty"`
	// HTTP proxy server to use to connect to the targets.
	ProxyURL URL `yaml:"proxy_url,omitempty"`
	// TLSConfig to use to connect to the targets.
	TLSConfig TLSConfig `yaml:"tls_config,omitempty"`
}

// BasicAuth contains basic HTTP authentication credentials.
type BasicAuth struct {
	Username     string `yaml:"username"`
	Password     Secret `yaml:"password,omitempty"`
	PasswordFile string `yaml:"password_file,omitempty"`
}

// TLSConfig configures the options for TLS connections.
type TLSConfig struct {
	// The CA cert to use for the targets.
	CAFile string `yaml:"ca_file,omitempty"`
	// The client cert file for the targets.
	CertFile string `yaml:"cert_file,omitempty"`
	// The client key file for the targets.
	KeyFile string `yaml:"key_file,omitempty"`
	// Used to verify the hostname for the targets.
	ServerName string `yaml:"server_name,omitempty"`
	// Disable target certificate validation.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// URL is a custom URL type that allows validation at configuration load time.
type URL struct {
	*url.URL
}

// Copy makes a deep-copy of the struct.
func (u *URL) Copy() *URL {
	v := *u.URL
	return &URL{&v}
}

// MarshalYAML implements the yaml.Marshaler interface for URL.
func (u URL) MarshalYAML() (interface{}, error) {
	if u.URL != nil {
		return u.URL.String(), nil
	}
	return nil, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for URL.
func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	urlp, err := parseURL(s)
	if err != nil {
		return err
	}
	u.URL = urlp.URL
	return nil
}

// MarshalJSON implements the json.Marshaler interface for URL.
func (u URL) MarshalJSON() ([]byte, error) {
	if u.URL != nil {
		return json.Marshal(u.URL.String())
	}
	return nil, nil
}

// UnmarshalJSON implements the json.Marshaler interface for URL.
func (u *URL) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	urlp, err := parseURL(s)
	if err != nil {
		return err
	}
	u.URL = urlp.URL
	return nil
}

func parseURL(s string) (*URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q for URL", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("missing host for URL")
	}
	return &URL{u}, nil
}
