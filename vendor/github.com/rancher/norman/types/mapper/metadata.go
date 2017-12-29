package mapper

import (
	"github.com/rancher/norman/types"
)

func NewMetadataMapper() types.Mapper {
	return types.Mappers{
		ChangeType{Field: "name", Type: "dnsLabel"},
		Drop{"generateName"},
		//Move{From: "selfLink", To: "resourcePath"},
		Drop{"selfLink"},
		//Drop{"ownerReferences"},
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
