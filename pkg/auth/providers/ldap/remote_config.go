package ldap

import (
	"fmt"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RemoteConfig is able to get ldap config from a remote source.
type RemoteConfig struct {
	authConfigs v3.AuthConfigInterface
}

func NewRemoteConfig(authConf v3.AuthConfigInterface) *RemoteConfig {
	return &RemoteConfig{
		authConfigs: authConf,
	}
}

func (rc *RemoteConfig) GetConfigMap(providerName string, opts metav1.GetOptions) (map[string]interface{}, error) {
	authConfigObj, err := rc.authConfigs.ObjectClient().UnstructuredClient().Get(providerName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve %s, error: %v", providerName, err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve %s, cannot read k8s Unstructured data", providerName)
	}

	return u.UnstructuredContent(), nil
}
