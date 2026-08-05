package mapper

import (
	"github.com/rancher/rancher/pkg/norman/types"
	"github.com/rancher/rancher/pkg/norman/types/convert"
	"github.com/rancher/rancher/pkg/norman/types/values"
)

type PendingStatus struct {
}

func (s PendingStatus) FromInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	if data["state"] != "active" {
		return
	}

	conditions := convert.ToMapSlice(values.GetValueN(data, "status", "conditions"))
	if len(conditions) > 0 {
		return
	}

	data["state"] = "pending"
}

func (s PendingStatus) ToInternal(data map[string]interface{}) error {
	return nil
}

func (s PendingStatus) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
