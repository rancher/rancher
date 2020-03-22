package ldap

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type unstructuredContenter interface {
	UnstructuredContent() map[string]interface{}
}

type runtimeObjectGetter interface {
	Get(name string, opts metav1.GetOptions) (runtime.Object, error)
}

// RemoteConfig is able to get ldap config from a remote source.
type RemoteConfig struct {
	authConfigs runtimeObjectGetter
}

func NewRemoteConfig(rog runtimeObjectGetter) *RemoteConfig {
	return &RemoteConfig{
		authConfigs: rog,
	}
}

func (rc *RemoteConfig) GetConfigMap(providerName string, opts metav1.GetOptions) (map[string]interface{}, error) {
	ro, err := rc.authConfigs.Get(providerName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve %s, error: %v", providerName, err)
	}
	u, ok := ro.(unstructuredContenter)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve %s, cannot read k8s Unstructured data", providerName)
	}
	return u.UnstructuredContent(), nil
}
