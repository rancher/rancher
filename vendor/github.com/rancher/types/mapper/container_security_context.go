package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type ContainerSecurityContext struct {
}

func (n ContainerSecurityContext) FromInternal(data map[string]interface{}) {
}

func (n ContainerSecurityContext) ToInternal(data map[string]interface{}) error {
	if v, ok := values.GetValue(data, "securityContext"); ok && v != nil {
		sc, err := convert.EncodeToMap(v)
		if err != nil {
			return nil
		}

		if v, ok := values.GetValue(sc, "capAdd"); ok && v != nil {
			capAdd := convert.ToStringSlice(v)
			if len(capAdd) == 0 {
				values.RemoveValue(data, "securityContext", "capAdd")
			}
		}

		if v, ok := values.GetValue(sc, "capDrop"); ok && v != nil {
			capAdd := convert.ToStringSlice(v)
			if len(capAdd) == 0 {
				values.RemoveValue(data, "securityContext", "capDrop")
			}
		}
	}

	return nil
}

func (n ContainerSecurityContext) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
