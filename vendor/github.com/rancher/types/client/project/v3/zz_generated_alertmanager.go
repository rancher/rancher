package client

import (
	"github.com/rancher/norman/types"
)

const (
	AlertmanagerType                      = "alertmanager"
	AlertmanagerFieldAdditionalPeers      = "additionalPeers"
	AlertmanagerFieldAffinity             = "affinity"
	AlertmanagerFieldAnnotations          = "annotations"
	AlertmanagerFieldBaseImage            = "baseImage"
	AlertmanagerFieldConfigMaps           = "configMaps"
	AlertmanagerFieldContainers           = "containers"
	AlertmanagerFieldCreated              = "created"
	AlertmanagerFieldCreatorID            = "creatorId"
	AlertmanagerFieldExternalURL          = "externalUrl"
	AlertmanagerFieldImage                = "image"
	AlertmanagerFieldImagePullSecrets     = "imagePullSecrets"
	AlertmanagerFieldInitContainers       = "initContainers"
	AlertmanagerFieldLabels               = "labels"
	AlertmanagerFieldListenLocal          = "listenLocal"
	AlertmanagerFieldLogFormat            = "logFormat"
	AlertmanagerFieldLogLevel             = "logLevel"
	AlertmanagerFieldName                 = "name"
	AlertmanagerFieldNamespaceId          = "namespaceId"
	AlertmanagerFieldNodeSelector         = "nodeSelector"
	AlertmanagerFieldOwnerReferences      = "ownerReferences"
	AlertmanagerFieldPaused               = "paused"
	AlertmanagerFieldPodMetadata          = "podMetadata"
	AlertmanagerFieldPortName             = "portName"
	AlertmanagerFieldPriorityClassName    = "priorityClassName"
	AlertmanagerFieldProjectID            = "projectId"
	AlertmanagerFieldRemoved              = "removed"
	AlertmanagerFieldReplicas             = "replicas"
	AlertmanagerFieldResources            = "resources"
	AlertmanagerFieldRetention            = "retention"
	AlertmanagerFieldRoutePrefix          = "routePrefix"
	AlertmanagerFieldSHA                  = "sha"
	AlertmanagerFieldSecrets              = "secrets"
	AlertmanagerFieldSecurityContext      = "securityContext"
	AlertmanagerFieldServiceAccountName   = "serviceAccountName"
	AlertmanagerFieldState                = "state"
	AlertmanagerFieldStorage              = "storage"
	AlertmanagerFieldTag                  = "tag"
	AlertmanagerFieldTolerations          = "tolerations"
	AlertmanagerFieldTransitioning        = "transitioning"
	AlertmanagerFieldTransitioningMessage = "transitioningMessage"
	AlertmanagerFieldUUID                 = "uuid"
	AlertmanagerFieldVersion              = "version"
	AlertmanagerFieldVolumeMounts         = "volumeMounts"
	AlertmanagerFieldVolumes              = "volumes"
)

type Alertmanager struct {
	types.Resource
	AdditionalPeers      []string               `json:"additionalPeers,omitempty" yaml:"additionalPeers,omitempty"`
	Affinity             *Affinity              `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Annotations          map[string]string      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	BaseImage            string                 `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	ConfigMaps           []string               `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Containers           []Container            `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created              string                 `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	ExternalURL          string                 `json:"externalUrl,omitempty" yaml:"externalUrl,omitempty"`
	Image                string                 `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullSecrets     []LocalObjectReference `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	InitContainers       []Container            `json:"initContainers,omitempty" yaml:"initContainers,omitempty"`
	Labels               map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	ListenLocal          bool                   `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	LogFormat            string                 `json:"logFormat,omitempty" yaml:"logFormat,omitempty"`
	LogLevel             string                 `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	Name                 string                 `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string                 `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeSelector         map[string]string      `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	OwnerReferences      []OwnerReference       `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Paused               bool                   `json:"paused,omitempty" yaml:"paused,omitempty"`
	PodMetadata          *ObjectMeta            `json:"podMetadata,omitempty" yaml:"podMetadata,omitempty"`
	PortName             string                 `json:"portName,omitempty" yaml:"portName,omitempty"`
	PriorityClassName    string                 `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	ProjectID            string                 `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed              string                 `json:"removed,omitempty" yaml:"removed,omitempty"`
	Replicas             *int64                 `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Resources            *ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Retention            string                 `json:"retention,omitempty" yaml:"retention,omitempty"`
	RoutePrefix          string                 `json:"routePrefix,omitempty" yaml:"routePrefix,omitempty"`
	SHA                  string                 `json:"sha,omitempty" yaml:"sha,omitempty"`
	Secrets              []string               `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecurityContext      *PodSecurityContext    `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccountName   string                 `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	State                string                 `json:"state,omitempty" yaml:"state,omitempty"`
	Storage              *StorageSpec           `json:"storage,omitempty" yaml:"storage,omitempty"`
	Tag                  string                 `json:"tag,omitempty" yaml:"tag,omitempty"`
	Tolerations          []Toleration           `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Transitioning        string                 `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string                 `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Version              string                 `json:"version,omitempty" yaml:"version,omitempty"`
	VolumeMounts         []VolumeMount          `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Volumes              []Volume               `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}

type AlertmanagerCollection struct {
	types.Collection
	Data   []Alertmanager `json:"data,omitempty"`
	client *AlertmanagerClient
}

type AlertmanagerClient struct {
	apiClient *Client
}

type AlertmanagerOperations interface {
	List(opts *types.ListOpts) (*AlertmanagerCollection, error)
	Create(opts *Alertmanager) (*Alertmanager, error)
	Update(existing *Alertmanager, updates interface{}) (*Alertmanager, error)
	Replace(existing *Alertmanager) (*Alertmanager, error)
	ByID(id string) (*Alertmanager, error)
	Delete(container *Alertmanager) error
}

func newAlertmanagerClient(apiClient *Client) *AlertmanagerClient {
	return &AlertmanagerClient{
		apiClient: apiClient,
	}
}

func (c *AlertmanagerClient) Create(container *Alertmanager) (*Alertmanager, error) {
	resp := &Alertmanager{}
	err := c.apiClient.Ops.DoCreate(AlertmanagerType, container, resp)
	return resp, err
}

func (c *AlertmanagerClient) Update(existing *Alertmanager, updates interface{}) (*Alertmanager, error) {
	resp := &Alertmanager{}
	err := c.apiClient.Ops.DoUpdate(AlertmanagerType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *AlertmanagerClient) Replace(obj *Alertmanager) (*Alertmanager, error) {
	resp := &Alertmanager{}
	err := c.apiClient.Ops.DoReplace(AlertmanagerType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *AlertmanagerClient) List(opts *types.ListOpts) (*AlertmanagerCollection, error) {
	resp := &AlertmanagerCollection{}
	err := c.apiClient.Ops.DoList(AlertmanagerType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *AlertmanagerCollection) Next() (*AlertmanagerCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &AlertmanagerCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *AlertmanagerClient) ByID(id string) (*Alertmanager, error) {
	resp := &Alertmanager{}
	err := c.apiClient.Ops.DoByID(AlertmanagerType, id, resp)
	return resp, err
}

func (c *AlertmanagerClient) Delete(container *Alertmanager) error {
	return c.apiClient.Ops.DoResourceDelete(AlertmanagerType, &container.Resource)
}
