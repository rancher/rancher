package client

import (
	"github.com/rancher/norman/types"
)

const (
	PrometheusType                                    = "prometheus"
	PrometheusFieldAdditionalAlertManagerConfigs      = "additionalAlertManagerConfigs"
	PrometheusFieldAdditionalAlertRelabelConfigs      = "additionalAlertRelabelConfigs"
	PrometheusFieldAdditionalScrapeConfigs            = "additionalScrapeConfigs"
	PrometheusFieldAffinity                           = "affinity"
	PrometheusFieldAlerting                           = "alerting"
	PrometheusFieldAllowOverlappingBlocks             = "allowOverlappingBlocks"
	PrometheusFieldAnnotations                        = "annotations"
	PrometheusFieldArbitraryFSAccessThroughSMs        = "arbitraryFSAccessThroughSMs"
	PrometheusFieldBaseImage                          = "baseImage"
	PrometheusFieldConfigMaps                         = "configMaps"
	PrometheusFieldContainers                         = "containers"
	PrometheusFieldCreated                            = "created"
	PrometheusFieldCreatorID                          = "creatorId"
	PrometheusFieldDescription                        = "description"
	PrometheusFieldDisableCompaction                  = "disableCompaction"
	PrometheusFieldEnableAdminAPI                     = "enableAdminAPI"
	PrometheusFieldEnableFeatures                     = "enableFeatures"
	PrometheusFieldEnforcedBodySizeLimit              = "enforcedBodySizeLimit"
	PrometheusFieldEnforcedLabelLimit                 = "enforcedLabelLimit"
	PrometheusFieldEnforcedLabelNameLengthLimit       = "enforcedLabelNameLengthLimit"
	PrometheusFieldEnforcedLabelValueLengthLimit      = "enforcedLabelValueLengthLimit"
	PrometheusFieldEnforcedNamespaceLabel             = "enforcedNamespaceLabel"
	PrometheusFieldEnforcedSampleLimit                = "enforcedSampleLimit"
	PrometheusFieldEnforcedTargetLimit                = "enforcedTargetLimit"
	PrometheusFieldEvaluationInterval                 = "evaluationInterval"
	PrometheusFieldExternalLabels                     = "externalLabels"
	PrometheusFieldExternalURL                        = "externalUrl"
	PrometheusFieldIgnoreNamespaceSelectors           = "ignoreNamespaceSelectors"
	PrometheusFieldImage                              = "image"
	PrometheusFieldImagePullSecrets                   = "imagePullSecrets"
	PrometheusFieldInitContainers                     = "initContainers"
	PrometheusFieldLabels                             = "labels"
	PrometheusFieldListenLocal                        = "listenLocal"
	PrometheusFieldLogFormat                          = "logFormat"
	PrometheusFieldLogLevel                           = "logLevel"
	PrometheusFieldMinReadySeconds                    = "minReadySeconds"
	PrometheusFieldName                               = "name"
	PrometheusFieldNamespaceId                        = "namespaceId"
	PrometheusFieldNodeSelector                       = "nodeSelector"
	PrometheusFieldOverrideHonorLabels                = "overrideHonorLabels"
	PrometheusFieldOverrideHonorTimestamps            = "overrideHonorTimestamps"
	PrometheusFieldOwnerReferences                    = "ownerReferences"
	PrometheusFieldPodMetadata                        = "podMetadata"
	PrometheusFieldPodMonitorNamespaceSelector        = "podMonitorNamespaceSelector"
	PrometheusFieldPodMonitorSelector                 = "podMonitorSelector"
	PrometheusFieldPortName                           = "portName"
	PrometheusFieldPriorityClassName                  = "priorityClassName"
	PrometheusFieldProbeNamespaceSelector             = "probeNamespaceSelector"
	PrometheusFieldProbeSelector                      = "probeSelector"
	PrometheusFieldProjectID                          = "projectId"
	PrometheusFieldPrometheusExternalLabelName        = "prometheusExternalLabelName"
	PrometheusFieldPrometheusRulesExcludedFromEnforce = "prometheusRulesExcludedFromEnforce"
	PrometheusFieldQuery                              = "query"
	PrometheusFieldQueryLogFile                       = "queryLogFile"
	PrometheusFieldRemoteRead                         = "remoteRead"
	PrometheusFieldRemoteWrite                        = "remoteWrite"
	PrometheusFieldRemoved                            = "removed"
	PrometheusFieldReplicaExternalLabelName           = "replicaExternalLabelName"
	PrometheusFieldReplicas                           = "replicas"
	PrometheusFieldResources                          = "resources"
	PrometheusFieldRetention                          = "retention"
	PrometheusFieldRetentionSize                      = "retentionSize"
	PrometheusFieldRoutePrefix                        = "routePrefix"
	PrometheusFieldRuleSelector                       = "ruleSelector"
	PrometheusFieldRules                              = "rules"
	PrometheusFieldSHA                                = "sha"
	PrometheusFieldScrapeInterval                     = "scrapeInterval"
	PrometheusFieldScrapeTimeout                      = "scrapeTimeout"
	PrometheusFieldSecrets                            = "secrets"
	PrometheusFieldSecurityContext                    = "securityContext"
	PrometheusFieldServiceAccountName                 = "serviceAccountName"
	PrometheusFieldServiceMonitorSelector             = "serviceMonitorSelector"
	PrometheusFieldShards                             = "shards"
	PrometheusFieldState                              = "state"
	PrometheusFieldStorage                            = "storage"
	PrometheusFieldTag                                = "tag"
	PrometheusFieldTolerations                        = "tolerations"
	PrometheusFieldTopologySpreadConstraints          = "topologySpreadConstraints"
	PrometheusFieldTransitioning                      = "transitioning"
	PrometheusFieldTransitioningMessage               = "transitioningMessage"
	PrometheusFieldUUID                               = "uuid"
	PrometheusFieldVersion                            = "version"
	PrometheusFieldVolumeMounts                       = "volumeMounts"
	PrometheusFieldVolumes                            = "volumes"
	PrometheusFieldWALCompression                     = "walCompression"
	PrometheusFieldWeb                                = "web"
)

type Prometheus struct {
	types.Resource
	AdditionalAlertManagerConfigs      *SecretKeySelector                 `json:"additionalAlertManagerConfigs,omitempty" yaml:"additionalAlertManagerConfigs,omitempty"`
	AdditionalAlertRelabelConfigs      *SecretKeySelector                 `json:"additionalAlertRelabelConfigs,omitempty" yaml:"additionalAlertRelabelConfigs,omitempty"`
	AdditionalScrapeConfigs            *SecretKeySelector                 `json:"additionalScrapeConfigs,omitempty" yaml:"additionalScrapeConfigs,omitempty"`
	Affinity                           *Affinity                          `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Alerting                           *AlertingSpec                      `json:"alerting,omitempty" yaml:"alerting,omitempty"`
	AllowOverlappingBlocks             bool                               `json:"allowOverlappingBlocks,omitempty" yaml:"allowOverlappingBlocks,omitempty"`
	Annotations                        map[string]string                  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ArbitraryFSAccessThroughSMs        *ArbitraryFSAccessThroughSMsConfig `json:"arbitraryFSAccessThroughSMs,omitempty" yaml:"arbitraryFSAccessThroughSMs,omitempty"`
	BaseImage                          string                             `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	ConfigMaps                         []string                           `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Containers                         []Container                        `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                            string                             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                          string                             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description                        string                             `json:"description,omitempty" yaml:"description,omitempty"`
	DisableCompaction                  bool                               `json:"disableCompaction,omitempty" yaml:"disableCompaction,omitempty"`
	EnableAdminAPI                     bool                               `json:"enableAdminAPI,omitempty" yaml:"enableAdminAPI,omitempty"`
	EnableFeatures                     []string                           `json:"enableFeatures,omitempty" yaml:"enableFeatures,omitempty"`
	EnforcedBodySizeLimit              string                             `json:"enforcedBodySizeLimit,omitempty" yaml:"enforcedBodySizeLimit,omitempty"`
	EnforcedLabelLimit                 *int64                             `json:"enforcedLabelLimit,omitempty" yaml:"enforcedLabelLimit,omitempty"`
	EnforcedLabelNameLengthLimit       *int64                             `json:"enforcedLabelNameLengthLimit,omitempty" yaml:"enforcedLabelNameLengthLimit,omitempty"`
	EnforcedLabelValueLengthLimit      *int64                             `json:"enforcedLabelValueLengthLimit,omitempty" yaml:"enforcedLabelValueLengthLimit,omitempty"`
	EnforcedNamespaceLabel             string                             `json:"enforcedNamespaceLabel,omitempty" yaml:"enforcedNamespaceLabel,omitempty"`
	EnforcedSampleLimit                *int64                             `json:"enforcedSampleLimit,omitempty" yaml:"enforcedSampleLimit,omitempty"`
	EnforcedTargetLimit                *int64                             `json:"enforcedTargetLimit,omitempty" yaml:"enforcedTargetLimit,omitempty"`
	EvaluationInterval                 string                             `json:"evaluationInterval,omitempty" yaml:"evaluationInterval,omitempty"`
	ExternalLabels                     map[string]string                  `json:"externalLabels,omitempty" yaml:"externalLabels,omitempty"`
	ExternalURL                        string                             `json:"externalUrl,omitempty" yaml:"externalUrl,omitempty"`
	IgnoreNamespaceSelectors           bool                               `json:"ignoreNamespaceSelectors,omitempty" yaml:"ignoreNamespaceSelectors,omitempty"`
	Image                              string                             `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullSecrets                   []LocalObjectReference             `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	InitContainers                     []Container                        `json:"initContainers,omitempty" yaml:"initContainers,omitempty"`
	Labels                             map[string]string                  `json:"labels,omitempty" yaml:"labels,omitempty"`
	ListenLocal                        bool                               `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	LogFormat                          string                             `json:"logFormat,omitempty" yaml:"logFormat,omitempty"`
	LogLevel                           string                             `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	MinReadySeconds                    *int64                             `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
	Name                               string                             `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                        string                             `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeSelector                       map[string]string                  `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	OverrideHonorLabels                bool                               `json:"overrideHonorLabels,omitempty" yaml:"overrideHonorLabels,omitempty"`
	OverrideHonorTimestamps            bool                               `json:"overrideHonorTimestamps,omitempty" yaml:"overrideHonorTimestamps,omitempty"`
	OwnerReferences                    []OwnerReference                   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodMetadata                        *EmbeddedObjectMetadata            `json:"podMetadata,omitempty" yaml:"podMetadata,omitempty"`
	PodMonitorNamespaceSelector        *LabelSelector                     `json:"podMonitorNamespaceSelector,omitempty" yaml:"podMonitorNamespaceSelector,omitempty"`
	PodMonitorSelector                 *LabelSelector                     `json:"podMonitorSelector,omitempty" yaml:"podMonitorSelector,omitempty"`
	PortName                           string                             `json:"portName,omitempty" yaml:"portName,omitempty"`
	PriorityClassName                  string                             `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	ProbeNamespaceSelector             *LabelSelector                     `json:"probeNamespaceSelector,omitempty" yaml:"probeNamespaceSelector,omitempty"`
	ProbeSelector                      *LabelSelector                     `json:"probeSelector,omitempty" yaml:"probeSelector,omitempty"`
	ProjectID                          string                             `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	PrometheusExternalLabelName        string                             `json:"prometheusExternalLabelName,omitempty" yaml:"prometheusExternalLabelName,omitempty"`
	PrometheusRulesExcludedFromEnforce []PrometheusRuleExcludeConfig      `json:"prometheusRulesExcludedFromEnforce,omitempty" yaml:"prometheusRulesExcludedFromEnforce,omitempty"`
	Query                              *QuerySpec                         `json:"query,omitempty" yaml:"query,omitempty"`
	QueryLogFile                       string                             `json:"queryLogFile,omitempty" yaml:"queryLogFile,omitempty"`
	RemoteRead                         []RemoteReadSpec                   `json:"remoteRead,omitempty" yaml:"remoteRead,omitempty"`
	RemoteWrite                        []RemoteWriteSpec                  `json:"remoteWrite,omitempty" yaml:"remoteWrite,omitempty"`
	Removed                            string                             `json:"removed,omitempty" yaml:"removed,omitempty"`
	ReplicaExternalLabelName           string                             `json:"replicaExternalLabelName,omitempty" yaml:"replicaExternalLabelName,omitempty"`
	Replicas                           *int64                             `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Resources                          *ResourceRequirements              `json:"resources,omitempty" yaml:"resources,omitempty"`
	Retention                          string                             `json:"retention,omitempty" yaml:"retention,omitempty"`
	RetentionSize                      string                             `json:"retentionSize,omitempty" yaml:"retentionSize,omitempty"`
	RoutePrefix                        string                             `json:"routePrefix,omitempty" yaml:"routePrefix,omitempty"`
	RuleSelector                       *LabelSelector                     `json:"ruleSelector,omitempty" yaml:"ruleSelector,omitempty"`
	Rules                              *Rules                             `json:"rules,omitempty" yaml:"rules,omitempty"`
	SHA                                string                             `json:"sha,omitempty" yaml:"sha,omitempty"`
	ScrapeInterval                     string                             `json:"scrapeInterval,omitempty" yaml:"scrapeInterval,omitempty"`
	ScrapeTimeout                      string                             `json:"scrapeTimeout,omitempty" yaml:"scrapeTimeout,omitempty"`
	Secrets                            []string                           `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecurityContext                    *PodSecurityContext                `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccountName                 string                             `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	ServiceMonitorSelector             *LabelSelector                     `json:"serviceMonitorSelector,omitempty" yaml:"serviceMonitorSelector,omitempty"`
	Shards                             *int64                             `json:"shards,omitempty" yaml:"shards,omitempty"`
	State                              string                             `json:"state,omitempty" yaml:"state,omitempty"`
	Storage                            *StorageSpec                       `json:"storage,omitempty" yaml:"storage,omitempty"`
	Tag                                string                             `json:"tag,omitempty" yaml:"tag,omitempty"`
	Tolerations                        []Toleration                       `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	TopologySpreadConstraints          []TopologySpreadConstraint         `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Transitioning                      string                             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage               string                             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                               string                             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Version                            string                             `json:"version,omitempty" yaml:"version,omitempty"`
	VolumeMounts                       []VolumeMount                      `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Volumes                            []Volume                           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WALCompression                     *bool                              `json:"walCompression,omitempty" yaml:"walCompression,omitempty"`
	Web                                *WebSpec                           `json:"web,omitempty" yaml:"web,omitempty"`
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
	ListAll(opts *types.ListOpts) (*PrometheusCollection, error)
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

func (c *PrometheusClient) ListAll(opts *types.ListOpts) (*PrometheusCollection, error) {
	resp := &PrometheusCollection{}
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
