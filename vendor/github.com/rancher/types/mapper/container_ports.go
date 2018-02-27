package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/mapper"
)

type ContainerPorts struct {
}

func (n ContainerPorts) FromInternal(data map[string]interface{}) {
	field := mapper.AnnotationField{
		Field: "ports",
		List:  true,
	}
	field.FromInternal(data)

	containers := convert.ToInterfaceSlice(data["containers"])
	ports := convert.ToInterfaceSlice(data["ports"])

	for i := 0; i < len(ports) && i < len(containers); i++ {
		container := convert.ToMapInterface(containers[i])
		if container != nil {
			container["ports"] = ports[i]
		}
	}
}

func (n ContainerPorts) ToInternal(data map[string]interface{}) {
	field := mapper.AnnotationField{
		Field: "ports",
		List:  true,
	}

	var ports []interface{}
	path := []string{"containers", "{ARRAY}", "ports"}
	convert.Transform(data, path, func(obj interface{}) interface{} {
		if l, ok := obj.([]interface{}); ok {
			ports = append(ports, l...)
		}
		return obj
	})

	if len(ports) != 0 {
		data["ports"] = ports
		field.ToInternal(data)
	}
}

func (n ContainerPorts) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
