package v3

import (
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Preference struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Value string `json:"value" norman:"required"`
}
