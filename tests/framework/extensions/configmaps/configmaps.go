package configmaps

import (
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConfigMapSteveType = "configmap"
)

type SteveConfigMap struct {
	metav1.TypeMeta    `json:",inline"`
	steveV1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Data               map[string]any `json:"data"`
}
