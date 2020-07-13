package clusterauthtoken

import (
	clusterv3 "github.com/rancher/rancher/pkg/types/apis/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type clusterUserAttributeHandler struct {
	userAttribute       managementv3.UserAttributeInterface
	userAttributeLister managementv3.UserAttributeLister
}

// Sync clusterUserAttributes and userAttributes
func (h *clusterUserAttributeHandler) Sync(key string, clusterUserAttribute *clusterv3.ClusterUserAttribute) (runtime.Object, error) {
	if clusterUserAttribute == nil || clusterUserAttribute.DeletionTimestamp != nil {
		return nil, nil
	}

	if !clusterUserAttribute.NeedsRefresh {
		return nil, nil
	}

	userAttribute, err := h.userAttributeLister.Get("", clusterUserAttribute.Name)
	if err != nil {
		return nil, err
	}

	if userAttribute.NeedsRefresh {
		return nil, nil
	}
	if userAttribute.LastRefresh != clusterUserAttribute.LastRefresh {
		return nil, nil
	}

	userAttribute.NeedsRefresh = true
	_, err = h.userAttribute.Update(userAttribute)
	return nil, err
}
