package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

type PersistVolumeClaim struct {
}

func (p PersistVolumeClaim) FromInternal(data map[string]interface{}) {
}

func (p PersistVolumeClaim) ToInternal(data map[string]interface{}) error {
	if v, ok := values.GetValue(data, "storageClassId"); ok && v == nil {
		values.PutValue(data, "", "storageClassId")
	}
	return nil
}

func (p PersistVolumeClaim) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
