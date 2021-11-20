package v1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKECluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RKEClusterSpec   `json:"spec"`
	Status            RKEClusterStatus `json:"status,omitempty"`
}

type RKEClusterStatus struct {
	Conditions         []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready              bool                                `json:"ready,omitempty"`
	ObservedGeneration int64                               `json:"observedGeneration"`
}

type RKEClusterSpecCommon struct {
	UpgradeStrategy       ClusterUpgradeStrategy `json:"upgradeStrategy,omitempty"`
	ChartValues           GenericMap             `json:"chartValues,omitempty" wrangler:"nullable"`
	MachineGlobalConfig   GenericMap             `json:"machineGlobalConfig,omitempty" wrangler:"nullable"`
	MachineSelectorConfig []RKESystemConfig      `json:"machineSelectorConfig,omitempty"`
	AdditionalManifest    string                 `json:"additionalManifest,omitempty"`
	Registries            *Registry              `json:"registries,omitempty"`
	ETCD                  *ETCD                  `json:"etcd,omitempty"`
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

type RKEClusterSpec struct {
	// Not used in anyway, just here to make cluster-api happy
	ControlPlaneEndpoint *Endpoint `json:"controlPlaneEndpoint,omitempty"`
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
	Enabled bool `json:"enabled,omitempty"`
	// Drain node even if there are pods not managed by a ReplicationController, Job, or DaemonSet
	// Drain will not proceed without Force set to true if there are such pods
	Force bool `json:"force,omitempty"`
	// If there are DaemonSet-managed pods, drain will not proceed without IgnoreDaemonSets set to true
	// (even when set to true, kubectl won't delete pods - so setting default to true)
	IgnoreDaemonSets *bool `json:"ignoreDaemonSets,omitempty"`
	// IgnoreErrors Ignore errors occurred between drain nodes in group
	IgnoreErrors bool
	// Continue even if there are pods using emptyDir
	DeleteEmptyDirData bool `json:"deleteEmptyDirData,omitempty"`
	// DisableEviction forces drain to use delete rather than evict
	DisableEviction bool `json:"disableEviction,omitempty"`
	//Period of time in seconds given to each pod to terminate gracefully.
	// If negative, the default value specified in the pod will be used
	GracePeriod int `json:"gracePeriod,omitempty"`
	// Time to wait (in seconds) before giving up for one try
	Timeout int `json:"timeout"`
	// SkipWaitForDeleteTimeoutSeconds If pod DeletionTimestamp older than N seconds, skip waiting for the pod.  Seconds must be greater than 0 to skip.
	SkipWaitForDeleteTimeoutSeconds int `json:"skipWaitForDeleteTimeoutSeconds,omitempty"`

	// PreDrainHooks A list of hooks to run prior to draining a node
	PreDrainHooks []DrainHook `json:"preDrainHooks,omitempty"`
	// PostDrainHook A list of hooks to run after draining AND UPDATING a node
	PostDrainHooks []DrainHook `json:"postDrainHooks,omitempty"`
}

type DrainHook struct {
	// Annotation This annotation will need to be populated on the capi.Machine with the value from the annotation
	// "rke.cattle.io/pre-drain" before the planner will continue with drain the specific node.  The annotation
	// "rke.cattle.io/pre-drain" is used for pre-drain and "rke.cattle.io/post-drain" is used for post drain.
	Annotation string `json:"annotation,omitempty"`
}

type Endpoint struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}
