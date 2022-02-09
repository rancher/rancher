package v3

import (
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ManagedChart struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedChartSpec   `json:"spec"`
	Status ManagedChartStatus `json:"status"`
}

type ManagedChartSpec struct {
	Paused           bool               `json:"paused,omitempty"`
	Chart            string             `json:"chart,omitempty"`
	RepoName         string             `json:"repoName,omitempty"`
	ReleaseName      string             `json:"releaseName,omitempty"`
	Version          string             `json:"version,omitempty"`
	TimeoutSeconds   int                `json:"timeoutSeconds,omitempty"`
	Values           *fleet.GenericMap  `json:"values,omitempty"`
	Force            bool               `json:"force,omitempty"`
	TakeOwnership    bool               `json:"takeOwnership,omitempty"`
	MaxHistory       int                `json:"maxHistory,omitempty"`
	DefaultNamespace string             `json:"defaultNamespace,omitempty"`
	TargetNamespace  string             `json:"namespace,omitempty"`
	ServiceAccount   string             `json:"serviceAccount,omitempty"`
	Diff             *fleet.DiffOptions `json:"diff,omitempty"`

	RolloutStrategy *fleet.RolloutStrategy `json:"rolloutStrategy,omitempty"`
	Targets         []fleet.BundleTarget   `json:"targets,omitempty"`
}

type ManagedChartStatus struct {
	fleet.BundleStatus
}
