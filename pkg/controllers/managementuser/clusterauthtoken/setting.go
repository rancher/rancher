package clusterauthtoken

import (
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken/common"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type settingHandler struct {
	namespace              string
	clusterConfigMap       corev1.ConfigMapInterface
	clusterConfigMapLister corev1.ConfigMapLister
	settingInterface       managementv3.SettingInterface
}

func (h *settingHandler) Sync(key string, setting *managementv3.Setting) (runtime.Object, error) {
	if setting == nil {
		return nil, nil
	}
	// remove legacy finalizers
	if setting.DeletionTimestamp != nil {
		finalizers := setting.GetFinalizers()
		for i, finalizer := range finalizers {
			if finalizer == "controller.cattle.io/cat-setting-controller" {
				finalizers = append(finalizers[:i], finalizers[i+1:]...)
				setting = setting.DeepCopy()
				setting.SetFinalizers(finalizers)
				_, err := h.settingInterface.Update(setting)
				if err != nil {
					return nil, err
				}
				break
			}
		}
		err := h.clusterConfigMap.Delete(setting.Name, &metav1.DeleteOptions{})
		return nil, err
	}
	if setting.Name != common.AuthProviderRefreshDebounceSettingName {
		return nil, nil
	}
	// create/update
	config, err := h.clusterConfigMapLister.Get(h.namespace, setting.Name)
	if errors.IsNotFound(err) {
		NewConfig := &v1.ConfigMap{
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
		_, err = h.clusterConfigMap.Create(NewConfig)
		return nil, err
	} else if err == nil {
		if config.Data == nil {
			config.Data = make(map[string]string)
		}
		if config.Data["value"] == setting.Value {
			return nil, nil
		}
		config.Data["value"] = setting.Value
		_, err = h.clusterConfigMap.Update(config)
		return nil, err
	} else {
		return nil, err
	}
}
