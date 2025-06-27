package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterConfiguration struct {
	UpgradeStrategy       ClusterUpgradeStrategy `json:"upgradeStrategy,omitempty"`
	ChartValues           GenericMap             `json:"chartValues,omitempty" wrangler:"nullable"`
	MachineGlobalConfig   GenericMap             `json:"machineGlobalConfig,omitempty" wrangler:"nullable"`
	MachineSelectorConfig []RKESystemConfig      `json:"machineSelectorConfig,omitempty"`
	MachineSelectorFiles  []RKEProvisioningFiles `json:"machineSelectorFiles,omitempty"`
	AdditionalManifest    string                 `json:"additionalManifest,omitempty"`
	Registries            *Registry              `json:"registries,omitempty"`
	ETCD                  *ETCD                  `json:"etcd,omitempty"`

	// Networking contains information regarding the desired and actual networking stack of the cluster.
	Networking *Networking `json:"networking,omitempty"`

	// DataDirectories contains the configuration for the data directories typically stored within /var/lib/rancher.
	DataDirectories DataDirectories `json:"dataDirectories,omitempty"`

	// Increment to force all nodes to re-provision
	ProvisionGeneration int `json:"provisionGeneration,omitempty"`
}

type ClusterUpgradeStrategy struct {
	// How many controlplane nodes should be upgrade at time, defaults to 1, 0 is infinite. Percentages are
	// accepted too.
	ControlPlaneConcurrency  string       `json:"controlPlaneConcurrency,omitempty"`
	ControlPlaneDrainOptions DrainOptions `json:"controlPlaneDrainOptions,omitempty"`

	// How many workers should be upgraded at a time
	WorkerConcurrency  string       `json:"workerConcurrency,omitempty"`
	WorkerDrainOptions DrainOptions `json:"workerDrainOptions,omitempty"`
}

type DrainOptions struct {
	// Enable will require nodes be drained before upgrade
	Enabled bool `json:"enabled"`
	// Drain node even if there are pods not managed by a ReplicationController, Job, or DaemonSet
	// Drain will not proceed without Force set to true if there are such pods
	Force bool `json:"force"`
	// If there are DaemonSet-managed pods, drain will not proceed without IgnoreDaemonSets set to true
	// (even when set to true, kubectl won't delete pods - so setting default to true)
	IgnoreDaemonSets *bool `json:"ignoreDaemonSets"`
	// IgnoreErrors Ignore errors occurred between drain nodes in group
	IgnoreErrors bool `json:"ignoreErrors"`
	// Continue even if there are pods using emptyDir
	DeleteEmptyDirData bool `json:"deleteEmptyDirData"`
	// DisableEviction forces drain to use delete rather than evict
	DisableEviction bool `json:"disableEviction"`
	// Period of time in seconds given to each pod to terminate gracefully.
	// If negative, the default value specified in the pod will be used
	GracePeriod int `json:"gracePeriod"`
	// Time to wait (in seconds) before giving up for one try
	Timeout int `json:"timeout"`
	// SkipWaitForDeleteTimeoutSeconds If pod DeletionTimestamp older than N seconds, skip waiting for the pod.  Seconds must be greater than 0 to skip.
	SkipWaitForDeleteTimeoutSeconds int `json:"skipWaitForDeleteTimeoutSeconds"`

	// PreDrainHooks A list of hooks to run prior to draining a node
	PreDrainHooks []DrainHook `json:"preDrainHooks"`
	// PostDrainHook A list of hooks to run after draining AND UPDATING a node
	PostDrainHooks []DrainHook `json:"postDrainHooks"`
}

type DrainHook struct {
	// Annotation This annotation will need to be populated on the machine-plan secret with the value from the annotation
	// "rke.cattle.io/pre-drain" before the planner will continue with drain the specific node.  The annotation
	// "rke.cattle.io/pre-drain" is used for pre-drain and "rke.cattle.io/post-drain" is used for post drain.
	Annotation string `json:"annotation,omitempty"`
}

type RKESystemConfig struct {
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`
	Config               GenericMap            `json:"config,omitempty" wrangler:"nullable"`
}

type RKEProvisioningFiles struct {
	MachineLabelSelector *metav1.LabelSelector    `json:"machineLabelSelector,omitempty"`
	FileSources          []ProvisioningFileSource `json:"fileSources,omitempty"`
}

type ProvisioningFileSource struct {
	Secret    K8sObjectFileSource `json:"secret,omitempty"`
	ConfigMap K8sObjectFileSource `json:"configMap,omitempty"`
}

type K8sObjectFileSource struct {
	Name               string      `json:"name"`
	Items              []KeyToPath `json:"items,omitempty"`
	DefaultPermissions string      `json:"defaultPermissions,omitempty"`
}

type KeyToPath struct {
	Key         string `json:"key"`
	Path        string `json:"path"`
	Dynamic     bool   `json:"dynamic,omitempty"`
	Permissions string `json:"permissions,omitempty"`
	Hash        string `json:"hash,omitempty"`
}

// Registry is registry settings configured
type Registry struct {
	// Mirrors are namespace to mirror mapping for all namespaces.
	Mirrors map[string]Mirror `json:"mirrors,omitempty"`
	// Configs are configs for each registry.
	// The key is the FDQN or IP of the registry.
	Configs map[string]RegistryConfig `json:"configs,omitempty"`
}

// Mirror contains the config related to the registry mirror
type Mirror struct {
	// Endpoints are endpoints for a namespace. CRI plugin will try the endpoints
	// one by one until a working one is found. The endpoint must be a valid url
	// with host specified.
	// The scheme, host and path from the endpoint URL will be used.
	Endpoints []string `json:"endpoint,omitempty"`

	// Rewrites are repository rewrite rules for a namespace. When fetching image resources
	// from an endpoint and a key matches the repository via regular expression matching
	// it will be replaced with the corresponding value from the map in the resource request.
	Rewrites map[string]string `json:"rewrite,omitempty"`
}

// RegistryConfig contains configuration used to communicate with the registry.
type RegistryConfig struct {
	// Auth contains information to authenticate to the registry.
	AuthConfigSecretName string `json:"authConfigSecretName,omitempty"`
	// TLS is a pair of Cert/Key which then are used when creating the transport
	// that communicates with the registry.
	TLSSecretName string `json:"tlsSecretName,omitempty"`
	CABundle      []byte `json:"caBundle,omitempty"`

	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

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
	// Data directory for the system-agent connection info and plans
	SystemAgent string `json:"systemAgent,omitempty"`
	// Data directory for provisioning related files (idempotency)
	Provisioning string `json:"provisioning,omitempty"`
	// Data directory for the k8s distro
	K8sDistro string `json:"k8sDistro,omitempty"`
}
