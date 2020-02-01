package mappers

import (
	"fmt"

	types "github.com/rancher/wrangler/pkg/schemas"
)

func ValidateField(field string, schema *types.Schema) error {
	if _, ok := schema.ResourceFields[field]; !ok {
		return fmt.Errorf("field %s missing on schema %s", field, schema.ID)
	}

	return nil
}
