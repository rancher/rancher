package clusterauthtoken

import (
	"github.com/rancher/rancher/pkg/controllers/user/clusterauthtoken/common"
	corev1 "github.com/rancher/types/apis/core/v1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SettingHandler struct {
	namespace              string
	clusterConfigMap       corev1.ConfigMapInterface
	clusterConfigMapLister corev1.ConfigMapLister
}

func (h *SettingHandler) Create(setting *managementv3.Setting) (runtime.Object, error) {
	if setting.Name != common.AuthProviderRefreshDebounceSettingName {
		return nil, nil
	}

	_, err := h.clusterConfigMapLister.Get(h.namespace, setting.Name)
	if !errors.IsNotFound(err) {
		return h.Updated(setting)
	}

	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: setting.Name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		Data: map[string]string{
			"value": setting.Value,
		},
	}
	_, err = h.clusterConfigMap.Create(config)
	return nil, err
}

func (h *SettingHandler) Updated(setting *managementv3.Setting) (runtime.Object, error) {
	if setting.Name != common.AuthProviderRefreshDebounceSettingName {
		return nil, nil
	}

	config, err := h.clusterConfigMapLister.Get(h.namespace, setting.Name)
	if errors.IsNotFound(err) {
		return h.Create(setting)
	}
	if err != nil {
		return nil, err
	}
	if config.Data == nil {
		config.Data = make(map[string]string)
	}
	if config.Data["value"] == setting.Value {
		return nil, nil
	}
	config.Data["value"] = setting.Value
	_, err = h.clusterConfigMap.Update(config)
	return nil, err
}

func (h *SettingHandler) Remove(setting *managementv3.Setting) (runtime.Object, error) {
	if setting.Name != common.AuthProviderRefreshDebounceSettingName {
		return nil, nil
	}

	_, err := h.clusterConfigMapLister.Get(h.namespace, setting.Name)
	if errors.IsNotFound(err) {
		return nil, nil
	}

	err = h.clusterConfigMap.Delete(setting.Name, &metav1.DeleteOptions{})
	return nil, err
}
