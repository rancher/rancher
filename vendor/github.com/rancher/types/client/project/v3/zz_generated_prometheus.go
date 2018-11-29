package client

import (
	"github.com/rancher/norman/types"
)

const (
	PrometheusType                               = "prometheus"
	PrometheusFieldAdditionalAlertManagerConfigs = "additionalAlertManagerConfigs"
	PrometheusFieldAdditionalAlertRelabelConfigs = "additionalAlertRelabelConfigs"
	PrometheusFieldAdditionalScrapeConfigs       = "additionalScrapeConfigs"
	PrometheusFieldAffinity                      = "affinity"
	PrometheusFieldAlerting                      = "alerting"
	PrometheusFieldAnnotations                   = "annotations"
	PrometheusFieldBaseImage                     = "baseImage"
	PrometheusFieldConfigMaps                    = "configMaps"
	PrometheusFieldContainers                    = "containers"
	PrometheusFieldCreated                       = "created"
	PrometheusFieldCreatorID                     = "creatorId"
	PrometheusFieldDescription                   = "description"
	PrometheusFieldEvaluationInterval            = "evaluationInterval"
	PrometheusFieldExternalLabels                = "externalLabels"
	PrometheusFieldExternalURL                   = "externalUrl"
	PrometheusFieldImagePullSecrets              = "imagePullSecrets"
	PrometheusFieldLabels                        = "labels"
	PrometheusFieldListenLocal                   = "listenLocal"
	PrometheusFieldLogLevel                      = "logLevel"
	PrometheusFieldName                          = "name"
	PrometheusFieldNamespaceId                   = "namespaceId"
	PrometheusFieldNodeSelector                  = "nodeSelector"
	PrometheusFieldOwnerReferences               = "ownerReferences"
	PrometheusFieldPodMetadata                   = "podMetadata"
	PrometheusFieldPriorityClassName             = "priorityClassName"
	PrometheusFieldProjectID                     = "projectId"
	PrometheusFieldRemoteRead                    = "remoteRead"
	PrometheusFieldRemoteWrite                   = "remoteWrite"
	PrometheusFieldRemoved                       = "removed"
	PrometheusFieldReplicas                      = "replicas"
	PrometheusFieldResources                     = "resources"
	PrometheusFieldRetention                     = "retention"
	PrometheusFieldRoutePrefix                   = "routePrefix"
	PrometheusFieldRuleSelector                  = "ruleSelector"
	PrometheusFieldSHA                           = "sha"
	PrometheusFieldScrapeInterval                = "scrapeInterval"
	PrometheusFieldSecrets                       = "secrets"
	PrometheusFieldSecurityContext               = "securityContext"
	PrometheusFieldServiceAccountName            = "serviceAccountName"
	PrometheusFieldServiceMonitorSelector        = "serviceMonitorSelector"
	PrometheusFieldState                         = "state"
	PrometheusFieldStorage                       = "storage"
	PrometheusFieldTag                           = "tag"
	PrometheusFieldTolerations                   = "tolerations"
	PrometheusFieldTransitioning                 = "transitioning"
	PrometheusFieldTransitioningMessage          = "transitioningMessage"
	PrometheusFieldUUID                          = "uuid"
	PrometheusFieldVersion                       = "version"
)

type Prometheus struct {
	types.Resource
	AdditionalAlertManagerConfigs *SecretKeySelector     `json:"additionalAlertManagerConfigs,omitempty" yaml:"additionalAlertManagerConfigs,omitempty"`
	AdditionalAlertRelabelConfigs *SecretKeySelector     `json:"additionalAlertRelabelConfigs,omitempty" yaml:"additionalAlertRelabelConfigs,omitempty"`
	AdditionalScrapeConfigs       *SecretKeySelector     `json:"additionalScrapeConfigs,omitempty" yaml:"additionalScrapeConfigs,omitempty"`
	Affinity                      *Affinity              `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Alerting                      *AlertingSpec          `json:"alerting,omitempty" yaml:"alerting,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	BaseImage                     string                 `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	ConfigMaps                    []string               `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Containers                    []Container            `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description                   string                 `json:"description,omitempty" yaml:"description,omitempty"`
	EvaluationInterval            string                 `json:"evaluationInterval,omitempty" yaml:"evaluationInterval,omitempty"`
	ExternalLabels                map[string]string      `json:"externalLabels,omitempty" yaml:"externalLabels,omitempty"`
	ExternalURL                   string                 `json:"externalUrl,omitempty" yaml:"externalUrl,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	Labels                        map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	ListenLocal                   bool                   `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	LogLevel                      string                 `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	Name                          string                 `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                   string                 `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeSelector                  map[string]string      `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	OwnerReferences               []OwnerReference       `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodMetadata                   *ObjectMeta            `json:"podMetadata,omitempty" yaml:"podMetadata,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	ProjectID                     string                 `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	RemoteRead                    []RemoteReadSpec       `json:"remoteRead,omitempty" yaml:"remoteRead,omitempty"`
	RemoteWrite                   []RemoteWriteSpec      `json:"remoteWrite,omitempty" yaml:"remoteWrite,omitempty"`
	Removed                       string                 `json:"removed,omitempty" yaml:"removed,omitempty"`
	Replicas                      *int64                 `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Resources                     *ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Retention                     string                 `json:"retention,omitempty" yaml:"retention,omitempty"`
	RoutePrefix                   string                 `json:"routePrefix,omitempty" yaml:"routePrefix,omitempty"`
	RuleSelector                  *LabelSelector         `json:"ruleSelector,omitempty" yaml:"ruleSelector,omitempty"`
	SHA                           string                 `json:"sha,omitempty" yaml:"sha,omitempty"`
	ScrapeInterval                string                 `json:"scrapeInterval,omitempty" yaml:"scrapeInterval,omitempty"`
	Secrets                       []string               `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecurityContext               *PodSecurityContext    `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	ServiceMonitorSelector        *LabelSelector         `json:"serviceMonitorSelector,omitempty" yaml:"serviceMonitorSelector,omitempty"`
	State                         string                 `json:"state,omitempty" yaml:"state,omitempty"`
	Storage                       *StorageSpec           `json:"storage,omitempty" yaml:"storage,omitempty"`
	Tag                           string                 `json:"tag,omitempty" yaml:"tag,omitempty"`
	Tolerations                   []Toleration           `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Transitioning                 string                 `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string                 `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                          string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Version                       string                 `json:"version,omitempty" yaml:"version,omitempty"`
}

type PrometheusCollection struct {
	types.Collection
	Data   []Prometheus `json:"data,omitempty"`
	client *PrometheusClient
}

type PrometheusClient struct {
	apiClient *Client
}

type PrometheusOperations interface {
	List(opts *types.ListOpts) (*PrometheusCollection, error)
	Create(opts *Prometheus) (*Prometheus, error)
	Update(existing *Prometheus, updates interface{}) (*Prometheus, error)
	Replace(existing *Prometheus) (*Prometheus, error)
	ByID(id string) (*Prometheus, error)
	Delete(container *Prometheus) error
}

func newPrometheusClient(apiClient *Client) *PrometheusClient {
	return &PrometheusClient{
		apiClient: apiClient,
	}
}

func (c *PrometheusClient) Create(container *Prometheus) (*Prometheus, error) {
	resp := &Prometheus{}
	err := c.apiClient.Ops.DoCreate(PrometheusType, container, resp)
	return resp, err
}

func (c *PrometheusClient) Update(existing *Prometheus, updates interface{}) (*Prometheus, error) {
	resp := &Prometheus{}
	err := c.apiClient.Ops.DoUpdate(PrometheusType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PrometheusClient) Replace(obj *Prometheus) (*Prometheus, error) {
	resp := &Prometheus{}
	err := c.apiClient.Ops.DoReplace(PrometheusType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PrometheusClient) List(opts *types.ListOpts) (*PrometheusCollection, error) {
	resp := &PrometheusCollection{}
	err := c.apiClient.Ops.DoList(PrometheusType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PrometheusCollection) Next() (*PrometheusCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PrometheusCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PrometheusClient) ByID(id string) (*Prometheus, error) {
	resp := &Prometheus{}
	err := c.apiClient.Ops.DoByID(PrometheusType, id, resp)
	return resp, err
}

func (c *PrometheusClient) Delete(container *Prometheus) error {
	return c.apiClient.Ops.DoResourceDelete(PrometheusType, &container.Resource)
}
