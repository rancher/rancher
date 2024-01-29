package v1

import (
	"github.com/rancher/wrangler/v2/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKECluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RKEClusterSpec   `json:"spec"`
	Status            RKEClusterStatus `json:"status,omitempty"`
}

type RKEClusterStatus struct {
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready      bool                                `json:"ready,omitempty"`
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

type RKEClusterSpecCommon struct {
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

	// Increment to force all nodes to re-provision
	ProvisionGeneration int `json:"provisionGeneration,omitempty"`
}

type LocalClusterAuthEndpoint struct {
	Enabled bool   `json:"enabled,omitempty"`
	FQDN    string `json:"fqdn,omitempty"`
	CACerts string `json:"caCerts,omitempty"`
}

type RKESystemConfig struct {
	MachineLabelSelector *metav1.LabelSelector `json:"machineLabelSelector,omitempty"`
	Config               GenericMap            `json:"config,omitempty" wrangler:"nullable"`
}

type RKEProvisioningFiles struct {
	MachineLabelSelector *metav1.LabelSelector    `json:"machineLabelSelector,omitempty"`
	FileSources          []ProvisioningFileSource `json:"fileSources,omitempty"`
}

type RKEClusterSpec struct {
	// Not used in anyway, just here to make cluster-api happy
	ControlPlaneEndpoint *capi.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
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
