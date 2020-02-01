package subscribe

import (
	"net/http"

	"github.com/rancher/steve/pkg/schemaserver/types"
)

func Register(schemas *types.APISchemas) {
	schemas.MustImportAndCustomize(Subscribe{}, func(schema *types.APISchema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{}
		schema.ListHandler = Handler
		schema.PluralName = "subscribe"
	})
}
