package client

const (
	PrometheusSpecType                                    = "prometheusSpec"
	PrometheusSpecFieldAdditionalAlertManagerConfigs      = "additionalAlertManagerConfigs"
	PrometheusSpecFieldAdditionalAlertRelabelConfigs      = "additionalAlertRelabelConfigs"
	PrometheusSpecFieldAdditionalScrapeConfigs            = "additionalScrapeConfigs"
	PrometheusSpecFieldAffinity                           = "affinity"
	PrometheusSpecFieldAlerting                           = "alerting"
	PrometheusSpecFieldAllowOverlappingBlocks             = "allowOverlappingBlocks"
	PrometheusSpecFieldArbitraryFSAccessThroughSMs        = "arbitraryFSAccessThroughSMs"
	PrometheusSpecFieldBaseImage                          = "baseImage"
	PrometheusSpecFieldConfigMaps                         = "configMaps"
	PrometheusSpecFieldContainers                         = "containers"
	PrometheusSpecFieldDisableCompaction                  = "disableCompaction"
	PrometheusSpecFieldEnableAdminAPI                     = "enableAdminAPI"
	PrometheusSpecFieldEnableFeatures                     = "enableFeatures"
	PrometheusSpecFieldEnforcedBodySizeLimit              = "enforcedBodySizeLimit"
	PrometheusSpecFieldEnforcedLabelLimit                 = "enforcedLabelLimit"
	PrometheusSpecFieldEnforcedLabelNameLengthLimit       = "enforcedLabelNameLengthLimit"
	PrometheusSpecFieldEnforcedLabelValueLengthLimit      = "enforcedLabelValueLengthLimit"
	PrometheusSpecFieldEnforcedNamespaceLabel             = "enforcedNamespaceLabel"
	PrometheusSpecFieldEnforcedSampleLimit                = "enforcedSampleLimit"
	PrometheusSpecFieldEnforcedTargetLimit                = "enforcedTargetLimit"
	PrometheusSpecFieldEvaluationInterval                 = "evaluationInterval"
	PrometheusSpecFieldExternalLabels                     = "externalLabels"
	PrometheusSpecFieldExternalURL                        = "externalUrl"
	PrometheusSpecFieldIgnoreNamespaceSelectors           = "ignoreNamespaceSelectors"
	PrometheusSpecFieldImage                              = "image"
	PrometheusSpecFieldImagePullSecrets                   = "imagePullSecrets"
	PrometheusSpecFieldInitContainers                     = "initContainers"
	PrometheusSpecFieldListenLocal                        = "listenLocal"
	PrometheusSpecFieldLogFormat                          = "logFormat"
	PrometheusSpecFieldLogLevel                           = "logLevel"
	PrometheusSpecFieldMinReadySeconds                    = "minReadySeconds"
	PrometheusSpecFieldNodeSelector                       = "nodeSelector"
	PrometheusSpecFieldOverrideHonorLabels                = "overrideHonorLabels"
	PrometheusSpecFieldOverrideHonorTimestamps            = "overrideHonorTimestamps"
	PrometheusSpecFieldPodMetadata                        = "podMetadata"
	PrometheusSpecFieldPodMonitorNamespaceSelector        = "podMonitorNamespaceSelector"
	PrometheusSpecFieldPodMonitorSelector                 = "podMonitorSelector"
	PrometheusSpecFieldPortName                           = "portName"
	PrometheusSpecFieldPriorityClassName                  = "priorityClassName"
	PrometheusSpecFieldProbeNamespaceSelector             = "probeNamespaceSelector"
	PrometheusSpecFieldProbeSelector                      = "probeSelector"
	PrometheusSpecFieldPrometheusExternalLabelName        = "prometheusExternalLabelName"
	PrometheusSpecFieldPrometheusRulesExcludedFromEnforce = "prometheusRulesExcludedFromEnforce"
	PrometheusSpecFieldQuery                              = "query"
	PrometheusSpecFieldQueryLogFile                       = "queryLogFile"
	PrometheusSpecFieldRemoteRead                         = "remoteRead"
	PrometheusSpecFieldRemoteWrite                        = "remoteWrite"
	PrometheusSpecFieldReplicaExternalLabelName           = "replicaExternalLabelName"
	PrometheusSpecFieldReplicas                           = "replicas"
	PrometheusSpecFieldResources                          = "resources"
	PrometheusSpecFieldRetention                          = "retention"
	PrometheusSpecFieldRetentionSize                      = "retentionSize"
	PrometheusSpecFieldRoutePrefix                        = "routePrefix"
	PrometheusSpecFieldRuleSelector                       = "ruleSelector"
	PrometheusSpecFieldRules                              = "rules"
	PrometheusSpecFieldSHA                                = "sha"
	PrometheusSpecFieldScrapeInterval                     = "scrapeInterval"
	PrometheusSpecFieldScrapeTimeout                      = "scrapeTimeout"
	PrometheusSpecFieldSecrets                            = "secrets"
	PrometheusSpecFieldSecurityContext                    = "securityContext"
	PrometheusSpecFieldServiceAccountName                 = "serviceAccountName"
	PrometheusSpecFieldServiceMonitorSelector             = "serviceMonitorSelector"
	PrometheusSpecFieldShards                             = "shards"
	PrometheusSpecFieldStorage                            = "storage"
	PrometheusSpecFieldTag                                = "tag"
	PrometheusSpecFieldTolerations                        = "tolerations"
	PrometheusSpecFieldTopologySpreadConstraints          = "topologySpreadConstraints"
	PrometheusSpecFieldVersion                            = "version"
	PrometheusSpecFieldVolumeMounts                       = "volumeMounts"
	PrometheusSpecFieldVolumes                            = "volumes"
	PrometheusSpecFieldWALCompression                     = "walCompression"
	PrometheusSpecFieldWeb                                = "web"
)

type PrometheusSpec struct {
	AdditionalAlertManagerConfigs      *SecretKeySelector                 `json:"additionalAlertManagerConfigs,omitempty" yaml:"additionalAlertManagerConfigs,omitempty"`
	AdditionalAlertRelabelConfigs      *SecretKeySelector                 `json:"additionalAlertRelabelConfigs,omitempty" yaml:"additionalAlertRelabelConfigs,omitempty"`
	AdditionalScrapeConfigs            *SecretKeySelector                 `json:"additionalScrapeConfigs,omitempty" yaml:"additionalScrapeConfigs,omitempty"`
	Affinity                           *Affinity                          `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Alerting                           *AlertingSpec                      `json:"alerting,omitempty" yaml:"alerting,omitempty"`
	AllowOverlappingBlocks             bool                               `json:"allowOverlappingBlocks,omitempty" yaml:"allowOverlappingBlocks,omitempty"`
	ArbitraryFSAccessThroughSMs        *ArbitraryFSAccessThroughSMsConfig `json:"arbitraryFSAccessThroughSMs,omitempty" yaml:"arbitraryFSAccessThroughSMs,omitempty"`
	BaseImage                          string                             `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	ConfigMaps                         []string                           `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Containers                         []Container                        `json:"containers,omitempty" yaml:"containers,omitempty"`
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
	ListenLocal                        bool                               `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	LogFormat                          string                             `json:"logFormat,omitempty" yaml:"logFormat,omitempty"`
	LogLevel                           string                             `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	MinReadySeconds                    *int64                             `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
	NodeSelector                       map[string]string                  `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	OverrideHonorLabels                bool                               `json:"overrideHonorLabels,omitempty" yaml:"overrideHonorLabels,omitempty"`
	OverrideHonorTimestamps            bool                               `json:"overrideHonorTimestamps,omitempty" yaml:"overrideHonorTimestamps,omitempty"`
	PodMetadata                        *EmbeddedObjectMetadata            `json:"podMetadata,omitempty" yaml:"podMetadata,omitempty"`
	PodMonitorNamespaceSelector        *LabelSelector                     `json:"podMonitorNamespaceSelector,omitempty" yaml:"podMonitorNamespaceSelector,omitempty"`
	PodMonitorSelector                 *LabelSelector                     `json:"podMonitorSelector,omitempty" yaml:"podMonitorSelector,omitempty"`
	PortName                           string                             `json:"portName,omitempty" yaml:"portName,omitempty"`
	PriorityClassName                  string                             `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	ProbeNamespaceSelector             *LabelSelector                     `json:"probeNamespaceSelector,omitempty" yaml:"probeNamespaceSelector,omitempty"`
	ProbeSelector                      *LabelSelector                     `json:"probeSelector,omitempty" yaml:"probeSelector,omitempty"`
	PrometheusExternalLabelName        string                             `json:"prometheusExternalLabelName,omitempty" yaml:"prometheusExternalLabelName,omitempty"`
	PrometheusRulesExcludedFromEnforce []PrometheusRuleExcludeConfig      `json:"prometheusRulesExcludedFromEnforce,omitempty" yaml:"prometheusRulesExcludedFromEnforce,omitempty"`
	Query                              *QuerySpec                         `json:"query,omitempty" yaml:"query,omitempty"`
	QueryLogFile                       string                             `json:"queryLogFile,omitempty" yaml:"queryLogFile,omitempty"`
	RemoteRead                         []RemoteReadSpec                   `json:"remoteRead,omitempty" yaml:"remoteRead,omitempty"`
	RemoteWrite                        []RemoteWriteSpec                  `json:"remoteWrite,omitempty" yaml:"remoteWrite,omitempty"`
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
	Storage                            *StorageSpec                       `json:"storage,omitempty" yaml:"storage,omitempty"`
	Tag                                string                             `json:"tag,omitempty" yaml:"tag,omitempty"`
	Tolerations                        []Toleration                       `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	TopologySpreadConstraints          []TopologySpreadConstraint         `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Version                            string                             `json:"version,omitempty" yaml:"version,omitempty"`
	VolumeMounts                       []VolumeMount                      `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Volumes                            []Volume                           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WALCompression                     *bool                              `json:"walCompression,omitempty" yaml:"walCompression,omitempty"`
	Web                                *WebSpec                           `json:"web,omitempty" yaml:"web,omitempty"`
}
