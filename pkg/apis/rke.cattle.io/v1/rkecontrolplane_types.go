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

// EnvVar represents a key value pair for an environment variable.
type EnvVar struct {
	// Name is the name of the environment variable.
	Name string `json:"name,omitempty"`

	// Value is the value of the environment variable.
	Value string `json:"value,omitempty"`
}

type RKEControlPlaneSpec struct {
	RKEClusterSpecCommon `json:",inline"`

	// AgentEnvVars is a list of environment variables that will be set on the cluster agent deployment and system agent service.
	// +optional
	AgentEnvVars []EnvVar `json:"agentEnvVars,omitempty"`

	// LocalClusterAuthEndpoint is the configuration for the local cluster auth endpoint.
	// +optional
	// +nullable
	LocalClusterAuthEndpoint LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty"`

	// ETCDSnapshotCreate is the configuration for the etcd snapshot creation operation.
	// +optional
	ETCDSnapshotCreate *ETCDSnapshotCreate `json:"etcdSnapshotCreate,omitempty"`

	// ETCDSnapshotRestore is the configuration for the etcd snapshot restore operation.
	// +optional
	ETCDSnapshotRestore *ETCDSnapshotRestore `json:"etcdSnapshotRestore,omitempty"`

	// RotateCertificates is the configuration for the certificate rotation operation.
	// +optional
	RotateCertificates *RotateCertificates `json:"rotateCertificates,omitempty"`

	// RotateEncryptionKeys is the configuration for the encryption key rotation operation.
	// +optional
	RotateEncryptionKeys *RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty"`

	// KubernetesVersion is the desired version of RKE2/K3s for the cluster.
	// This field is only populated for provisioned and custom clusters.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// ClusterName is the name of the provisioning cluster object.
	// +kubebuilder:validation:MaxLength=63
	// +required
	ClusterName string `json:"clusterName,omitempty"`

	// ManagementClusterName is the name of the management cluster object that relates to this cluster.
	// +kubebuilder:validation:MaxLength=63
	// +required
	ManagementClusterName string `json:"managementClusterName,omitempty"`

	// UnamanagedConfig indicates whether the configuration files for this cluster are managed by Rancher or externally.
	UnmanagedConfig bool `json:"unmanagedConfig,omitempty"`
}

type NetworkingStackPreference = string

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
	// StackPreference specifies which networking stack to prefer for external cluster communication. In practice, this is used by the
	// planner to render the various probes to force IPv4, IPv6, or default to localhost. There is currently no
	// sanitization or validation as cluster configuration can be specified with machineGlobalConfig and
	// machineSelectorConfig, which although easy to instrument to determine a potential interface, user defined
	// configuration can be specified in the `/etc/rancher/<rke2/k3s>/config.yaml.d` directory either manually or via
	// cloud-init, and there is currently no mechanism to extract the completely rendered configuration via the planner
	// nor various engines themselves.
	// +kubebuilder:validation:Enum=ipv4;ipv6;dual
	// +optional
	StackPreference string `json:"stackPreference,omitempty"`
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
	// UpgradeStrategy contains the concurrency and drain configuration to be used when upgrading machine pools of servers and agents.
	// +optional
	UpgradeStrategy ClusterUpgradeStrategy `json:"upgradeStrategy,omitempty"`

	// ChartValues is a map whose keys correspond to charts to be installed by the distro, with values corresponding
	// to the helm values configurable in the chart.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ChartValues GenericMap `json:"chartValues,omitempty"`

	// MachineGlobalConfig is a list of distro arguments which will be copied to
	// /etc/rancher/<rke2/k3s>/config.yaml.d/50-rancher.yaml for all machines.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	MachineGlobalConfig GenericMap `json:"machineGlobalConfig,omitempty"`

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
	// Resources created as additional manifests will be deleted if removed from additional manifests.
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

// +kubebuilder:validation:XValidation:rule="!self.enabled || has(self.fqdn) || !has(self.caCerts)",message="CACerts defined but FQDN is not defined"

type LocalClusterAuthEndpoint struct {
	// Enabled indicates whether the local cluster auth endpoint should be enabled.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// FQDN is the fully qualified domain name of the local cluster auth endpoint.
	// +kubebuilder:validation:MaxLength=255
	// +optional
	FQDN string `json:"fqdn,omitempty"`

	// CACerts is the CA certificate for the local cluster auth endpoint.
	// +optional
	CACerts string `json:"caCerts,omitempty"`
}

type RKESystemConfig struct {
	// MachineLabelSelector is a label selector used to match machines.
	// An empty/null label selector matches all machines.
	// +optional
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`

	// Config is a map of distro arguments which will be copied to /etc/rancher/<rke2/k3s>/config.yaml.d/50-rancher.yaml if the machine matches the label selector.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	// +nullable
	Config GenericMap `json:"config,omitempty"`
}

type RKEProvisioningFiles struct {
	// MachineLabelSelector is a label selector used to match machines.
	// An empty/null label selector matches all machines.
	// +optional
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`

	// FileSources is a list of file sources that will be copied to the machine if the machine matches the label selector.
	FileSources []ProvisioningFileSource `json:"fileSources,omitempty"`
}

type ClusterUpgradeStrategy struct {
	// ControlPlaneConcurrency is the number of server nodes that should be upgraded at a time.
	// The default value is 1, a 0 value is infinite. Percentages are also accepted.
	ControlPlaneConcurrency string `json:"controlPlaneConcurrency,omitempty"`

	// ControlPlaneDrainOptions is the drain configuration to be used when draining controlplane nodes, during both upgrades and machine rollouts.
	// +optional
	ControlPlaneDrainOptions DrainOptions `json:"controlPlaneDrainOptions,omitempty"`

	// WorkerConcurrency is the number of worker nodes that should be upgraded at a time.
	// The default value is 1, a 0 value is infinite. Percentages are also accepted.
	WorkerConcurrency string `json:"workerConcurrency,omitempty"`

	// WorkerDrainOptions is the drain configuration to be used when draining worker nodes, during both upgrades and machine rollouts.
	// +optional
	WorkerDrainOptions DrainOptions `json:"workerDrainOptions,omitempty"`
}

// DrainOptions contains the drain configuration for a machine pool.
type DrainOptions struct {
	// Enabled specifies whether draining is required for the machine pool before upgrading.
	Enabled bool `json:"enabled"`

	// Force specifies whether to drain the node even if there are pods not managed by a ReplicationController, Job, or DaemonSet.
	// Drain will not proceed without Force set to true if there are such pods.
	// +kubebuilder:default=false
	Force bool `json:"force"`

	// IgnoreDaemonSets specifies whether to ignore DaemonSet-managed pods.
	// If there are DaemonSet-managed pods, drain will not proceed without IgnoreDaemonSets set to true
	// (even when set to true, kubectl won't delete pods - so setting default to true)
	// +kubebuilder:default=true
	// +optional
	IgnoreDaemonSets bool `json:"ignoreDaemonSets,omitempty"`

	// IgnoreErrors Ignore errors occurred between drain nodes in group
	// NOTE: currently unimplemented
	// +optional
	IgnoreErrors bool `json:"ignoreErrors,omitempty"`

	// DeleteEmptyDirData instructs the drain operation to proceed even if there are pods using emptyDir
	DeleteEmptyDirData bool `json:"deleteEmptyDirData"`

	// DisableEviction forces drain to use delete rather than evict
	DisableEviction bool `json:"disableEviction"`

	// GracePeriod is the period of time in seconds given to each pod to terminate gracefully.
	// If negative, the default value specified in the pod will be used.
	GracePeriod int `json:"gracePeriod"`

	// Time to wait (in seconds) before giving up for one try
	// +kubebuilder:validation:Minimum=0
	Timeout int `json:"timeout"`

	// SkipWaitForDeleteTimeoutSeconds If pod DeletionTimestamp older than N seconds, skip waiting for the pod. Seconds must be greater than 0 to skip.
	// +kubebuilder:validation:Minimum=0
	SkipWaitForDeleteTimeoutSeconds int `json:"skipWaitForDeleteTimeoutSeconds"`

	// PreDrainHooks is a list of hooks to run before draining a node
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
	// +kubebuilder:validation:MaxLength=63
	Annotation string `json:"annotation,omitempty"`
}

type ProvisioningFileSource struct {
	// Secret is the configuration for mapping a secret containing arbitrary data to a series of files on the system-agent host.
	// +optional
	Secret K8sObjectFileSource `json:"secret,omitempty"`

	// ConfigMap is the configuration for mapping a configmap containing arbitrary data to a series of files on the system-agent host.
	// +optional
	ConfigMap K8sObjectFileSource `json:"configMap,omitempty"`
}

type K8sObjectFileSource struct {
	// Name is the name of the resource.
	// The namespace is required to be the same as the related RKEControlPlane object.
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Items is a list of mappings from the keys within the resource to the files to create on the downstream machine.
	// +optional
	// +kubebuilder:validation:MinItems=1
	Items []KeyToPath `json:"items,omitempty"`

	// DefaultPermissions provides a fallback value for all files within the configmap/secret.
	// +optional
	DefaultPermissions string `json:"defaultPermissions,omitempty"`
}

type KeyToPath struct {
	// Key is the key used to index the associated configmap or secret.
	Key string `json:"key"`

	// Path is the absolute path the data within the configmap or secret should be written to by the system-agent.
	Path string `json:"path"`

	// Dynamic indicates whether the rendered file should be included when calculating the restart stamp
	// i.e. whether changes to this resource should trigger draining when reconciling.
	// +optional
	Dynamic bool `json:"dynamic,omitempty"`

	// Permissions specifies the desired permissions for this file on the machine's filesystem.
	// +optional
	Permissions string `json:"permissions,omitempty"`

	// Hash is used to ensure that the data within the configmap or secret matches the expected sha256sum of the value at the provided key.
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
	// AuthConfigSecretName contains information to authenticate to the registry.
	// The accepted keys are as follows:
	// - username
	// - password
	// - auth
	// - identityToken
	// +kubebuilder:validation:MaxLength=63
	// +optional
	AuthConfigSecretName string `json:"authConfigSecretName,omitempty"`

	// TLSSecretName is the name of the secret residing within the same namespace as the RKEControlPlane object
	// that contains the keys "Cert" and "Key" which are used when creating the transport
	// that communicates with the registry.
	// +kubebuilder:validation:MaxLength=63
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`

	// CABundle is the CA chain used when communicating with the image registry.
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// InsecureSkipVerify indicates whether validation of the server's certificate should be skipped.
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
	// AppliedSpec is the state for which the last reconciliation loop for the controlplane was completed.
	// +optional
	AppliedSpec *RKEControlPlaneSpec `json:"appliedSpec,omitempty"`

	// Conditions is a representation of the current state of the RKEControlPlane object,
	// this includes its machine reconciliation status (Bootstrapped, Provisioned, Stable, Reconciled),
	// the status of the system-upgrade-controller (SystemUpgradeControllerReady), and CAPI required conditions
	// (ScalingUp, ScalingDown, RollingOut). Information related to
	// errors encountered while transitioning to one of these states will be
	// populated in the Message and Reason fields.
	// +optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	// Ready denotes that the API server has been initialized and is ready to receive requests.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// ObservedGeneration is the generation for which the RKEControlPlane has started processing.
	ObservedGeneration int64 `json:"observedGeneration"`

	// CertificateRotationGeneration is the last observed state for which the certificate rotation operation was successful.
	// +optional
	CertificateRotationGeneration int64 `json:"certificateRotationGeneration,omitempty"`

	// RotateEncryptionKeys is the state for which the last encryption key rotation operation was successful.
	// +optional
	RotateEncryptionKeys *RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty"`

	// RotateEncryptionKeysPhase is the current phase the encryption key rotation operation is currently executing.
	// +optional
	RotateEncryptionKeysPhase RotateEncryptionKeysPhase `json:"rotateEncryptionKeysPhase,omitempty"`

	// RotateEncryptionKeysLeader is the name of the CAPI machine object which has been elected leader of the controlplane nodes for
	// encryption key rotation purposes.
	// +optional
	RotateEncryptionKeysLeader string `json:"rotateEncryptionKeysLeader,omitempty"`

	// ETCDSnapshotRestore is the state for which the last etcd snapshot restore operation was successful.
	// +optional
	ETCDSnapshotRestore *ETCDSnapshotRestore `json:"etcdSnapshotRestore,omitempty"`

	// ETCDSnapshotRestorePhase is the current phase the etcd snapshot restore operation is currently executing.
	// +kubebuilder:validation:Enum=Started;Shutdown;Restore;PostRestorePodCleanup;InitialRestartCluster;PostRestoreNodeCleanup;RestartCluster;Finished;Failed
	// +optional
	ETCDSnapshotRestorePhase ETCDSnapshotPhase `json:"etcdSnapshotRestorePhase,omitempty"`

	// ETCDSnapshotCreate is the state for which the last etcd snapshot create operation was successful.
	// +optional
	ETCDSnapshotCreate *ETCDSnapshotCreate `json:"etcdSnapshotCreate,omitempty"`

	// ETCDSnapshotCreatePhase is the current phase the etcd snapshot create operation is currently executing.
	// +kubebuilder:validation:Enum=Started;RestartCluster;Finished;Failed
	// +optional
	ETCDSnapshotCreatePhase ETCDSnapshotPhase `json:"etcdSnapshotCreatePhase,omitempty"`

	// ConfigGeneration is the current generation of the configuration for a given cluster.
	// Changing this value (which is done automatically during an etcd restore) will trigger a reconciliation loop
	// which will invoke draining (if enabled).
	// +optional
	ConfigGeneration int64 `json:"configGeneration,omitempty"`

	// Initialized denotes that the API server is initialized and worker nodes can be joined to the cluster.
	// +optional
	Initialized bool `json:"initialized,omitempty"`

	// AgentConnected denotes that the cluster-agent connection is currently established for the cluster.
	// +optional
	AgentConnected bool `json:"agentConnected,omitempty"`
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
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="cluster.x-k8s.io/v1beta1=v1"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Initialized",type=string,JSONPath=".status.initialized",description="This denotes whether or not the control plane is initialized"
// +kubebuilder:printcolumn:name="API Server Available",type=boolean,JSONPath=".status.ready",description="RKEControlPlane API Server is ready to receive requests"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp",description="Time duration since creation of RKEControlPlane"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.kubernetesVersion",description="Kubernetes version associated with this control plane"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RKEControlPlane is the Schema for the controlplane.
type RKEControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the controlplane.
	// +optional
	Spec RKEControlPlaneSpec `json:"spec"`

	// Status is the observed state of the controlplane.
	// +optional
	Status RKEControlPlaneStatus `json:"status,omitempty"`
}
