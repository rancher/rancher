package client

const (
	AlertmanagerSpecType                    = "alertmanagerSpec"
	AlertmanagerSpecFieldAdditionalPeers    = "additionalPeers"
	AlertmanagerSpecFieldAffinity           = "affinity"
	AlertmanagerSpecFieldBaseImage          = "baseImage"
	AlertmanagerSpecFieldConfigMaps         = "configMaps"
	AlertmanagerSpecFieldConfigSecret       = "configSecret"
	AlertmanagerSpecFieldContainers         = "containers"
	AlertmanagerSpecFieldExternalURL        = "externalUrl"
	AlertmanagerSpecFieldImage              = "image"
	AlertmanagerSpecFieldImagePullSecrets   = "imagePullSecrets"
	AlertmanagerSpecFieldInitContainers     = "initContainers"
	AlertmanagerSpecFieldListenLocal        = "listenLocal"
	AlertmanagerSpecFieldLogFormat          = "logFormat"
	AlertmanagerSpecFieldLogLevel           = "logLevel"
	AlertmanagerSpecFieldNodeSelector       = "nodeSelector"
	AlertmanagerSpecFieldPaused             = "paused"
	AlertmanagerSpecFieldPodMetadata        = "podMetadata"
	AlertmanagerSpecFieldPortName           = "portName"
	AlertmanagerSpecFieldPriorityClassName  = "priorityClassName"
	AlertmanagerSpecFieldReplicas           = "replicas"
	AlertmanagerSpecFieldResources          = "resources"
	AlertmanagerSpecFieldRetention          = "retention"
	AlertmanagerSpecFieldRoutePrefix        = "routePrefix"
	AlertmanagerSpecFieldSHA                = "sha"
	AlertmanagerSpecFieldSecrets            = "secrets"
	AlertmanagerSpecFieldSecurityContext    = "securityContext"
	AlertmanagerSpecFieldServiceAccountName = "serviceAccountName"
	AlertmanagerSpecFieldStorage            = "storage"
	AlertmanagerSpecFieldTag                = "tag"
	AlertmanagerSpecFieldTolerations        = "tolerations"
	AlertmanagerSpecFieldVersion            = "version"
	AlertmanagerSpecFieldVolumeMounts       = "volumeMounts"
	AlertmanagerSpecFieldVolumes            = "volumes"
)

type AlertmanagerSpec struct {
	AdditionalPeers    []string               `json:"additionalPeers,omitempty" yaml:"additionalPeers,omitempty"`
	Affinity           *Affinity              `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	BaseImage          string                 `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	ConfigMaps         []string               `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	ConfigSecret       string                 `json:"configSecret,omitempty" yaml:"configSecret,omitempty"`
	Containers         []Container            `json:"containers,omitempty" yaml:"containers,omitempty"`
	ExternalURL        string                 `json:"externalUrl,omitempty" yaml:"externalUrl,omitempty"`
	Image              string                 `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullSecrets   []LocalObjectReference `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	InitContainers     []Container            `json:"initContainers,omitempty" yaml:"initContainers,omitempty"`
	ListenLocal        bool                   `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	LogFormat          string                 `json:"logFormat,omitempty" yaml:"logFormat,omitempty"`
	LogLevel           string                 `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	NodeSelector       map[string]string      `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Paused             bool                   `json:"paused,omitempty" yaml:"paused,omitempty"`
	PodMetadata        *ObjectMeta            `json:"podMetadata,omitempty" yaml:"podMetadata,omitempty"`
	PortName           string                 `json:"portName,omitempty" yaml:"portName,omitempty"`
	PriorityClassName  string                 `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	Replicas           *int64                 `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Resources          *ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Retention          string                 `json:"retention,omitempty" yaml:"retention,omitempty"`
	RoutePrefix        string                 `json:"routePrefix,omitempty" yaml:"routePrefix,omitempty"`
	SHA                string                 `json:"sha,omitempty" yaml:"sha,omitempty"`
	Secrets            []string               `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecurityContext    *PodSecurityContext    `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccountName string                 `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	Storage            *StorageSpec           `json:"storage,omitempty" yaml:"storage,omitempty"`
	Tag                string                 `json:"tag,omitempty" yaml:"tag,omitempty"`
	Tolerations        []Toleration           `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Version            string                 `json:"version,omitempty" yaml:"version,omitempty"`
	VolumeMounts       []VolumeMount          `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Volumes            []Volume               `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}
