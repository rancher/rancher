package mappers

import (
	types "github.com/rancher/wrangler/pkg/schemas"
)

func NewMetadataMapper() types.Mapper {
	return types.Mappers{
		Move{From: "name", To: "id", CodeName: "ID"},
		Drop{Field: "namespace"},
		Drop{Field: "generateName"},
		Move{From: "uid", To: "uuid", CodeName: "UUID"},
		Drop{Field: "resourceVersion"},
		Drop{Field: "generation"},
		Move{From: "creationTimestamp", To: "created"},
		Move{From: "deletionTimestamp", To: "removed"},
		Drop{Field: "deletionGracePeriodSeconds"},
		Drop{Field: "initializers"},
		Drop{Field: "finalizers"},
		Drop{Field: "managedFields"},
		Drop{Field: "ownerReferences"},
		Drop{Field: "clusterName"},
		Drop{Field: "selfLink"},
		Access{
			Fields: map[string]string{
				"labels":      "cu",
				"annotations": "cu",
			},
		},
	}
}
