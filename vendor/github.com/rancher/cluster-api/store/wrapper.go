package store

import (
	"strings"

	"github.com/rancher/cluster-api/api/namespace"
	"github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/types/client/project/v3"
)

func ProjectSetter(wrapper api.StoreWrapper) api.StoreWrapper {
	return func(store types.Store) types.Store {
		return wrapper(&projectIDSetterStore{
			Store: store,
		})
	}
}

type projectIDSetterStore struct {
	types.Store
}

func (p *projectIDSetterStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := p.Store.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

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

func (p *projectIDSetterStore) List(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) ([]map[string]interface{}, error) {
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

func (p *projectIDSetterStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) (chan map[string]interface{}, error) {
	c, err := p.Store.Watch(apiContext, schema, opt)
	if err != nil || c == nil {
		return nil, err
	}

	namespaceMap, err := namespace.ProjectMap(apiContext)
	if err != nil {
		return nil, err
	}

	result := make(chan map[string]interface{})
	go func() {
		for data := range c {
			typeName := definition.GetType(data)
			if strings.Contains(typeName, "namespace") || strings.Contains(typeName, "project") {
				tempNamespaceMap, err := namespace.ProjectMap(apiContext)
				if err == nil {
					namespaceMap = tempNamespaceMap
				}
			}
			setProjectID(namespaceMap, data)
			result <- data
		}
	}()

	return result, nil
}

func setProjectID(namespaceMap map[string]string, data map[string]interface{}) {
	namespace, _ := data[client.PodFieldNamespaceId].(string)
	projectID, _ := data[client.NamespaceFieldProjectID].(string)
	if projectID != "" {
		return
	}

	data[client.NamespaceFieldProjectID] = namespaceMap[namespace]
}
