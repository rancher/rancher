package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

var namespaceMapping = map[string]string{
	"hostNetwork": "net",
	"hostIPC":     "ipc",
	"hostPID":     "pid",
}

type NamespaceMapper struct {
}

func (e NamespaceMapper) FromInternal(data map[string]interface{}) {
	for name, friendlyName := range namespaceMapping {
		value := convert.ToBool(data[name])
		if value {
			data[friendlyName] = "host"
		}
		delete(data, name)
	}
}

func (e NamespaceMapper) ToInternal(data map[string]interface{}) {
	for name, friendlyName := range namespaceMapping {
		value := convert.ToString(data[friendlyName])
		if value == "host" {
			data[name] = true
		} else {
			data[name] = false
		}
		delete(data, friendlyName)
	}
}

func (e NamespaceMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	delete(schema.ResourceFields, "hostNetwork")
	delete(schema.ResourceFields, "hostPID")
	delete(schema.ResourceFields, "hostIPC")
	return nil
}
