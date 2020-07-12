package v3

import (
	"github.com/rancher/norman/types"
	rketypes "github.com/rancher/rke/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EtcdBackup struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// backup spec
	Spec rketypes.EtcdBackupSpec `json:"spec"`
	// backup status
	Status rketypes.EtcdBackupStatus `yaml:"status" json:"status,omitempty"`
}

type RkeK8sSystemImage struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	SystemImages rketypes.RKESystemImages `yaml:"system_images" json:"systemImages,omitempty"`
}

type RkeK8sServiceOption struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ServiceOptions rketypes.KubernetesServicesOptions `yaml:"service_options" json:"serviceOptions,omitempty"`
}

type RkeAddon struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Template string `yaml:"template" json:"template,omitempty"`
}
