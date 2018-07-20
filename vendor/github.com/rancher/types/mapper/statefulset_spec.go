package mapper

import (
	"github.com/rancher/norman/types"
)

type StatefulSetSpecMapper struct {
}

func (s StatefulSetSpecMapper) FromInternal(data map[string]interface{}) {
}

func (s StatefulSetSpecMapper) ToInternal(data map[string]interface{}) error {
	return nil
}

func (s StatefulSetSpecMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
