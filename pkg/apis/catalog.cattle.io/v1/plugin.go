package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type UIPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UIPluginSpec   `json:"spec"`
	Status            UIPluginStatus `json:"status"`
}

type UIPluginSpec struct {
	Plugin UIPluginEntry `json:"plugin,omitempty"`
}

type UIPluginEntry struct {
	Name     string            `json:"name,omitempty"`
	Version  string            `json:"version,omitempty"`
	Endpoint string            `json:"endpoint,omitempty"`
	NoCache  bool              `json:"noCache,omitempty"`
	NoAuth   bool              `json:"noAuth,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type UIPluginStatus struct {
	CacheState string `json:"cacheState,omitempty"`
}
