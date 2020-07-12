package client

const (
	PrometheusSpecType                               = "prometheusSpec"
	PrometheusSpecFieldAdditionalAlertManagerConfigs = "additionalAlertManagerConfigs"
	PrometheusSpecFieldAdditionalAlertRelabelConfigs = "additionalAlertRelabelConfigs"
	PrometheusSpecFieldAdditionalScrapeConfigs       = "additionalScrapeConfigs"
	PrometheusSpecFieldAffinity                      = "affinity"
	PrometheusSpecFieldAlerting                      = "alerting"
	PrometheusSpecFieldArbitraryFSAccessThroughSMs   = "arbitraryFSAccessThroughSMs"
	PrometheusSpecFieldBaseImage                     = "baseImage"
	PrometheusSpecFieldConfigMaps                    = "configMaps"
	PrometheusSpecFieldContainers                    = "containers"
	PrometheusSpecFieldDisableCompaction             = "disableCompaction"
	PrometheusSpecFieldEnableAdminAPI                = "enableAdminAPI"
	PrometheusSpecFieldEnforcedNamespaceLabel        = "enforcedNamespaceLabel"
	PrometheusSpecFieldEvaluationInterval            = "evaluationInterval"
	PrometheusSpecFieldExternalLabels                = "externalLabels"
	PrometheusSpecFieldExternalURL                   = "externalUrl"
	PrometheusSpecFieldIgnoreNamespaceSelectors      = "ignoreNamespaceSelectors"
	PrometheusSpecFieldImage                         = "image"
	PrometheusSpecFieldImagePullSecrets              = "imagePullSecrets"
	PrometheusSpecFieldInitContainers                = "initContainers"
	PrometheusSpecFieldListenLocal                   = "listenLocal"
	PrometheusSpecFieldLogFormat                     = "logFormat"
	PrometheusSpecFieldLogLevel                      = "logLevel"
	PrometheusSpecFieldNodeSelector                  = "nodeSelector"
	PrometheusSpecFieldOverrideHonorLabels           = "overrideHonorLabels"
	PrometheusSpecFieldOverrideHonorTimestamps       = "overrideHonorTimestamps"
	PrometheusSpecFieldPodMetadata                   = "podMetadata"
	PrometheusSpecFieldPodMonitorNamespaceSelector   = "podMonitorNamespaceSelector"
	PrometheusSpecFieldPodMonitorSelector            = "podMonitorSelector"
	PrometheusSpecFieldPortName                      = "portName"
	PrometheusSpecFieldPriorityClassName             = "priorityClassName"
	PrometheusSpecFieldPrometheusExternalLabelName   = "prometheusExternalLabelName"
	PrometheusSpecFieldQuery                         = "query"
	PrometheusSpecFieldRemoteRead                    = "remoteRead"
	PrometheusSpecFieldRemoteWrite                   = "remoteWrite"
	PrometheusSpecFieldReplicaExternalLabelName      = "replicaExternalLabelName"
	PrometheusSpecFieldReplicas                      = "replicas"
	PrometheusSpecFieldResources                     = "resources"
	PrometheusSpecFieldRetention                     = "retention"
	PrometheusSpecFieldRetentionSize                 = "retentionSize"
	PrometheusSpecFieldRoutePrefix                   = "routePrefix"
	PrometheusSpecFieldRuleSelector                  = "ruleSelector"
	PrometheusSpecFieldRules                         = "rules"
	PrometheusSpecFieldSHA                           = "sha"
	PrometheusSpecFieldScrapeInterval                = "scrapeInterval"
	PrometheusSpecFieldSecrets                       = "secrets"
	PrometheusSpecFieldSecurityContext               = "securityContext"
	PrometheusSpecFieldServiceAccountName            = "serviceAccountName"
	PrometheusSpecFieldServiceMonitorSelector        = "serviceMonitorSelector"
	PrometheusSpecFieldStorage                       = "storage"
	PrometheusSpecFieldTag                           = "tag"
	PrometheusSpecFieldTolerations                   = "tolerations"
	PrometheusSpecFieldVersion                       = "version"
	PrometheusSpecFieldVolumes                       = "volumes"
	PrometheusSpecFieldWALCompression                = "walCompression"
)

type PrometheusSpec struct {
	AdditionalAlertManagerConfigs *SecretKeySelector                 `json:"additionalAlertManagerConfigs,omitempty" yaml:"additionalAlertManagerConfigs,omitempty"`
	AdditionalAlertRelabelConfigs *SecretKeySelector                 `json:"additionalAlertRelabelConfigs,omitempty" yaml:"additionalAlertRelabelConfigs,omitempty"`
	AdditionalScrapeConfigs       *SecretKeySelector                 `json:"additionalScrapeConfigs,omitempty" yaml:"additionalScrapeConfigs,omitempty"`
	Affinity                      *Affinity                          `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Alerting                      *AlertingSpec                      `json:"alerting,omitempty" yaml:"alerting,omitempty"`
	ArbitraryFSAccessThroughSMs   *ArbitraryFSAccessThroughSMsConfig `json:"arbitraryFSAccessThroughSMs,omitempty" yaml:"arbitraryFSAccessThroughSMs,omitempty"`
	BaseImage                     string                             `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	ConfigMaps                    []string                           `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Containers                    []Container                        `json:"containers,omitempty" yaml:"containers,omitempty"`
	DisableCompaction             bool                               `json:"disableCompaction,omitempty" yaml:"disableCompaction,omitempty"`
	EnableAdminAPI                bool                               `json:"enableAdminAPI,omitempty" yaml:"enableAdminAPI,omitempty"`
	EnforcedNamespaceLabel        string                             `json:"enforcedNamespaceLabel,omitempty" yaml:"enforcedNamespaceLabel,omitempty"`
	EvaluationInterval            string                             `json:"evaluationInterval,omitempty" yaml:"evaluationInterval,omitempty"`
	ExternalLabels                map[string]string                  `json:"externalLabels,omitempty" yaml:"externalLabels,omitempty"`
	ExternalURL                   string                             `json:"externalUrl,omitempty" yaml:"externalUrl,omitempty"`
	IgnoreNamespaceSelectors      bool                               `json:"ignoreNamespaceSelectors,omitempty" yaml:"ignoreNamespaceSelectors,omitempty"`
	Image                         string                             `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullSecrets              []LocalObjectReference             `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	InitContainers                []Container                        `json:"initContainers,omitempty" yaml:"initContainers,omitempty"`
	ListenLocal                   bool                               `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	LogFormat                     string                             `json:"logFormat,omitempty" yaml:"logFormat,omitempty"`
	LogLevel                      string                             `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	NodeSelector                  map[string]string                  `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	OverrideHonorLabels           bool                               `json:"overrideHonorLabels,omitempty" yaml:"overrideHonorLabels,omitempty"`
	OverrideHonorTimestamps       bool                               `json:"overrideHonorTimestamps,omitempty" yaml:"overrideHonorTimestamps,omitempty"`
	PodMetadata                   *ObjectMeta                        `json:"podMetadata,omitempty" yaml:"podMetadata,omitempty"`
	PodMonitorNamespaceSelector   *LabelSelector                     `json:"podMonitorNamespaceSelector,omitempty" yaml:"podMonitorNamespaceSelector,omitempty"`
	PodMonitorSelector            *LabelSelector                     `json:"podMonitorSelector,omitempty" yaml:"podMonitorSelector,omitempty"`
	PortName                      string                             `json:"portName,omitempty" yaml:"portName,omitempty"`
	PriorityClassName             string                             `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	PrometheusExternalLabelName   string                             `json:"prometheusExternalLabelName,omitempty" yaml:"prometheusExternalLabelName,omitempty"`
	Query                         *QuerySpec                         `json:"query,omitempty" yaml:"query,omitempty"`
	RemoteRead                    []RemoteReadSpec                   `json:"remoteRead,omitempty" yaml:"remoteRead,omitempty"`
	RemoteWrite                   []RemoteWriteSpec                  `json:"remoteWrite,omitempty" yaml:"remoteWrite,omitempty"`
	ReplicaExternalLabelName      string                             `json:"replicaExternalLabelName,omitempty" yaml:"replicaExternalLabelName,omitempty"`
	Replicas                      *int64                             `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Resources                     *ResourceRequirements              `json:"resources,omitempty" yaml:"resources,omitempty"`
	Retention                     string                             `json:"retention,omitempty" yaml:"retention,omitempty"`
	RetentionSize                 string                             `json:"retentionSize,omitempty" yaml:"retentionSize,omitempty"`
	RoutePrefix                   string                             `json:"routePrefix,omitempty" yaml:"routePrefix,omitempty"`
	RuleSelector                  *LabelSelector                     `json:"ruleSelector,omitempty" yaml:"ruleSelector,omitempty"`
	Rules                         *Rules                             `json:"rules,omitempty" yaml:"rules,omitempty"`
	SHA                           string                             `json:"sha,omitempty" yaml:"sha,omitempty"`
	ScrapeInterval                string                             `json:"scrapeInterval,omitempty" yaml:"scrapeInterval,omitempty"`
	Secrets                       []string                           `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecurityContext               *PodSecurityContext                `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccountName            string                             `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	ServiceMonitorSelector        *LabelSelector                     `json:"serviceMonitorSelector,omitempty" yaml:"serviceMonitorSelector,omitempty"`
	Storage                       *StorageSpec                       `json:"storage,omitempty" yaml:"storage,omitempty"`
	Tag                           string                             `json:"tag,omitempty" yaml:"tag,omitempty"`
	Tolerations                   []Toleration                       `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Version                       string                             `json:"version,omitempty" yaml:"version,omitempty"`
	Volumes                       []Volume                           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WALCompression                *bool                              `json:"walCompression,omitempty" yaml:"walCompression,omitempty"`
}
