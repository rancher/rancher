package projectsetter

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/rancher/pkg/api/customization/namespace"
	"github.com/rancher/types/client/cluster/v3"
)

func Wrap(store types.Store) types.Store {
	return &Store{
		store,
	}
}

type Store struct {
	types.Store
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	data, err := p.Store.Create(apiContext, schema, data)
	if err != nil {
		return nil, err
	}

	return lookupAndSetProjectID(apiContext, schema, data)
}

func (p *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := p.Store.Delete(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	return lookupAndSetProjectID(apiContext, schema, data)
}

func (p *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := p.Store.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	return lookupAndSetProjectID(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	data, err := p.Store.Update(apiContext, schema, data, id)
	if err != nil {
		return nil, err
	}

	return lookupAndSetProjectID(apiContext, schema, data)
}

func (p *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	datas, err := p.Store.List(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}

	if _, ok := schema.ResourceFields[client.NamespaceFieldProjectID]; !ok || schema.ID == client.NamespaceType {
		return datas, nil
	}

	namespaceMap, err := namespace.ProjectMap(apiContext)
	if err != nil {
		return nil, err
	}

	for _, data := range datas {
		setProjectID(namespaceMap, data)
	}

	return datas, nil
}

func (p *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	c, err := p.Store.Watch(apiContext, schema, opt)
	if err != nil || c == nil {
		return nil, err
	}

	namespaceMap, err := namespace.ProjectMap(apiContext)
	if err != nil {
		return nil, err
	}

	return convert.Chan(c, func(data map[string]interface{}) map[string]interface{} {
		typeName := definition.GetType(data)
		if strings.Contains(typeName, "namespace") || strings.Contains(typeName, "project") {
			tempNamespaceMap, err := namespace.ProjectMap(apiContext)
			if err == nil {
				namespaceMap = tempNamespaceMap
			}
		}
		setProjectID(namespaceMap, data)
		return data
	}), nil
}

func lookupAndSetProjectID(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := schema.ResourceFields[client.NamespaceFieldProjectID]; !ok || schema.ID == client.NamespaceType {
		return data, nil
	}

	namespaceMap, err := namespace.ProjectMap(apiContext)
	if err != nil {
		return nil, err
	}

	setProjectID(namespaceMap, data)

	return data, nil
}

func setProjectID(namespaceMap map[string]string, data map[string]interface{}) {
	if data == nil {
		return
	}

	namespace, _ := data["namespaceId"].(string)
	projectID, _ := data[client.NamespaceFieldProjectID].(string)
	if projectID != "" {
		return
	}

	data[client.NamespaceFieldProjectID] = namespaceMap[namespace]
}
