package cluster

import (
	"github.com/rancher/norman/types"
)

type Store struct {
	types.Store
	ShellHandler types.RequestHandler
}

func (r *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	// Really we want a link handler but the URL parse makes it impossible to add links to clusters for now.  So this
	// is basically a hack
	if apiContext.Query.Get("shell") == "true" {
		return nil, r.ShellHandler(apiContext, nil)
	}
	return r.Store.ByID(apiContext, schema, id)
}
