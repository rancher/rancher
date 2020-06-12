package table

import (
	types2 "github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/wrangler/pkg/data"
	types "github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/mappers"
)

type Column struct {
	Name        string `json:"name,omitempty"`
	Field       string `json:"field,omitempty"`
	Type        string `json:"type,omitempty"`
	Format      string `json:"format,omitempty"`
	Description string `json:"description,omitempty"`
	Priority    int    `json:"priority,omitempty"`
}

type Table struct {
	Columns  []Column
	Computed func(data.Object)
}

type ColumnMapper struct {
	definition Table
	mappers.EmptyMapper
}

func NewColumns(computed func(data.Object), columns ...Column) *ColumnMapper {
	return &ColumnMapper{
		definition: Table{
			Columns:  columns,
			Computed: computed,
		},
	}
}

func (t *ColumnMapper) FromInternal(d data.Object) {
	if t.definition.Computed != nil {
		t.definition.Computed(d)
	}
}

func (t *ColumnMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	as := &types2.APISchema{
		Schema: schema,
	}
	cols := t.definition.Columns
	columnObj := attributes.Columns(as)
	if columns, ok := columnObj.([]Column); ok {
		cols = append(columns, cols...)
	}
	attributes.SetColumns(as, cols)
	return nil
}
