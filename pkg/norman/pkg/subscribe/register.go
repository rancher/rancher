package subscribe

import (
	"net/http"

	"github.com/rancher/norman/types"
)

func Register(version *types.APIVersion, schemas *types.Schemas) {
	schemas.MustImportAndCustomize(version, Subscribe{}, func(schema *types.Schema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{}
		schema.ListHandler = Handler
		schema.PluralName = "subscribe"
	})
}
