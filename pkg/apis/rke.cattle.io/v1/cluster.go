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
	LocalClusterAuthEndpoint LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty"`
	UpgradeStrategy          ClusterUpgradeStrategy   `json:"upgradeStrategy,omitempty"`
	CNIDriver                string                   `json:"cni,omitempty"`
	ChartValues              GenericMap               `json:"chartValues,omitempty" wrangler:"nullable"`
	ControlPlaneConfig       GenericMap               `json:"controlPlaneConfig,omitempty" wrangler:"nullable"`
	NodeConfig               []RKESystemConfig        `json:"config,omitempty"`
}

type LocalClusterAuthEndpoint struct {
	Enabled *bool  `json:"enabled,omitempty"`
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
	// How many controlplane nodes should be upgrade at time, defaults to 1
	ServerConcurrency int `json:"serverConcurrency,omitempty" norman:"min=1"`
	// How many workers should be upgraded at a time
	WorkerConcurrency int `json:"workerConcurrency,omitempty" norman:"min=1"`
	// Whether controlplane nodes should be drained
	DrainServerNodes bool `json:"drainServerNodes,omitempty"`
	// Whether worker nodes should be drained
	DrainWorkerNodes bool `json:"drainWorkerNodes,omitempty"`
}

type Endpoint struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}
