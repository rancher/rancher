package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterConfiguration struct {
	// UpgradeStrategy contains the concurrency and drain configuration to be
	// used when upgrading machine pools of servers and agents.
	// +optional
	UpgradeStrategy ClusterUpgradeStrategy `json:"upgradeStrategy,omitempty"`

	// ChartValues is a map whose keys correspond to charts to be installed
	// by the distro, with values corresponding to the helm values
	// configurable in the chart.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	// +optional
	ChartValues GenericMap `json:"chartValues,omitempty"`

	// MachineGlobalConfig is a list of distro arguments which will be copied
	// to /etc/rancher/<rke2/k3s>/config.yaml.d/50-rancher.yaml for all
	// machines.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	// +optional
	MachineGlobalConfig GenericMap `json:"machineGlobalConfig,omitempty"`

	// MachineSelectorConfig is a list of distro arguments which will be
	// copied to /etc/rancher/<rke2/k3s>/config.yaml.d/50-rancher.yaml if the
	// machine matches the label selector.
	// +nullable
	// +optional
	MachineSelectorConfig []RKESystemConfig `json:"machineSelectorConfig,omitempty"`

	// MachineSelectorFiles is a list of files which will be copied to the
	// machine if the machine matches the label selector.
	// +nullable
	// +optional
	MachineSelectorFiles []RKEProvisioningFiles `json:"machineSelectorFiles,omitempty"`

	// AdditionalManifest is a string containing a yaml blob to insert in the
	// /var/lib/rancher/<rke2/k3s>/server/manifests/rancher/addons.yaml file.
	// The distro will automatically create these resources.
	// Resources created as additional manifests will be deleted if removed
	// from additional manifests.
	// +nullable
	// +optional
	AdditionalManifest string `json:"additionalManifest,omitempty"`

	// Registries is the list of mirrors and configurations for the cluster's
	// container registries.
	// +nullable
	// +optional
	Registries *Registry `json:"registries,omitempty"`

	// ETCD contains the etcd snapshot configuration for the cluster.
	// +nullable
	// +optional
	ETCD *ETCD `json:"etcd,omitempty"`

	// Networking contains information regarding the desired networking stack
	// of the cluster.
	// +nullable
	// +optional
	Networking *Networking `json:"networking,omitempty"`

	// DataDirectories contains the configuration for the data directories
	// typically stored within /var/lib/rancher. The data directories must be
	// configured via the provisioning cluster object and are immutable once
	// set.
	// +optional
	DataDirectories DataDirectories `json:"dataDirectories,omitempty"`

	// ProvisionGeneration is used to force the planner to reconcile the
	// cluster, regardless of whether a reconciliation is required.
	// +optional
	ProvisionGeneration int `json:"provisionGeneration,omitempty"`
}

type ClusterUpgradeStrategy struct {
	// ControlPlaneConcurrency is the number of server nodes that should be
	// upgraded at a time.
	// The default value is 1, a 0 value is infinite.
	// Percentages are also accepted.
	// +kubebuilder:validation:Pattern="^((([1-9]|[1-9][0-9]|100)%)|([1-9][0-9]*|0)|)$"
	// +kubebuilder:validation:MaxLength=10
	// +nullable
	// +optional
	ControlPlaneConcurrency string `json:"controlPlaneConcurrency,omitempty"`

	// ControlPlaneDrainOptions is the drain configuration to be used when
	// draining controlplane nodes, during both upgrades and machine
	// rollouts.
	// +optional
	ControlPlaneDrainOptions DrainOptions `json:"controlPlaneDrainOptions,omitempty"`

	// WorkerConcurrency is the number of worker nodes that should be
	// upgraded at a time.
	// The default value is 1, a 0 value is infinite.
	// Percentages are also accepted.
	// +kubebuilder:validation:Pattern="^((([1-9]|[1-9][0-9]|100)%)|([1-9][0-9]*|0)|)$"
	// +kubebuilder:validation:MaxLength=10
	// +nullable
	// +optional
	WorkerConcurrency string `json:"workerConcurrency,omitempty"`

	// WorkerDrainOptions is the drain configuration to be used when draining
	// worker nodes, during both upgrades and machine rollouts.
	// +optional
	WorkerDrainOptions DrainOptions `json:"workerDrainOptions,omitempty"`
}

// DrainOptions contains the drain configuration for a machine pool.
type DrainOptions struct {
	// Enabled specifies whether draining is required for the machine pool
	// before upgrading.
	// +optional
	Enabled bool `json:"enabled"`

	// Force specifies whether to drain the node even if there are pods not
	// managed by a ReplicationController, Job, or DaemonSet.
	// Drain will not proceed without Force set to true if there are such
	// pods.
	// +optional
	Force bool `json:"force"`

	// IgnoreDaemonSets specifies whether to ignore DaemonSet-managed pods.
	// If there are DaemonSet-managed pods, drain will not proceed without
	// IgnoreDaemonSets set to true (even when set to true, kubectl won't
	// delete pods - so an unset value will default to true).
	// +nullable
	// +optional
	IgnoreDaemonSets *bool `json:"ignoreDaemonSets,omitempty"`

	// IgnoreErrors Ignore errors occurred between drain nodes in group
	// NOTE: currently unimplemented
	// +optional
	IgnoreErrors bool `json:"ignoreErrors,omitempty"`

	// DeleteEmptyDirData instructs the drain operation to proceed even if
	// there are pods using emptyDir.
	// +optional
	DeleteEmptyDirData bool `json:"deleteEmptyDirData"`

	// DisableEviction forces drain to use delete rather than evict.
	// +optional
	DisableEviction bool `json:"disableEviction"`

	// GracePeriod is the period of time in seconds given to each pod to
	// terminate gracefully.
	// If negative, the default value specified in the pod will be used.
	// +optional
	GracePeriod int `json:"gracePeriod"`

	// Timeout is the time to wait (in seconds) before giving up for one try.
	// +optional
	Timeout int `json:"timeout"`

	// SkipWaitForDeleteTimeoutSeconds defines how long the draining
	// operation should wait for a given to be removed after deletion.
	// If the pod's DeletionTimestamp is older than N seconds, the drain
	// operation will move on.
	// Seconds must be greater than 0 to skip.
	// +optional
	SkipWaitForDeleteTimeoutSeconds int `json:"skipWaitForDeleteTimeoutSeconds"`

	// PreDrainHooks is a list of hooks to run before draining a node.
	// +nullable
	// +optional
	PreDrainHooks []DrainHook `json:"preDrainHooks,omitempty"`

	// PostDrainHooks is a list of hooks to run after draining and updating
	// a node.
	// +nullable
	// +optional
	PostDrainHooks []DrainHook `json:"postDrainHooks,omitempty"`
}

type DrainHook struct {
	// Annotation that will need to be populated on the machine-plan secret
	// with the value from the annotation "rke.cattle.io/pre-drain" before
	// the planner will continue to drain the specific node.
	// The annotation "rke.cattle.io/pre-drain" is used for pre-drain and
	// "rke.cattle.io/post-drain" is used for post-drain.
	// +kubebuilder:validation:MaxLength=317
	// +nullable
	// +optional
	Annotation string `json:"annotation,omitempty"`
}

type RKESystemConfig struct {
	// MachineLabelSelector is a label selector used to match machines.
	// An empty/null label selector matches all machines.
	// +nullable
	// +optional
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`

	// Config is a map of distro arguments which will be copied to
	// /etc/rancher/<rke2/k3s>/config.yaml.d/50-rancher.yaml if the machine
	// matches the label selector.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	// +optional
	Config GenericMap `json:"config,omitempty"`
}

type RKEProvisioningFiles struct {
	// MachineLabelSelector is a label selector used to match machines.
	// An empty/null label selector matches all machines.
	// +nullable
	// +optional
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`

	// FileSources is a list of file sources that will be copied to the
	// machine if the machine matches the label selector.
	// +nullable
	// +optional
	FileSources []ProvisioningFileSource `json:"fileSources,omitempty"`
}

type ProvisioningFileSource struct {
	// Secret is the configuration for mapping a secret containing arbitrary
	// data to a series of files on the system-agent host.
	// +optional
	Secret K8sObjectFileSource `json:"secret,omitempty"`

	// ConfigMap is the configuration for mapping a configmap containing
	// arbitrary data to a series of files on the system-agent host.
	// +optional
	ConfigMap K8sObjectFileSource `json:"configMap,omitempty"`
}

type K8sObjectFileSource struct {
	// Name is the name of the resource.
	// The namespace is required to be the same as the related
	// RKEControlPlane object.
	// +kubebuilder:validation:MaxLength=253
	// +nullable
	// +required
	Name string `json:"name"`

	// Items is a list of mappings from the keys within the resource to the
	// files to create on the downstream machine.
	// +nullable
	// +optional
	Items []KeyToPath `json:"items,omitempty"`

	// DefaultPermissions provides a fallback value for all files within the
	// configmap/secret.
	// +nullable
	// +optional
	DefaultPermissions string `json:"defaultPermissions,omitempty"`
}

type KeyToPath struct {
	// Key is the key used to index the associated configmap or secret.
	// +nullable
	Key string `json:"key"`

	// Path is the absolute path the data within the configmap or secret
	// should be written to by the system-agent.
	// +nullable
	// +required
	Path string `json:"path"`

	// Dynamic indicates whether the rendered file should be included when
	// calculating the restart stamp i.e. whether changes to this resource
	// should trigger draining when reconciling.
	// +optional
	Dynamic bool `json:"dynamic,omitempty"`

	// Permissions specifies the desired permissions for this file on the
	// machine's filesystem.
	// +nullable
	// +optional
	Permissions string `json:"permissions,omitempty"`

	// Hash is used to ensure that the data within the configmap or secret
	// matches the expected sha256sum of the value at the provided key.
	// +nullable
	// +optional
	Hash string `json:"hash,omitempty"`
}

type Registry struct {
	// Mirrors are namespace to mirror mapping for all namespaces.
	// +nullable
	// +optional
	Mirrors map[string]Mirror `json:"mirrors,omitempty"`

	// Configs are configs for each registry.
	// The key is the FDQN or IP of the registry.
	// +nullable
	// +optional
	Configs map[string]RegistryConfig `json:"configs,omitempty"`
}

// Mirror contains the config related to the registry mirror
type Mirror struct {
	// Endpoints are endpoints for a namespace. CRI plugin will try the
	// endpoints one by one until a working one is found.
	// The endpoint must be a valid url with host specified.
	// The scheme, host, and path from the endpoint URL will be used.
	// +nullable
	// +optional
	Endpoints []string `json:"endpoint,omitempty"`

	// Rewrites are repository rewrite rules for a Mirror.
	// When fetching image resources from a registry, a regular expression
	// can be used to match the image name and modify it using
	// the corresponding value from the map in the resource request.
	// +nullable
	// +optional
	Rewrites map[string]string `json:"rewrite,omitempty"`
}

// RegistryConfig contains configuration used to communicate with the registry.
type RegistryConfig struct {
	// AuthConfigSecretName contains information to authenticate to the
	// registry.
	// The accepted keys are as follows:
	// - username
	// - password
	// - auth
	// - identityToken
	// +kubebuilder:validation:MaxLength=253
	// +nullable
	// +optional
	AuthConfigSecretName string `json:"authConfigSecretName,omitempty"`

	// TLSSecretName is the name of the secret residing within the same
	// namespace as the RKEControlPlane object that contains the keys "Cert"
	// and "Key" which are used when creating the transport that communicates
	// with the registry.
	// +kubebuilder:validation:MaxLength=253
	// +nullable
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`

	// CABundle is the CA chain used when communicating with the image
	// registry.
	// +nullable
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// InsecureSkipVerify indicates whether validation of the server's
	// certificate should be skipped.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

type ETCD struct {
	// DisableSnapshots disables the creation of snapshots for the cluster.
	// +optional
	DisableSnapshots bool `json:"disableSnapshots,omitempty"`

	// SnapshotScheduleCron is the cron schedule for the snapshot creation.
	// +nullable
	// +optional
	SnapshotScheduleCron string `json:"snapshotScheduleCron,omitempty"`

	// SnapshotRetention is the number of snapshots the downstream cluster
	// should retain per snapshot generation.
	// +optional
	SnapshotRetention int `json:"snapshotRetention,omitempty"`

	// S3 defines the S3 configuration for the cluster if enabled.
	// +nullable
	// +optional
	S3 *ETCDSnapshotS3 `json:"s3,omitempty"`
}

// Networking contains information regarding the desired and actual networking stack of the cluster.
type Networking struct {
	// StackPreference specifies which networking stack to prefer for
	// external cluster communication.
	// In practice, this is used by the planner to render the various probes
	// to force IPv4, IPv6, or default to localhost.
	// There is currently no sanitization or validation as cluster
	// configuration can be specified with machineGlobalConfig and
	// machineSelectorConfig, which although easy to instrument to determine
	// a potential interface, user defined configuration can be specified in
	// the `/etc/rancher/<rke2/k3s>/config.yaml.d` directory either manually
	// or via cloud-init, and there is currently no mechanism to extract the
	// completely rendered configuration via the planner nor various engines
	// themselves.
	// +nullable
	// +optional
	StackPreference NetworkingStackPreference `json:"stackPreference,omitempty"`
}

type DataDirectories struct {
	// SystemAgent is the data directory for the system-agent connection info
	// and plans.
	// +nullable
	// +optional
	SystemAgent string `json:"systemAgent,omitempty"`

	// Provisioning is the data directory for provisioning related files
	// (e.g. idempotency).
	// +nullable
	// +optional
	Provisioning string `json:"provisioning,omitempty"`

	// K8sDistro is the data directory for the k8s distro, i.e. the data-dir
	// arg.
	// +nullable
	// +optional
	K8sDistro string `json:"k8sDistro,omitempty"`
}
