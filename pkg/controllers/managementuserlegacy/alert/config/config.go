package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
)

// Secret is a string that must not be revealed on marshaling.
type Secret string

// Load parses the YAML input s into a Config.
func Load(s string) (*Config, error) {
	cfg := &Config{}
	err := yaml.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	// Check if we have a root route. We cannot check for it in the
	// UnmarshalYAML method because it won't be called if the input is empty
	// (e.g. the config file is empty or only contains whitespace).
	if cfg.Route == nil {
		return nil, errors.New("no route provided in config")
	}

	// Check if continue in root route.
	if cfg.Route.Continue {
		return nil, errors.New("cannot have continue in root route")
	}

	cfg.original = s
	return cfg, nil
}

// LoadFile parses the given YAML file into a Config.
func LoadFile(filename string) (*Config, []byte, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := Load(string(content))
	if err != nil {
		return nil, nil, err
	}

	resolveFilepaths(filepath.Dir(filename), cfg)
	return cfg, content, nil
}

// resolveFilepaths joins all relative paths in a configuration
// with a given base directory.
func resolveFilepaths(baseDir string, cfg *Config) {
	join := func(fp string) string {
		if len(fp) > 0 && !filepath.IsAbs(fp) {
			fp = filepath.Join(baseDir, fp)
		}
		return fp
	}

	for i, tf := range cfg.Templates {
		cfg.Templates[i] = join(tf)
	}
}

// Config is the top-level configuration for Alertmanager's config files.
type Config struct {
	Global       *GlobalConfig  `yaml:"global,omitempty" json:"global,omitempty"`
	Route        *Route         `yaml:"route,omitempty" json:"route,omitempty"`
	InhibitRules []*InhibitRule `yaml:"inhibit_rules,omitempty" json:"inhibit_rules,omitempty"`
	Receivers    []*Receiver    `yaml:"receivers,omitempty" json:"receivers,omitempty"`
	Templates    []string       `yaml:"templates" json:"templates"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`

	// original is the input from which the config was parsed.
	original string
}

func checkOverflow(m map[string]interface{}, ctx string) error {
	if len(m) > 0 {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Errorf("unknown fields in %s: %s", ctx, strings.Join(keys, ", "))
	}
	return nil
}

func (c Config) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// We want to set c to the defaults and then overwrite it with the input.
	// To make unmarshal fill the plain data struct rather than calling UnmarshalYAML
	// again, we have to hide it using a type indirection.
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	// If a global block was open but empty the default global config is overwritten.
	// We have to restore it here.
	if c.Global == nil {
		c.Global = &GlobalConfig{}
		*c.Global = DefaultGlobalConfig
	}

	names := map[string]struct{}{}

	for _, rcv := range c.Receivers {
		if _, ok := names[rcv.Name]; ok {
			return fmt.Errorf("notification config name %q is not unique", rcv.Name)
		}
		for _, ec := range rcv.EmailConfigs {
			if ec.Smarthost == "" {
				if c.Global.SMTPSmarthost == "" {
					return fmt.Errorf("no global SMTP smarthost set")
				}
				ec.Smarthost = c.Global.SMTPSmarthost
			}
			if ec.From == "" {
				if c.Global.SMTPFrom == "" {
					return fmt.Errorf("no global SMTP from set")
				}
				ec.From = c.Global.SMTPFrom
			}
			if ec.Hello == "" {
				ec.Hello = c.Global.SMTPHello
			}
			if ec.AuthUsername == "" {
				ec.AuthUsername = c.Global.SMTPAuthUsername
			}
			if ec.AuthPassword == "" {
				ec.AuthPassword = c.Global.SMTPAuthPassword
			}
			if ec.AuthSecret == "" {
				ec.AuthSecret = c.Global.SMTPAuthSecret
			}
			if ec.AuthIdentity == "" {
				ec.AuthIdentity = c.Global.SMTPAuthIdentity
			}
			if ec.RequireTLS == nil {
				ec.RequireTLS = new(bool)
				*ec.RequireTLS = c.Global.SMTPRequireTLS
			}
		}
		/*
			for _, sc := range rcv.SlackConfigs {
				if sc.APIURL == "" {
					if c.Global.SlackAPIURL == "" {
						return fmt.Errorf("no global Slack API URL set")
					}
					sc.APIURL = c.Global.SlackAPIURL
				}
			}
		*/
		for _, hc := range rcv.HipchatConfigs {
			if hc.APIURL == "" {
				if c.Global.HipchatURL == "" {
					return fmt.Errorf("no global Hipchat API URL set")
				}
				hc.APIURL = c.Global.HipchatURL
			}
			if !strings.HasSuffix(hc.APIURL, "/") {
				hc.APIURL += "/"
			}
			if hc.AuthToken == "" {
				if c.Global.HipchatAuthToken == "" {
					return fmt.Errorf("no global Hipchat Auth Token set")
				}
				hc.AuthToken = c.Global.HipchatAuthToken
			}
		}
		for _, pdc := range rcv.PagerdutyConfigs {
			if pdc.URL == "" {
				if c.Global.PagerdutyURL == "" {
					return fmt.Errorf("no global PagerDuty URL set")
				}
				pdc.URL = c.Global.PagerdutyURL
			}
		}
		for _, wcc := range rcv.WechatConfigs {
			if wcc.APIURL == "" {
				if c.Global.WechatURL == "" {
					return fmt.Errorf("no global Wechat URL set")
				}
				wcc.APIURL = c.Global.WechatURL
			}
		}
		for _, ogc := range rcv.OpsGenieConfigs {
			if ogc.APIHost == "" {
				if c.Global.OpsGenieAPIHost == "" {
					return fmt.Errorf("no global OpsGenie URL set")
				}
				ogc.APIHost = c.Global.OpsGenieAPIHost
			}
			if !strings.HasSuffix(ogc.APIHost, "/") {
				ogc.APIHost += "/"
			}
		}
		for _, voc := range rcv.VictorOpsConfigs {
			if voc.APIURL == "" {
				if c.Global.VictorOpsAPIURL == "" {
					return fmt.Errorf("no global VictorOps URL set")
				}
				voc.APIURL = c.Global.VictorOpsAPIURL
			}
			if !strings.HasSuffix(voc.APIURL, "/") {
				voc.APIURL += "/"
			}
			if voc.APIKey == "" {
				if c.Global.VictorOpsAPIKey == "" {
					return fmt.Errorf("no global VictorOps API Key set")
				}
				voc.APIKey = c.Global.VictorOpsAPIKey
			}
		}
		names[rcv.Name] = struct{}{}
	}

	// The root route must not have any matchers as it is the fallback node
	// for all alerts.
	if c.Route == nil {
		return fmt.Errorf("No routes provided")
	}
	if len(c.Route.Receiver) == 0 {
		return fmt.Errorf("Root route must specify a default receiver")
	}
	if len(c.Route.Match) > 0 || len(c.Route.MatchRE) > 0 {
		return fmt.Errorf("Root route must not have any matchers")
	}

	// Validate that all receivers used in the routing tree are defined.
	if err := checkReceiver(c.Route, names); err != nil {
		return err
	}

	return checkOverflow(c.XXX, "config")
}

// checkReceiver returns an error if a node in the routing tree
// references a receiver not in the given map.
func checkReceiver(r *Route, receivers map[string]struct{}) error {
	if r.Receiver == "" {
		return nil
	}
	if _, ok := receivers[r.Receiver]; !ok {
		return fmt.Errorf("Undefined receiver %q used in route", r.Receiver)
	}
	for _, sr := range r.Routes {
		if err := checkReceiver(sr, receivers); err != nil {
			return err
		}
	}
	return nil
}

// DefaultGlobalConfig provides global default values.
var DefaultGlobalConfig = GlobalConfig{
	ResolveTimeout: model.Duration(5 * time.Minute),

	SMTPRequireTLS:  true,
	PagerdutyURL:    "https://events.pagerduty.com/v2/enqueue",
	WechatURL:       "https://qyapi.weixin.qq.com/cgi-bin/",
	HipchatURL:      "https://api.hipchat.com/",
	OpsGenieAPIHost: "https://api.opsgenie.com/",
	VictorOpsAPIURL: "https://alert.victorops.com/integrations/generic/20131114/alert/",
}

// GlobalConfig defines configuration parameters that are valid globally
// unless overwritten.
type GlobalConfig struct {
	// ResolveTimeout is the time after which an alert is declared resolved
	// if it has not been updated.
	ResolveTimeout model.Duration `yaml:"resolve_timeout" json:"resolve_timeout"`

	SMTPFrom         string `yaml:"smtp_from,omitempty" json:"smtp_from,omitempty"`
	SMTPHello        string `yaml:"smtp_hello,omitempty" json:"smtp_hello,omitempty"`
	SMTPSmarthost    string `yaml:"smtp_smarthost,omitempty" json:"smtp_smarthost,omitempty"`
	SMTPAuthUsername string `yaml:"smtp_auth_username,omitempty" json:"smtp_auth_username,omitempty"`
	SMTPAuthPassword Secret `yaml:"smtp_auth_password,omitempty" json:"smtp_auth_password,omitempty"`
	SMTPAuthSecret   Secret `yaml:"smtp_auth_secret,omitempty" json:"smtp_auth_secret,omitempty"`
	SMTPAuthIdentity string `yaml:"smtp_auth_identity,omitempty" json:"smtp_auth_identity,omitempty"`
	SMTPRequireTLS   bool   `yaml:"smtp_require_tls" json:"smtp_require_tls,omitempty"`
	SlackAPIURL      Secret `yaml:"slack_api_url,omitempty" json:"slack_api_url,omitempty"`
	PagerdutyURL     string `yaml:"pagerduty_url,omitempty" json:"pagerduty_url,omitempty"`
	HipchatURL       string `yaml:"hipchat_url,omitempty" json:"hipchat_url,omitempty"`
	HipchatAuthToken Secret `yaml:"hipchat_auth_token,omitempty" json:"hipchat_auth_token,omitempty"`
	OpsGenieAPIHost  string `yaml:"opsgenie_api_host,omitempty" json:"opsgenie_api_host,omitempty"`
	VictorOpsAPIURL  string `yaml:"victorops_api_url,omitempty" json:"victorops_api_url,omitempty"`
	VictorOpsAPIKey  Secret `yaml:"victorops_api_key,omitempty" json:"victorops_api_key,omitempty"`
	WechatURL        string `yaml:"wechat_url,omitempty" json:"wechat_url,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *GlobalConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultGlobalConfig
	type plain GlobalConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	return checkOverflow(c.XXX, "global")
}

// A Route is a node that contains definitions of how to handle alerts.
type Route struct {
	Receiver string            `yaml:"receiver,omitempty" json:"receiver,omitempty"`
	GroupBy  []model.LabelName `yaml:"group_by,omitempty" json:"group_by,omitempty"`

	Match    map[string]string `yaml:"match,omitempty" json:"match,omitempty"`
	MatchRE  map[string]Regexp `yaml:"match_re,omitempty" json:"match_re,omitempty"`
	Continue bool              `yaml:"continue,omitempty" json:"continue,omitempty"`
	Routes   []*Route          `yaml:"routes,omitempty" json:"routes,omitempty"`

	GroupWait      *model.Duration `yaml:"group_wait,omitempty" json:"group_wait,omitempty"`
	GroupInterval  *model.Duration `yaml:"group_interval,omitempty" json:"group_interval,omitempty"`
	RepeatInterval *model.Duration `yaml:"repeat_interval,omitempty" json:"repeat_interval,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (r *Route) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Route
	if err := unmarshal((*plain)(r)); err != nil {
		return err
	}

	for k := range r.Match {
		if !model.LabelNameRE.MatchString(k) {
			return fmt.Errorf("invalid label name %q", k)
		}
	}

	for k := range r.MatchRE {
		if !model.LabelNameRE.MatchString(k) {
			return fmt.Errorf("invalid label name %q", k)
		}
	}

	groupBy := map[model.LabelName]struct{}{}

	for _, ln := range r.GroupBy {
		if _, ok := groupBy[ln]; ok {
			return fmt.Errorf("duplicated label %q in group_by", ln)
		}
		groupBy[ln] = struct{}{}
	}

	return checkOverflow(r.XXX, "route")
}

// InhibitRule defines an inhibition rule that mutes alerts that match the
// target labels if an alert matching the source labels exists.
// Both alerts have to have a set of labels being equal.
type InhibitRule struct {
	// SourceMatch defines a set of labels that have to equal the given
	// value for source alerts.
	SourceMatch map[string]string `yaml:"source_match,omitempty" json:"source_match,omitempty"`
	// SourceMatchRE defines pairs like SourceMatch but does regular expression
	// matching.
	SourceMatchRE map[string]Regexp `yaml:"source_match_re,omitempty" json:"source_match_re,omitempty"`
	// TargetMatch defines a set of labels that have to equal the given
	// value for target alerts.
	TargetMatch map[string]string `yaml:"target_match,omitempty" json:"target_match,omitempty"`
	// TargetMatchRE defines pairs like TargetMatch but does regular expression
	// matching.
	TargetMatchRE map[string]Regexp `yaml:"target_match_re,omitempty" json:"target_match_re,omitempty"`
	// A set of labels that must be equal between the source and target alert
	// for them to be a match.
	Equal model.LabelNames `yaml:"equal,omitempty" json:"equal,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (r *InhibitRule) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain InhibitRule
	if err := unmarshal((*plain)(r)); err != nil {
		return err
	}

	for k := range r.SourceMatch {
		if !model.LabelNameRE.MatchString(k) {
			return fmt.Errorf("invalid label name %q", k)
		}
	}

	for k := range r.SourceMatchRE {
		if !model.LabelNameRE.MatchString(k) {
			return fmt.Errorf("invalid label name %q", k)
		}
	}

	for k := range r.TargetMatch {
		if !model.LabelNameRE.MatchString(k) {
			return fmt.Errorf("invalid label name %q", k)
		}
	}

	for k := range r.TargetMatchRE {
		if !model.LabelNameRE.MatchString(k) {
			return fmt.Errorf("invalid label name %q", k)
		}
	}

	return checkOverflow(r.XXX, "inhibit rule")
}

// Receiver configuration provides configuration on how to contact a receiver.
type Receiver struct {
	// A unique identifier for this receiver.
	Name string `yaml:"name" json:"name"`

	EmailConfigs     []*EmailConfig     `yaml:"email_configs,omitempty" json:"email_configs,omitempty"`
	PagerdutyConfigs []*PagerdutyConfig `yaml:"pagerduty_configs,omitempty" json:"pagerduty_configs,omitempty"`
	HipchatConfigs   []*HipchatConfig   `yaml:"hipchat_configs,omitempty" json:"hipchat_configs,omitempty"`
	SlackConfigs     []*SlackConfig     `yaml:"slack_configs,omitempty" json:"slack_configs,omitempty"`
	WebhookConfigs   []*WebhookConfig   `yaml:"webhook_configs,omitempty" json:"webhook_configs,omitempty"`
	OpsGenieConfigs  []*OpsGenieConfig  `yaml:"opsgenie_configs,omitempty" json:"opsgenie_configs,omitempty"`
	PushoverConfigs  []*PushoverConfig  `yaml:"pushover_configs,omitempty" json:"pushover_configs,omitempty"`
	VictorOpsConfigs []*VictorOpsConfig `yaml:"victorops_configs,omitempty" json:"victorops_configs,omitempty"`
	WechatConfigs    []*WechatConfig    `yaml:"wechat_configs,omitempty" json:"wechat_configs,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *Receiver) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Receiver
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.Name == "" {
		return fmt.Errorf("missing name in receiver")
	}
	return checkOverflow(c.XXX, "receiver config")
}

// Regexp encapsulates a regexp.Regexp and makes it YAML marshalable.
type Regexp struct {
	*regexp.Regexp
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (re *Regexp) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	regex, err := regexp.Compile("^(?:" + s + ")$")
	if err != nil {
		return err
	}
	re.Regexp = regex
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (re Regexp) MarshalYAML() (interface{}, error) {
	if re.Regexp != nil {
		return re.String(), nil
	}
	return nil, nil
}

// UnmarshalJSON implements the json.Marshaler interface
func (re *Regexp) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	regex, err := regexp.Compile("^(?:" + s + ")$")
	if err != nil {
		return err
	}
	re.Regexp = regex
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (re Regexp) MarshalJSON() ([]byte, error) {
	if re.Regexp != nil {
		return json.Marshal(re.String())
	}
	return nil, nil
}
