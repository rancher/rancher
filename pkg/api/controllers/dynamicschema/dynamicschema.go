package dynamicschema

import (
	"context"
	"sync"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	managementSchema "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	sync.Mutex
	Schemas *types.Schemas
	lister  v3.DynamicSchemaLister
	known   map[string]bool
}

func Register(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas) {
	c := &Controller{
		Schemas: schemas,
	}
	management.Management.DynamicSchemas("").AddHandler(ctx, "dynamic-schema", c.Sync)
}

func (c *Controller) Sync(key string, dynamicSchema *v3.DynamicSchema) (runtime.Object, error) {
	c.Lock()
	defer c.Unlock()

	if dynamicSchema == nil {
		return nil, c.remove(key)
	}

	return nil, c.add(dynamicSchema)
}

func (c *Controller) remove(id string) error {
	schema := c.Schemas.Schema(&managementSchema.Version, id)
	if schema != nil {
		c.Schemas.RemoveSchema(*schema)
	}
	return nil
}

func (c *Controller) add(dynamicSchema *v3.DynamicSchema) error {
	schema := types.Schema{}
	if err := convert.ToObj(dynamicSchema.Spec, &schema); err != nil {
		return err
	}

	for name, field := range schema.ResourceFields {
		defMap, ok := field.Default.(map[string]interface{})
		if !ok {
			continue
		}

		// set to nil because if map is len() == 0
		field.Default = nil

		switch field.Type {
		case "string", "password":
			field.Default = defMap["stringValue"]
		case "int":
			field.Default = defMap["intValue"]
		case "boolean":
			field.Default = defMap["boolValue"]
		case "array[string]":
			field.Default = defMap["stringSliceValue"]
		}

		field.DynamicField = true

		schema.ResourceFields[name] = field
	}

	// we need to maintain backwards compatibility with older dynamic schemas that were created before we had the
	// schema name field
	if dynamicSchema.Spec.SchemaName != "" {
		schema.ID = dynamicSchema.Spec.SchemaName
	} else {
		schema.ID = dynamicSchema.Name
	}
	schema.Version = managementSchema.Version
	schema.DynamicSchemaVersion = dynamicSchema.ResourceVersion

	if schema.Embed {
		c.Schemas.AddSchema(schema)
	} else {
		c.Schemas.ForceAddSchema(schema)
	}

	return nil
}
