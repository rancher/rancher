package common

import (
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema/table"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/mappers"
)

var (
	NameColumn = table.Column{
		Name:   "Name",
		Field:  "metadata.name",
		Type:   "string",
		Format: "name",
	}
	CreatedColumn = table.Column{
		Name:   "Created",
		Field:  "metadata.creationTimestamp",
		Type:   "string",
		Format: "date",
	}
)

type DefaultColumns struct {
	mappers.EmptyMapper
}

func (d *DefaultColumns) ModifySchema(schema *schemas.Schema, schemas *schemas.Schemas) error {
	as := &types.APISchema{
		Schema: schema,
	}
	if attributes.Columns(as) == nil {
		attributes.SetColumns(as, []table.Column{
			NameColumn,
			CreatedColumn,
		})
	}

	return nil
}
