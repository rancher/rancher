package namespacedresource

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/namespace"
)

//NamespacedStore makes sure that the namespaced resources are assigned to a given namespace
type namespacedStore struct {
	types.Store
	NamespaceInterface v1.NamespaceInterface
	Namespace          string
}

func Wrap(store types.Store, nsClient v1.NamespaceInterface, namespace string) types.Store {
	return &namespacedStore{
		Store:              store,
		NamespaceInterface: nsClient,
		Namespace:          namespace,
	}
}

func (s *namespacedStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	ns, ok := values.GetValue(data, client.PreferenceFieldNamespaceId)
	if ok && !strings.EqualFold(convert.ToString(ns), s.Namespace) {
		return nil, fmt.Errorf("error creating namespaced resource, cannot assign to %v since already assigned to %v namespace", namespace.GlobalNamespace, ns)
	} else if !ok {
		data[client.PreferenceFieldNamespaceId] = s.Namespace
	}

	return s.Store.Create(apiContext, schema, data)
}
