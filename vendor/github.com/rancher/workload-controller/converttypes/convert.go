package converttypes

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func InternalToInternal(from interface{}, fromSchema *types.Schema, toSchema *types.Schema, target interface{}) error {
	data, err := convert.EncodeToMap(from)
	if err != nil {
		return err
	}
	fromSchema.Mapper.FromInternal(data)
	toSchema.Mapper.ToInternal(data)
	return convert.ToObj(data, target)
}

func ToInternal(from interface{}, schema *types.Schema, target interface{}) error {
	data, err := convert.EncodeToMap(from)
	if err != nil {
		return err
	}
	schema.Mapper.ToInternal(data)
	return convert.ToObj(data, target)
}
