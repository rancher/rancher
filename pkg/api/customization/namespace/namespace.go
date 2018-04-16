package namespace

import (
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	"github.com/rancher/types/client/cluster/v3"
	"k8s.io/apimachinery/pkg/util/cache"
)

var (
	namespaceOwnerMap = cache.NewLRUExpireCache(1000)
	systemNamespaces  = map[string]bool{
		"cattle-system": true,
		"kube-public":   true,
		"kube-system":   true,
	}
)

func updateNamespaceOwnerMap(apiContext *types.APIContext) error {
	var namespaces []client.Namespace
	if err := access.List(apiContext, &schema.Version, client.NamespaceType, &types.QueryOptions{}, &namespaces); err != nil {
		return err
	}

	for _, namespace := range namespaces {
		namespaceOwnerMap.Add(namespace.Name, namespace.ProjectID, time.Hour)
	}

	return nil
}

func ProjectMap(apiContext *types.APIContext, refresh bool) (map[string]string, error) {
	if refresh {
		err := updateNamespaceOwnerMap(apiContext)
		if err != nil {
			return nil, err
		}
	}

	data := map[string]string{}
	for _, key := range namespaceOwnerMap.Keys() {
		if val, ok := namespaceOwnerMap.Get(key); ok {
			data[key.(string)] = val.(string)
		}
	}

	return data, nil
}

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	name := request.ID
	if len(name) == 0 {
		return nil
	}
	if _, ok := systemNamespaces[name]; ok && !convert.IsEmpty(data["projectId"]) {
		return httperror.NewAPIError(httperror.InvalidOption, "Can not set projectID on a system namespace")
	}

	return nil
}
