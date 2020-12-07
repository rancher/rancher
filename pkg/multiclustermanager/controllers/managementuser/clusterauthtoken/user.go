package clusterauthtoken

import (
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type userHandler struct {
	namespace                  string
	clusterUserAttribute       clusterv3.ClusterUserAttributeInterface
	clusterUserAttributeLister clusterv3.ClusterUserAttributeLister
}

func (h *userHandler) Sync(key string, user *managementv3.User) (runtime.Object, error) {
	if user == nil || user.DeletionTimestamp != nil {
		return nil, nil
	}

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
