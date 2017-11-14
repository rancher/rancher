package authorization

import (
	"net/http"

	"github.com/rancher/norman/types"
)

type AllAccess struct {
}

func (*AllAccess) CanCreate(schema *types.Schema) bool {
	for _, method := range schema.CollectionMethods {
		if method == http.MethodPost {
			return true
		}
	}
	return false
}

func (*AllAccess) CanList(schema *types.Schema) bool {
	for _, method := range schema.CollectionMethods {
		if method == http.MethodGet {
			return true
		}
	}
	return false
}
