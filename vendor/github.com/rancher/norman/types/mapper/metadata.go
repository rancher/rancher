package mapper

import (
	"github.com/rancher/norman/types"
)

func NewMetadataMapper() types.Mapper {
	return types.Mappers{
		ChangeType{Field: "name", Type: "dnsLabel"},
		Drop{Field: "generateName"},
		//Move{From: "selfLink", To: "resourcePath"},
		Drop{Field: "selfLink"},
		//Drop{"ownerReferences"},
		Move{From: "uid", To: "uuid"},
		Drop{Field: "resourceVersion"},
		Drop{Field: "generation"},
		Move{From: "creationTimestamp", To: "created"},
		Move{From: "deletionTimestamp", To: "removed"},
		Drop{Field: "deletionGracePeriodSeconds"},
		Drop{Field: "initializers"},
		//Drop{"finalizers"},
		Drop{Field: "clusterName"},
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
