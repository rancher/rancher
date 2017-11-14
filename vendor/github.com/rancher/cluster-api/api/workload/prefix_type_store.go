package workload

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
)

type PrefixTypeStore struct {
	Store types.Store
}

func (p *PrefixTypeStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	_, shortID := splitTypeAndID(id)
	data, err := p.Store.ByID(apiContext, schema, shortID)
	return addTypeToID(data), err
}

func (p *PrefixTypeStore) List(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) ([]map[string]interface{}, error) {
	data, err := p.Store.List(apiContext, schema, opt)
	return addListTypeToID(data), err
}

func (p *PrefixTypeStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) (chan map[string]interface{}, error) {
	c, err := p.Store.Watch(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	result := make(chan map[string]interface{})

	go func() {
		for item := range c {
			result <- addTypeToID(item)
		}
		close(result)
	}()

	return result, nil
}

func (p *PrefixTypeStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	data, err := p.Store.Create(apiContext, schema, data)
	return addTypeToID(data), err
}

func (p *PrefixTypeStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	_, shortID := splitTypeAndID(id)
	data, err := p.Store.Update(apiContext, schema, data, shortID)
	return addTypeToID(data), err
}

func (p *PrefixTypeStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) error {
	_, shortID := splitTypeAndID(id)
	return p.Store.Delete(apiContext, schema, shortID)
}

func splitTypeAndID(id string) (string, string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) < 2 {
		// Must conform
		return "", ""
	}
	return parts[0], parts[1]
}

func addTypeToID(data map[string]interface{}) map[string]interface{} {
	typeName := definition.GetType(data)
	id, _ := data["id"].(string)
	if typeName != "" && id != "" {
		data["id"] = strings.ToLower(typeName) + ":" + id
	}
	return data
}

func addListTypeToID(data []map[string]interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	for _, item := range data {
		result = append(result, addTypeToID(item))
	}
	return result
}
