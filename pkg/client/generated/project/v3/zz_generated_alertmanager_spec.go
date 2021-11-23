package client

const (
	AlertmanagerSpecType                                     = "alertmanagerSpec"
	AlertmanagerSpecFieldAdditionalPeers                     = "additionalPeers"
	AlertmanagerSpecFieldAffinity                            = "affinity"
	AlertmanagerSpecFieldAlertmanagerConfigNamespaceSelector = "alertmanagerConfigNamespaceSelector"
	AlertmanagerSpecFieldAlertmanagerConfigSelector          = "alertmanagerConfigSelector"
	AlertmanagerSpecFieldBaseImage                           = "baseImage"
	AlertmanagerSpecFieldClusterAdvertiseAddress             = "clusterAdvertiseAddress"
	AlertmanagerSpecFieldClusterGossipInterval               = "clusterGossipInterval"
	AlertmanagerSpecFieldClusterPeerTimeout                  = "clusterPeerTimeout"
	AlertmanagerSpecFieldClusterPushpullInterval             = "clusterPushpullInterval"
	AlertmanagerSpecFieldConfigMaps                          = "configMaps"
	AlertmanagerSpecFieldConfigSecret                        = "configSecret"
	AlertmanagerSpecFieldContainers                          = "containers"
	AlertmanagerSpecFieldExternalURL                         = "externalUrl"
	AlertmanagerSpecFieldForceEnableClusterMode              = "forceEnableClusterMode"
	AlertmanagerSpecFieldImage                               = "image"
	AlertmanagerSpecFieldImagePullSecrets                    = "imagePullSecrets"
	AlertmanagerSpecFieldInitContainers                      = "initContainers"
	AlertmanagerSpecFieldListenLocal                         = "listenLocal"
	AlertmanagerSpecFieldLogFormat                           = "logFormat"
	AlertmanagerSpecFieldLogLevel                            = "logLevel"
	AlertmanagerSpecFieldMinReadySeconds                     = "minReadySeconds"
	AlertmanagerSpecFieldNodeSelector                        = "nodeSelector"
	AlertmanagerSpecFieldPaused                              = "paused"
	AlertmanagerSpecFieldPodMetadata                         = "podMetadata"
	AlertmanagerSpecFieldPortName                            = "portName"
	AlertmanagerSpecFieldPriorityClassName                   = "priorityClassName"
	AlertmanagerSpecFieldReplicas                            = "replicas"
	AlertmanagerSpecFieldResources                           = "resources"
	AlertmanagerSpecFieldRetention                           = "retention"
	AlertmanagerSpecFieldRoutePrefix                         = "routePrefix"
	AlertmanagerSpecFieldSHA                                 = "sha"
	AlertmanagerSpecFieldSecrets                             = "secrets"
	AlertmanagerSpecFieldSecurityContext                     = "securityContext"
	AlertmanagerSpecFieldServiceAccountName                  = "serviceAccountName"
	AlertmanagerSpecFieldStorage                             = "storage"
	AlertmanagerSpecFieldTag                                 = "tag"
	AlertmanagerSpecFieldTolerations                         = "tolerations"
	AlertmanagerSpecFieldTopologySpreadConstraints           = "topologySpreadConstraints"
	AlertmanagerSpecFieldVersion                             = "version"
	AlertmanagerSpecFieldVolumeMounts                        = "volumeMounts"
	AlertmanagerSpecFieldVolumes                             = "volumes"
)

type AlertmanagerSpec struct {
	AdditionalPeers                     []string                   `json:"additionalPeers,omitempty" yaml:"additionalPeers,omitempty"`
	Affinity                            *Affinity                  `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	AlertmanagerConfigNamespaceSelector *LabelSelector             `json:"alertmanagerConfigNamespaceSelector,omitempty" yaml:"alertmanagerConfigNamespaceSelector,omitempty"`
	AlertmanagerConfigSelector          *LabelSelector             `json:"alertmanagerConfigSelector,omitempty" yaml:"alertmanagerConfigSelector,omitempty"`
	BaseImage                           string                     `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	ClusterAdvertiseAddress             string                     `json:"clusterAdvertiseAddress,omitempty" yaml:"clusterAdvertiseAddress,omitempty"`
	ClusterGossipInterval               string                     `json:"clusterGossipInterval,omitempty" yaml:"clusterGossipInterval,omitempty"`
	ClusterPeerTimeout                  string                     `json:"clusterPeerTimeout,omitempty" yaml:"clusterPeerTimeout,omitempty"`
	ClusterPushpullInterval             string                     `json:"clusterPushpullInterval,omitempty" yaml:"clusterPushpullInterval,omitempty"`
	ConfigMaps                          []string                   `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	ConfigSecret                        string                     `json:"configSecret,omitempty" yaml:"configSecret,omitempty"`
	Containers                          []Container                `json:"containers,omitempty" yaml:"containers,omitempty"`
	ExternalURL                         string                     `json:"externalUrl,omitempty" yaml:"externalUrl,omitempty"`
	ForceEnableClusterMode              bool                       `json:"forceEnableClusterMode,omitempty" yaml:"forceEnableClusterMode,omitempty"`
	Image                               string                     `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullSecrets                    []LocalObjectReference     `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	InitContainers                      []Container                `json:"initContainers,omitempty" yaml:"initContainers,omitempty"`
	ListenLocal                         bool                       `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	LogFormat                           string                     `json:"logFormat,omitempty" yaml:"logFormat,omitempty"`
	LogLevel                            string                     `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	MinReadySeconds                     *int64                     `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
	NodeSelector                        map[string]string          `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Paused                              bool                       `json:"paused,omitempty" yaml:"paused,omitempty"`
	PodMetadata                         *EmbeddedObjectMetadata    `json:"podMetadata,omitempty" yaml:"podMetadata,omitempty"`
	PortName                            string                     `json:"portName,omitempty" yaml:"portName,omitempty"`
	PriorityClassName                   string                     `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	Replicas                            *int64                     `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Resources                           *ResourceRequirements      `json:"resources,omitempty" yaml:"resources,omitempty"`
	Retention                           string                     `json:"retention,omitempty" yaml:"retention,omitempty"`
	RoutePrefix                         string                     `json:"routePrefix,omitempty" yaml:"routePrefix,omitempty"`
	SHA                                 string                     `json:"sha,omitempty" yaml:"sha,omitempty"`
	Secrets                             []string                   `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecurityContext                     *PodSecurityContext        `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccountName                  string                     `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	Storage                             *StorageSpec               `json:"storage,omitempty" yaml:"storage,omitempty"`
	Tag                                 string                     `json:"tag,omitempty" yaml:"tag,omitempty"`
	Tolerations                         []Toleration               `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	TopologySpreadConstraints           []TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Version                             string                     `json:"version,omitempty" yaml:"version,omitempty"`
	VolumeMounts                        []VolumeMount              `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Volumes                             []Volume                   `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}
