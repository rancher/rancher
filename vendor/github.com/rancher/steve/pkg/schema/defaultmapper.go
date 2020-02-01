package schema

import (
	"fmt"

	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/mappers"
)

func newDefaultMapper() schemas.Mapper {
	return &defaultMapper{}
}

type defaultMapper struct {
	mappers.EmptyMapper
}

func (d *defaultMapper) FromInternal(data data.Object) {
	if data["kind"] != "" && data["apiVersion"] != "" {
		if t, ok := data["type"]; ok && data != nil {
			data["_type"] = t
		}
	}

	if _, ok := data["id"]; ok || data == nil {
		return
	}

	name := types.Name(data)
	namespace := types.Namespace(data)

	if namespace == "" {
		data["id"] = name
	} else {
		data["id"] = fmt.Sprintf("%s/%s", namespace, name)
	}
}
