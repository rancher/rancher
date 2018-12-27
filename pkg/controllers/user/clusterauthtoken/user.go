package clusterauthtoken

import (
	clusterv3 "github.com/rancher/types/apis/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserHandler struct {
	namespace                  string
	clusterUserAttribute       clusterv3.ClusterUserAttributeInterface
	clusterUserAttributeLister clusterv3.ClusterUserAttributeLister
}

func (h *UserHandler) Create(user *managementv3.User) (runtime.Object, error) {
	return nil, nil
}

func (h *UserHandler) Updated(user *managementv3.User) (runtime.Object, error) {
	clusterUserAttribute, err := h.clusterUserAttributeLister.Get(h.namespace, user.Name)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	userEnabled := user.Enabled == nil || *user.Enabled
	if clusterUserAttribute.Enabled == userEnabled {
		return nil, nil
	}

	clusterUserAttribute.Enabled = userEnabled
	_, err = h.clusterUserAttribute.Update(clusterUserAttribute)
	return nil, err
}

func (h *UserHandler) Remove(user *managementv3.User) (runtime.Object, error) {
	_, err := h.clusterUserAttributeLister.Get(h.namespace, user.Name)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	err = h.clusterUserAttribute.Delete(user.Name, &metav1.DeleteOptions{})
	return nil, err
}
