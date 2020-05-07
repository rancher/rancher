package dynamicschema

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rancher/wrangler/pkg/schemas/openapi"
	wranglerSchema "github.com/rancher/wrangler/pkg/schemas"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"sync"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/config"
	wrCrd "github.com/rancher/wrangler/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime"
	//apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	//"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

type Controller struct {
	sync.Mutex
	Schemas *types.Schemas
	lister  v3.DynamicSchemaLister
	known   map[string]bool
	CRDClient clientset.Interface
}

func Register(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas) {
	crdF, err := wrCrd.NewFactoryFromClient(&management.RESTConfig)
	if err != nil {
		fmt.Printf("\nerror getting NewFactoryFromClient: %v\n", crdF)
	}
	c := &Controller{
		Schemas: schemas,
		CRDClient: crdF.CRDClient,
	}
	management.Management.DynamicSchemas("").AddHandler(ctx, "dynamic-schema", c.Sync)
}

func (c *Controller) Sync(key string, dynamicSchema *v3.DynamicSchema) (runtime.Object, error) {
	fmt.Printf("\nDynamicSchema sync for %v\n", key)
	c.Lock()
	defer c.Unlock()

	if dynamicSchema == nil {
		return nil, c.remove(key)
	}
fmt.Printf("\ncalling add now for %v\n", key)
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
		fmt.Printf("\nWTF wrangler err: %v\n", err)
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

	openapiSchema, err := openapi.SchemaToProps(&wSchema, wranglerSchema.EmptySchemas(), map[string]bool{})
	if err != nil {
		fmt.Printf("\nARGH error for %v: %v\n", key, err)
	} else {
		fmt.Printf("\nyaaay no error for %v\n", key)
	}

//fmt.Printf("\nfor %v, newcrd: %#v\n", key, newCrd.Schema)

	//openAPIMap, err := convert.EncodeToMap(newCrd.Schema)
	//if err != nil {
	//	fmt.Printf("\nEncodeToMap err : %v\n", err)
	//}
	//fmt.Printf("\nopenAPIMap: %v\n", openAPIMap)
	bytes, err := json.Marshal(openapiSchema)
	if err != nil {
		fmt.Printf("\nMarshal err : %v\n", err)
	}
	fmt.Printf("\nopenAPIMap for %v: %v\n", key, string(bytes))

	clusterCRD, err := c.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(context.Background(), "clusters.management.cattle.io", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("\nError %v in getting cluster CRD\n", err)
	}
	//if key == "digitaloceanconfig" {
		//fmt.Printf("\nclusterCRD spec: %v\n", clusterCRD)
		//fmt.Printf("\nclusterCRD validation openapi prop for %v: %v\n",key, clusterCRD.Spec.Validation.OpenAPIV3Schema.Properties)
	//}
	if openapiSchema != nil {
		clusterCRD.Spec.Validation.OpenAPIV3Schema.Properties[key] = *openapiSchema
		_, err = c.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(context.Background(), clusterCRD, metav1.UpdateOptions{})
		if err != nil {
			fmt.Printf("\nerr updating cluster crd: %v\n", err)
		}
	}

	return nil
}
