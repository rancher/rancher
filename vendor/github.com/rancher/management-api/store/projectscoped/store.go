package projectscoped

import "github.com/rancher/norman/types"

type Store struct {
	types.Store
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if data != nil {
		data["namespaceId"] = data["projectId"]
		delete(data, "projectId")
	}

	return s.Store.Create(apiContext, schema, data)
}
