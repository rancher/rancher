package namespace

import (
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
)

func ProjectMap(apiContext *types.APIContext) (map[string]string, error) {
	var namespaces []client.Namespace
	if err := access.List(apiContext, &schema.Version, client.NamespaceType, types.QueryOptions{}, &namespaces); err != nil {
		return nil, err
	}

	result := map[string]string{}
	for _, namespace := range namespaces {
		result[namespace.Name] = namespace.ProjectID
	}

	return result, nil
}
