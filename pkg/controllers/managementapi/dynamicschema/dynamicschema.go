package dynamicschema

import (
	"context"
	"sync"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/crd"
	wranglerSchema "github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/openapi"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	sync.Mutex
	Schemas *types.Schemas
	lister  v3.DynamicSchemaLister
	known   map[string]bool
	CRDs    clientset.Interface
	ctx     context.Context
}

var nonClusterFields = map[string]bool{"credentialconfig": true, "nodeconfig": true, "nodetemplateconfig": true}

func Register(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas) {
	crdFactory, err := crd.NewFactoryFromClient(&management.RESTConfig)
	if err != nil {
		return
	}
	c := &Controller{
		Schemas: schemas,
		CRDs:    crdFactory.CRDClient,
		ctx:     ctx,
	}
	management.Management.DynamicSchemas("").AddHandler(ctx, "dynamic-schema", c.Sync)
}

func (c *Controller) Sync(key string, dynamicSchema *v3.DynamicSchema) (runtime.Object, error) {
	c.Lock()
	defer c.Unlock()

	if dynamicSchema == nil {
		return nil, c.remove(key)
	}

	return nil, c.add(dynamicSchema, key)
}

func (c *Controller) remove(id string) error {
	schema := c.Schemas.Schema(&managementSchema.Version, id)
	if schema != nil {
		c.Schemas.RemoveSchema(*schema)
	}
	return nil
}

func (c *Controller) add(dynamicSchema *v3.DynamicSchema, key string) error {
	schema := types.Schema{}
	if err := convert.ToObj(dynamicSchema.Spec, &schema); err != nil {
		return err
	}

	wSchema := wranglerSchema.Schema{}
	if err := convert.ToObj(dynamicSchema.Spec, &wSchema); err != nil {
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

	// openapischema for cluster fields is generated after adding cluster CRD
	if nonClusterFields[key] || key == "cluster" {
		return nil
	}

	openapiSchema, err := openapi.SchemaToProps(&wSchema, wranglerSchema.EmptySchemas(), map[string]bool{})
	if err != nil {
		return err
	}

	clusterCRD, err := c.CRDs.ApiextensionsV1beta1().CustomResourceDefinitions().Get(c.ctx, "clusters.management.cattle.io", metav1.GetOptions{})
	if err != nil {
		return err
	}

	// we need to maintain backwards compatibility with older dynamic schemas that were created before we had the
	// schema name field
	if dynamicSchema.Spec.SchemaName != "" {
		clusterCRD.Spec.Validation.OpenAPIV3Schema.Properties[dynamicSchema.Spec.SchemaName] = *openapiSchema
	} else {
		clusterCRD.Spec.Validation.OpenAPIV3Schema.Properties[dynamicSchema.Name] = *openapiSchema
	}

	_, err = c.CRDs.ApiextensionsV1beta1().CustomResourceDefinitions().Update(c.ctx, clusterCRD, metav1.UpdateOptions{})
	return err
}
