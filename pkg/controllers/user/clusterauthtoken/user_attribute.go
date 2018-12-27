package clusterauthtoken

import (
	"reflect"
	"sort"

	clusterv3 "github.com/rancher/types/apis/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type userAttributeCompare struct {
	groups       []string
	lastRefresh  string
	needsRefresh bool
	enabled      bool
}

type UserAttributeHandler struct {
	namespace                  string
	clusterUserAttribute       clusterv3.ClusterUserAttributeInterface
	clusterUserAttributeLister clusterv3.ClusterUserAttributeLister
}

func (h *UserAttributeHandler) Sync(key string, userAttribute *managementv3.UserAttribute) (runtime.Object, error) {
	if userAttribute == nil || userAttribute.DeletionTimestamp != nil {
		return nil, nil
	}

	clusterUserAttribute, err := h.clusterUserAttributeLister.Get(h.namespace, userAttribute.Name)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	groups, equal := compareUserAttributeClusterUserAttribute(*userAttribute, *clusterUserAttribute)
	if equal {
		return nil, nil
	}
	clusterUserAttribute.Groups = groups
	clusterUserAttribute.LastRefresh = userAttribute.LastRefresh
	clusterUserAttribute.NeedsRefresh = userAttribute.NeedsRefresh

	_, err = h.clusterUserAttribute.Update(clusterUserAttribute)
	return nil, err
}

func compareUserAttributeClusterUserAttribute(userAttribute managementv3.UserAttribute, clusterUserAttribute clusterv3.ClusterUserAttribute) ([]string, bool) {
	groups := []string{}
	for _, gp := range userAttribute.GroupPrincipals {
		for i := range gp.Items {
			groups = append(groups, gp.Items[i].Name)
		}
	}
	sort.Strings(groups)

	new := userAttributeCompare{
		groups:       groups,
		lastRefresh:  userAttribute.LastRefresh,
		needsRefresh: userAttribute.NeedsRefresh,
	}
	old := userAttributeCompare{
		groups:       clusterUserAttribute.Groups,
		lastRefresh:  clusterUserAttribute.LastRefresh,
		needsRefresh: clusterUserAttribute.NeedsRefresh,
	}
	return groups, reflect.DeepEqual(new, old)
}
