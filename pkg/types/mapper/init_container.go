package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type InitContainerMapper struct {
}

func (e InitContainerMapper) FromInternal(data map[string]interface{}) {
	containers, _ := data["containers"].([]interface{})

	for _, initContainer := range convert.ToMapSlice(data["initContainers"]) {
		if initContainer == nil {
			continue
		}
		initContainer["initContainer"] = true
		containers = append(containers, initContainer)
	}

	if data != nil {
		data["containers"] = containers
	}
}

func (e InitContainerMapper) ToInternal(data map[string]interface{}) error {
	var newContainers []interface{}
	var newInitContainers []interface{}

	for _, container := range convert.ToMapSlice(data["containers"]) {
		if convert.ToBool(container["initContainer"]) {
			newInitContainers = append(newInitContainers, container)
		} else {
			newContainers = append(newContainers, container)
		}
		delete(container, "initContainer")
	}

	if data != nil {
		data["containers"] = newContainers
		data["initContainers"] = newInitContainers
	}

	return nil
}

func (e InitContainerMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	delete(schema.ResourceFields, "initContainers")
	return nil
}
