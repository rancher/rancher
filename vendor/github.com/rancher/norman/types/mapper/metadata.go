package mapper

import (
	"github.com/rancher/norman/types"
)

func NewMetadataMapper() types.Mapper {
	return types.Mappers{
		Drop{"generateName"},
		Move{From: "selfLink", To: "resourcePath"},
		Move{From: "uid", To: "uuid"},
		Drop{"resourceVersion"},
		Drop{"generation"},
		Move{From: "creationTimestamp", To: "created"},
		Move{From: "deletionTimestamp", To: "removed"},
		Drop{"deletionGracePeriodSeconds"},
		Drop{"initializers"},
		//Drop{"finalizers"},
		Drop{"clusterName"},
		ReadOnly{Field: "*"},
		Access{
			Fields: map[string]string{
				"name":        "c",
				"namespace":   "cu",
				"labels":      "cu",
				"annotations": "cu",
			},
		},
	}
}
