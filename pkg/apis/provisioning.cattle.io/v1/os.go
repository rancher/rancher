package v1

import (
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ManagedOS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedOSSpec   `json:"spec"`
	Status ManagedOSStatus `json:"status"`
}

type ManagedOSSpec struct {
	Paused       bool                  `json:"paused,omitempty"`
	OSImage      string                `json:"osImage,omitempty"`
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
	Concurrency  *int64                `json:"concurrency,omitempty"`

	Prepare *upgradev1.ContainerSpec `json:"prepare,omitempty"`
	Cordon  *bool                    `json:"cordon,omitempty"`
	Drain   *upgradev1.DrainSpec     `json:"drain,omitempty"`

	ClusterRolloutStrategy *fleet.RolloutStrategy `json:"clusterRolloutStrategy,omitempty"`
	Targets                []fleet.BundleTarget   `json:"clusterTargets,omitempty"`
}

type ManagedOSStatus struct {
	fleet.BundleStatus
}
