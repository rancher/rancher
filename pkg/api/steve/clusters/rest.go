package clusters

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type ApplyInput struct {
	DefaultNamespace string `json:"defaultNamespace,omitempty"`
	YAML             string `json:"yaml,omitempty"`
}

type ApplyOutput struct {
	Resources []runtime.Object `json:"resources,omitempty"`
}
