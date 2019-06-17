package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ServiceEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ServiceEntrySpec `json:"spec"`
}

type ServiceEntrySpec struct {
	Hosts      []string                `json:"hosts,omitempty"`
	Addresses  []string                `json:"addresses,omitempty"`
	Ports      []Port                  `json:"ports,omitempty"`
	Location   int32                   `json:"location,omitempty"`
	Resolution int32                   `json:"resolution,omitempty"`
	Endpoints  []ServiceEntry_Endpoint `json:"endpoints,omitempty"`
}

type ServiceEntry_Endpoint struct {
	Address string            `json:"address,omitempty"`
	Ports   map[string]uint32 `json:"ports,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ServiceEntryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items           []ServiceEntry `json:"items"`
}
