package client

import (
	"github.com/rancher/norman/types"
)

const (
	NotifierType                          = "notifier"
	NotifierFieldAnnotations              = "annotations"
	NotifierFieldClusterID                = "clusterId"
	NotifierFieldCreated                  = "created"
	NotifierFieldCreatorID                = "creatorId"
	NotifierFieldDescription              = "description"
	NotifierFieldDingtalkConfig           = "dingtalkConfig"
	NotifierFieldDingtalkCredentialSecret = "dingtalkCredentialSecret"
	NotifierFieldLabels                   = "labels"
	NotifierFieldMSTeamsConfig            = "msteamsConfig"
	NotifierFieldName                     = "name"
	NotifierFieldNamespaceId              = "namespaceId"
	NotifierFieldOwnerReferences          = "ownerReferences"
	NotifierFieldPagerdutyConfig          = "pagerdutyConfig"
	NotifierFieldRemoved                  = "removed"
	NotifierFieldSMTPConfig               = "smtpConfig"
	NotifierFieldSMTPCredentialSecret     = "smtpCredentialSecret"
	NotifierFieldSendResolved             = "sendResolved"
	NotifierFieldSlackConfig              = "slackConfig"
	NotifierFieldState                    = "state"
	NotifierFieldTransitioning            = "transitioning"
	NotifierFieldTransitioningMessage     = "transitioningMessage"
	NotifierFieldUUID                     = "uuid"
	NotifierFieldWebhookConfig            = "webhookConfig"
	NotifierFieldWechatConfig             = "wechatConfig"
	NotifierFieldWechatCredentialSecret   = "wechatCredentialSecret"
)

type Notifier struct {
	types.Resource
	Annotations              map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterID                string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created                  string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description              string            `json:"description,omitempty" yaml:"description,omitempty"`
	DingtalkConfig           *DingtalkConfig   `json:"dingtalkConfig,omitempty" yaml:"dingtalkConfig,omitempty"`
	DingtalkCredentialSecret string            `json:"dingtalkCredentialSecret,omitempty" yaml:"dingtalkCredentialSecret,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	MSTeamsConfig            *MSTeamsConfig    `json:"msteamsConfig,omitempty" yaml:"msteamsConfig,omitempty"`
	Name                     string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId              string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences          []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PagerdutyConfig          *PagerdutyConfig  `json:"pagerdutyConfig,omitempty" yaml:"pagerdutyConfig,omitempty"`
	Removed                  string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SMTPConfig               *SMTPConfig       `json:"smtpConfig,omitempty" yaml:"smtpConfig,omitempty"`
	SMTPCredentialSecret     string            `json:"smtpCredentialSecret,omitempty" yaml:"smtpCredentialSecret,omitempty"`
	SendResolved             bool              `json:"sendResolved,omitempty" yaml:"sendResolved,omitempty"`
	SlackConfig              *SlackConfig      `json:"slackConfig,omitempty" yaml:"slackConfig,omitempty"`
	State                    string            `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning            string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage     string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                     string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	WebhookConfig            *WebhookConfig    `json:"webhookConfig,omitempty" yaml:"webhookConfig,omitempty"`
	WechatConfig             *WechatConfig     `json:"wechatConfig,omitempty" yaml:"wechatConfig,omitempty"`
	WechatCredentialSecret   string            `json:"wechatCredentialSecret,omitempty" yaml:"wechatCredentialSecret,omitempty"`
}

type NotifierCollection struct {
	types.Collection
	Data   []Notifier `json:"data,omitempty"`
	client *NotifierClient
}

type NotifierClient struct {
	apiClient *Client
}

type NotifierOperations interface {
	List(opts *types.ListOpts) (*NotifierCollection, error)
	ListAll(opts *types.ListOpts) (*NotifierCollection, error)
	Create(opts *Notifier) (*Notifier, error)
	Update(existing *Notifier, updates interface{}) (*Notifier, error)
	Replace(existing *Notifier) (*Notifier, error)
	ByID(id string) (*Notifier, error)
	Delete(container *Notifier) error

	ActionSend(resource *Notifier, input *Notification) error

	CollectionActionSend(resource *NotifierCollection, input *Notification) error
}

func newNotifierClient(apiClient *Client) *NotifierClient {
	return &NotifierClient{
		apiClient: apiClient,
	}
}

func (c *NotifierClient) Create(container *Notifier) (*Notifier, error) {
	resp := &Notifier{}
	err := c.apiClient.Ops.DoCreate(NotifierType, container, resp)
	return resp, err
}

func (c *NotifierClient) Update(existing *Notifier, updates interface{}) (*Notifier, error) {
	resp := &Notifier{}
	err := c.apiClient.Ops.DoUpdate(NotifierType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NotifierClient) Replace(obj *Notifier) (*Notifier, error) {
	resp := &Notifier{}
	err := c.apiClient.Ops.DoReplace(NotifierType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NotifierClient) List(opts *types.ListOpts) (*NotifierCollection, error) {
	resp := &NotifierCollection{}
	err := c.apiClient.Ops.DoList(NotifierType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *NotifierClient) ListAll(opts *types.ListOpts) (*NotifierCollection, error) {
	resp := &NotifierCollection{}
	resp, err := c.List(opts)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (cc *NotifierCollection) Next() (*NotifierCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NotifierCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NotifierClient) ByID(id string) (*Notifier, error) {
	resp := &Notifier{}
	err := c.apiClient.Ops.DoByID(NotifierType, id, resp)
	return resp, err
}

func (c *NotifierClient) Delete(container *Notifier) error {
	return c.apiClient.Ops.DoResourceDelete(NotifierType, &container.Resource)
}

func (c *NotifierClient) ActionSend(resource *Notifier, input *Notification) error {
	err := c.apiClient.Ops.DoAction(NotifierType, "send", &resource.Resource, input, nil)
	return err
}

func (c *NotifierClient) CollectionActionSend(resource *NotifierCollection, input *Notification) error {
	err := c.apiClient.Ops.DoCollectionAction(NotifierType, "send", &resource.Collection, input, nil)
	return err
}
