package v1

import (
	"encoding/json"

	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RotateEncryptionKeysPhase string

const (
	RotateEncryptionKeysPhasePrepare              RotateEncryptionKeysPhase = "Prepare"
	RotateEncryptionKeysPhasePostPrepareRestart   RotateEncryptionKeysPhase = "PostPrepareRestart"
	RotateEncryptionKeysPhaseRotate               RotateEncryptionKeysPhase = "Rotate"
	RotateEncryptionKeysPhasePostRotateRestart    RotateEncryptionKeysPhase = "PostRotateRestart"
	RotateEncryptionKeysPhaseReencrypt            RotateEncryptionKeysPhase = "Reencrypt"
	RotateEncryptionKeysPhasePostReencryptRestart RotateEncryptionKeysPhase = "PostReencryptRestart"
	RotateEncryptionKeysPhaseDone                 RotateEncryptionKeysPhase = "Done"
	RotateEncryptionKeysPhaseFailed               RotateEncryptionKeysPhase = "Failed"
)

type EnvVar struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type RKEControlPlaneSpec struct {
	RKEClusterSpecCommon

	AgentEnvVars             []EnvVar                 `json:"agentEnvVars,omitempty"`
	LocalClusterAuthEndpoint LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint"`
	ETCDSnapshotCreate       *ETCDSnapshotCreate      `json:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotRestore      *ETCDSnapshotRestore     `json:"etcdSnapshotRestore,omitempty"`
	RotateCertificates       *RotateCertificates      `json:"rotateCertificates,omitempty"`
	RotateEncryptionKeys     *RotateEncryptionKeys    `json:"rotateEncryptionKeys,omitempty"`
	KubernetesVersion        string                   `json:"kubernetesVersion,omitempty"`
	ClusterName              string                   `json:"clusterName,omitempty" wrangler:"required"`
	ManagementClusterName    string                   `json:"managementClusterName,omitempty" wrangler:"required"`
	UnmanagedConfig          bool                     `json:"unmanagedConfig,omitempty"`
}

type NetworkingStackPreference string

const (
	// DualStackPreference signifies a dual stack networking strategy, defaulting "localhost" for communication on the
	// loopback interface
	DualStackPreference = NetworkingStackPreference("dual")

	// SingleStackIPv4Preference signifies a single stack IPv4 networking strategy, defaulting "127.0.0.1" for
	// communication on the loopback interface
	SingleStackIPv4Preference = NetworkingStackPreference("ipv4")

	// SingleStackIPv6Preference signifies a single stack IPv6 networking strategy, defaulting "::1" for
	// communication on the loopback interface
	SingleStackIPv6Preference = NetworkingStackPreference("ipv6")

	// DefaultStackPreference is the stack preference used when no preference is defined, or is invalid. Defaults to
	// "127.0.0.1" to support existing behavior.
	DefaultStackPreference = SingleStackIPv4Preference
)

// Networking contains information regarding the desired and actual networking stack of the cluster.
type Networking struct {
	// Specifies which networking stack to prefer for external cluster communication. In practice, this is used by the
	// planner to render the various probes to force IPv4, IPv6, or default to localhost. There is currently no
	// sanitization or validation as cluster configuration can be specified with machineGlobalConfig and
	// machineSelectorConfig, which although easy to instrument to determine a potential interface, user defined
	// configuration can be specified in the `/etc/rancher/<rke2/k3s>/config.yaml.d` directory either manually or via
	// cloud-init, and there is currently no mechanism to extract the completely rendered configuration via the planner
	// nor various engines themselves.
	StackPreference NetworkingStackPreference `json:"stackPreference,omitempty"`
}

type DataDirectories struct {
	// SystemAgent is the data directory for the system-agent connection info and plans.
	// +optional
	SystemAgent string `json:"systemAgent,omitempty"`
	// Provisioning is the data directory for provisioning related files (e.g. idempotency).
	// +optional
	Provisioning string `json:"provisioning,omitempty"`
	// K8sDistro is the data directory for the k8s distro, i.e. the data-dir arg.
	// +optional
	K8sDistro string `json:"k8sDistro,omitempty"`
}

// RKEClusterSpecCommon contains
type RKEClusterSpecCommon struct {
	// +optional
	UpgradeStrategy ClusterUpgradeStrategy `json:"upgradeStrategy,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ChartValues GenericMap `json:"chartValues,omitempty" wrangler:"nullable"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	MachineGlobalConfig GenericMap `json:"machineGlobalConfig,omitempty" wrangler:"nullable"`

	// MachineSelectorConfig is a list of distro arguments which will be copied to
	// /etc/rancher/<rke2/k3s>/config.yaml.d/50-rancher.yaml if the machine matches
	// the label selector.
	// +optional
	MachineSelectorConfig []RKESystemConfig `json:"machineSelectorConfig,omitempty"`

	// MachineSelectorFiles is a list of files which will be copied to the machine if the machine matches the label selector.
	// +optional
	MachineSelectorFiles []RKEProvisioningFiles `json:"machineSelectorFiles,omitempty"`

	// AdditionalManifest is a string containing a yaml blob to insert in the
	// /var/lib/rancher/<rke2/k3s>/server/manifests/rancher/addons.yaml file.
	// The distro will automatically create these resources.
	// +optional
	AdditionalManifest string `json:"additionalManifest,omitempty"`

	// Registries is the list of mirrors and configurations for the cluster's container registries.
	// +optional
	Registries *Registry `json:"registries,omitempty"`

	// ETCD contains the etcd snapshot configuration for the cluster.
	// +optional
	ETCD *ETCD `json:"etcd,omitempty"`

	// Networking contains information regarding the desired networking stack of the cluster.
	// +optional
	Networking *Networking `json:"networking,omitempty"`

	// DataDirectories contains the configuration for the data directories typically stored within /var/lib/rancher.
	// The data directories must be configured via the provisioning cluster object and are immutable once set.
	// +optional
	DataDirectories *DataDirectories `json:"dataDirectories,omitempty"`

	// ProvisionGeneration is used to force the planner to reconcile the cluster,
	// regardless of whether a reconciliation is required.
	// +optional
	ProvisionGeneration int `json:"provisionGeneration,omitempty"`
}

type GenericMap struct {
	Data map[string]any `json:"-"`
}

func (in GenericMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(in.Data)
}

func (in *GenericMap) UnmarshalJSON(data []byte) error {
	in.Data = map[string]any{}
	return json.Unmarshal(data, &in.Data)
}

func (in *GenericMap) DeepCopyInto(out *GenericMap) {
	out.Data = map[string]any{}
	if err := convert.ToObj(in.Data, &out.Data); err != nil {
		panic(err)
	}
}

type ETCD struct {
	// DisableSnapshots disables the creation of snapshots for the cluster.
	// +optional
	DisableSnapshots bool `json:"disableSnapshots,omitempty"`
	// SnapshotScheduleCron is the cron schedule for the snapshot creation.
	// +optional
	SnapshotScheduleCron string `json:"snapshotScheduleCron,omitempty"`
	// SnapshotRetention is the number of snapshots the downstream cluster should retain per snapshot generation.
	// +optional
	SnapshotRetention int `json:"snapshotRetention,omitempty"`
	// S3 defines the S3 configuration for the cluster if enabled.
	// +optional
	S3 *ETCDSnapshotS3 `json:"s3,omitempty"`
}

type LocalClusterAuthEndpoint struct {
	// Enabled indicates whether the local cluster auth endpoint should be enabled.
	Enabled bool `json:"enabled,omitempty"`
	// FQDN is the fully qualified domain name of the local cluster auth endpoint.
	FQDN string `json:"fqdn,omitempty"`
	// CACerts is the CA certificate for the local cluster auth endpoint.
	CACerts string `json:"caCerts,omitempty"`
}

type RKESystemConfig struct {
	// MachineLabelSelector is a label selector that is used to match machines.
	// An empty label selector matches all machines.
	// +optional
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`

	// Config is a map of distro arguments which will be copied to /etc/rancher/<rke2/k3s>/config.yaml.d/50-rancher.yaml if the machine matches the label selector.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Config GenericMap `json:"config,omitempty" wrangler:"nullable"`
}

type RKEProvisioningFiles struct {
	// MachineLabelSelector is a label selector used to match machines.
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`

	// FileSources is a list of file sources that will be copied to the machine if the machine matches the label selector.
	FileSources []ProvisioningFileSource `json:"fileSources,omitempty"`
}

type ClusterUpgradeStrategy struct {
	// ControlPlaneConcurrency is the number of server nodes that should be upgraded at a time.
	// The default value is 1, a 0 value is infinite. Percentages are also accepted.
	ControlPlaneConcurrency  string       `json:"controlPlaneConcurrency,omitempty"`
	ControlPlaneDrainOptions DrainOptions `json:"controlPlaneDrainOptions,omitempty"`

	// WorkerConcurrency is the number of worker nodes that should be upgraded at a time.
	// The default value is 1, a 0 value is infinite. Percentages are also accepted.
	WorkerConcurrency  string       `json:"workerConcurrency,omitempty"`
	WorkerDrainOptions DrainOptions `json:"workerDrainOptions,omitempty"`
}

// DrainOptions contains the drain configuration for a machine pool.
type DrainOptions struct {
	// Enabled specifies whether draining is required for the machine pool before upgrading.
	Enabled bool `json:"enabled"`

	// Force specifies whether to drain the node even if there are pods not managed by a ReplicationController, Job, or DaemonSet.
	// Drain will not proceed without Force set to true if there are such pods.
	Force bool `json:"force"`

	// IgnoreDaemonSets specifies whether to ignore DaemonSet-managed pods.
	// If there are DaemonSet-managed pods, drain will not proceed without IgnoreDaemonSets set to true
	// (even when set to true, kubectl won't delete pods - so setting default to true)
	IgnoreDaemonSets *bool `json:"ignoreDaemonSets"`

	// IgnoreErrors Ignore errors occurred between drain nodes in group
	// NOTE: currently unimplemented
	// +optional
	IgnoreErrors bool `json:"ignoreErrors"`

	// Continue even if there are pods using emptyDir
	DeleteEmptyDirData bool `json:"deleteEmptyDirData"`

	// DisableEviction forces drain to use delete rather than evict
	DisableEviction bool `json:"disableEviction"`

	// GracePeriod is the period of time in seconds given to each pod to terminate gracefully.
	// If negative, the default value specified in the pod will be used.
	GracePeriod int `json:"gracePeriod"`

	// Time to wait (in seconds) before giving up for one try
	Timeout int `json:"timeout"`

	// SkipWaitForDeleteTimeoutSeconds If pod DeletionTimestamp older than N seconds, skip waiting for the pod.  Seconds must be greater than 0 to skip.
	SkipWaitForDeleteTimeoutSeconds int `json:"skipWaitForDeleteTimeoutSeconds"`

	// PreDrainHooks is a list of hooks to run prior to draining a node
	// +optional
	PreDrainHooks []DrainHook `json:"preDrainHooks,omitempty"`
	// PostDrainHooks is a list of hooks to run after draining AND UPDATING a node
	// +optional
	PostDrainHooks []DrainHook `json:"postDrainHooks,omitempty"`
}

type DrainHook struct {
	// Annotation that will need to be populated on the machine-plan secret with the value from the annotation
	// "rke.cattle.io/pre-drain" before the planner will continue with drain the specific node. The annotation
	// "rke.cattle.io/pre-drain" is used for pre-drain and "rke.cattle.io/post-drain" is used for post-drain.
	Annotation string `json:"annotation,omitempty"`
}

type ProvisioningFileSource struct {
	// +optional
	Secret K8sObjectFileSource `json:"secret,omitempty"`

	// +optional
	ConfigMap K8sObjectFileSource `json:"configMap,omitempty"`
}

type K8sObjectFileSource struct {
	Name string `json:"name"`

	// +optional
	// +kubebuilder:validation:MinItems=1
	Items []KeyToPath `json:"items,omitempty"`

	// +optional
	DefaultPermissions string `json:"defaultPermissions,omitempty"`
}

type KeyToPath struct {
	Key string `json:"key"`

	Path string `json:"path"`

	// +optional
	Dynamic bool `json:"dynamic,omitempty"`

	// +optional
	Permissions string `json:"permissions,omitempty"`

	// +optional
	Hash string `json:"hash,omitempty"`
}

const (
	AuthConfigSecretType = "rke.cattle.io/auth-config"

	UsernameAuthConfigSecretKey      = "username"
	PasswordAuthConfigSecretKey      = "password"
	AuthAuthConfigSecretKey          = "auth"
	IdentityTokenAuthConfigSecretKey = "identityToken"
)

// Registry is registry settings configured
type Registry struct {
	// Mirrors are namespace to mirror mapping for all namespaces.
	// +optional
	Mirrors map[string]Mirror `json:"mirrors,omitempty"`
	// Configs are configs for each registry.
	// The key is the FDQN or IP of the registry.
	// +optional
	Configs map[string]RegistryConfig `json:"configs,omitempty"`
}

// Mirror contains the config related to the registry mirror
type Mirror struct {
	// Endpoints are endpoints for a namespace. CRI plugin will try the endpoints
	// one by one until a working one is found. The endpoint must be a valid url
	// with host specified.
	// The scheme, host, and path from the endpoint URL will be used.
	Endpoints []string `json:"endpoint,omitempty"`

	// Rewrites are repository rewrite rules for a namespace. When fetching image resources
	// from an endpoint and a key matches the repository via regular expression matching
	// it will be replaced with the corresponding value from the map in the resource request.
	Rewrites map[string]string `json:"rewrite,omitempty"`
}

// RegistryConfig contains configuration used to communicate with the registry.
type RegistryConfig struct {
	// Auth contains information to authenticate to the registry.
	// +optional
	AuthConfigSecretName string `json:"authConfigSecretName,omitempty"`
	// TLSSecretName is a pair of Cert/Key which are used when creating the transport
	// that communicates with the registry.
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`
	// CABundle
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// InsecureSkipVerify
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

type ETCDSnapshotCreate struct {
	// Changing the Generation is the only thing required to initiate a snapshot creation.
	// +optional
	Generation int `json:"generation,omitempty"`
}

type ETCDSnapshotRestore struct {
	// Name refers to the name of the associated etcdsnapshot object.

	Name string `json:"name,omitempty"`

	// Changing the Generation is the only thing required to initiate a snapshot restore.
	// +optional
	Generation int `json:"generation,omitempty"`
	// Set to either none (or empty string), all, or kubernetesVersion
	// +kubebuilder:validation:Enum=none;all;kubernetesVersion
	// +optional
	RestoreRKEConfig string `json:"restoreRKEConfig,omitempty"`
}

type RotateCertificates struct {
	// Generation defines the current desired generation of certificate rotation.
	// Setting the generation to a different value than the current generation will trigger a rotation.
	// +optional
	Generation int64 `json:"generation,omitempty"`
	// Services is a list of services to rotate certificates for.
	// If the list is empty, all services will be rotated.
	// +optional
	Services []string `json:"services,omitempty"`
}

type RotateEncryptionKeys struct {
	// Generation defines the current desired generation of encryption key rotation.
	// Setting the generation to a different value than the current generation will trigger a rotation.
	// +optional
	Generation int64 `json:"generation,omitempty"`
}

type RKEControlPlaneStatus struct {
	AppliedSpec                   *RKEControlPlaneSpec                `json:"appliedSpec,omitempty"`
	Conditions                    []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready                         bool                                `json:"ready,omitempty"`
	ObservedGeneration            int64                               `json:"observedGeneration"`
	CertificateRotationGeneration int64                               `json:"certificateRotationGeneration"`
	RotateEncryptionKeys          *RotateEncryptionKeys               `json:"rotateEncryptionKeys,omitempty"`
	RotateEncryptionKeysPhase     RotateEncryptionKeysPhase           `json:"rotateEncryptionKeysPhase,omitempty"`
	RotateEncryptionKeysLeader    string                              `json:"rotateEncryptionKeysLeader,omitempty"`
	ETCDSnapshotRestore           *ETCDSnapshotRestore                `json:"etcdSnapshotRestore,omitempty"`
	ETCDSnapshotRestorePhase      ETCDSnapshotPhase                   `json:"etcdSnapshotRestorePhase,omitempty"`
	ETCDSnapshotCreate            *ETCDSnapshotCreate                 `json:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotCreatePhase       ETCDSnapshotPhase                   `json:"etcdSnapshotCreatePhase,omitempty"`
	ConfigGeneration              int64                               `json:"configGeneration,omitempty"`
	Initialized                   bool                                `json:"initialized,omitempty"`
	AgentConnected                bool                                `json:"agentConnected,omitempty"`
}

type ETCDSnapshotPhase string

const (
	ETCDSnapshotPhaseStarted                ETCDSnapshotPhase = "Started"
	ETCDSnapshotPhaseShutdown               ETCDSnapshotPhase = "Shutdown"
	ETCDSnapshotPhaseRestore                ETCDSnapshotPhase = "Restore"
	ETCDSnapshotPhasePostRestorePodCleanup  ETCDSnapshotPhase = "PostRestorePodCleanup"
	ETCDSnapshotPhaseInitialRestartCluster  ETCDSnapshotPhase = "InitialRestartCluster"
	ETCDSnapshotPhasePostRestoreNodeCleanup ETCDSnapshotPhase = "PostRestoreNodeCleanup"
	ETCDSnapshotPhaseRestartCluster         ETCDSnapshotPhase = "RestartCluster"
	ETCDSnapshotPhaseFinished               ETCDSnapshotPhase = "Finished"
	ETCDSnapshotPhaseFailed                 ETCDSnapshotPhase = "Failed"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKEControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RKEControlPlaneSpec   `json:"spec"`
	Status RKEControlPlaneStatus `json:"status,omitempty"`
}
