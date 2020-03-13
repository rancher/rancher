package converter

import (
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema/table"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/apiextensions.k8s.io/v1beta1"
	"github.com/rancher/wrangler/pkg/schemas"
	beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	staticFields = map[string]schemas.Field{
		"apiVersion": {
			Type: "string",
		},
		"kind": {
			Type: "string",
		},
		"metadata": {
			Type: "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
		},
	}
)

func AddCustomResources(crd v1beta1.CustomResourceDefinitionClient, schemas map[string]*types.APISchema) error {
	crds, err := crd.List(metav1.ListOptions{})
	if err != nil {
		return nil
	}

	for _, crd := range crds.Items {
		if crd.Status.AcceptedNames.Plural == "" {
			continue
		}

		var columns []table.Column
		for _, col := range crd.Spec.AdditionalPrinterColumns {
			columns = append(columns, table.Column{
				Name:  col.Name,
				Field: col.JSONPath,
				Type:  col.Type,
			})
		}

		group, kind := crd.Spec.Group, crd.Status.AcceptedNames.Kind

		if crd.Spec.Version != "" {
			forVersion(&crd, group, crd.Spec.Version, kind, schemas, crd.Spec.AdditionalPrinterColumns, columns)
		}
		for _, version := range crd.Spec.Versions {
			forVersion(&crd, group, version.Name, kind, schemas, crd.Spec.AdditionalPrinterColumns, columns)
		}
	}

	return nil
}

func forVersion(crd *beta1.CustomResourceDefinition, group, version, kind string, schemasMap map[string]*types.APISchema, columnDefs []beta1.CustomResourceColumnDefinition, columns []table.Column) {
	var versionColumns []table.Column
	for _, col := range columnDefs {
		versionColumns = append(versionColumns, table.Column{
			Name:   col.Name,
			Field:  col.JSONPath,
			Type:   col.Type,
			Format: col.Format,
		})
	}
	if len(versionColumns) == 0 {
		versionColumns = columns
	}

	id := GVKToVersionedSchemaID(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})

	schema := schemasMap[id]
	if schema == nil {
		return
	}
	if len(columns) > 0 {
		attributes.SetColumns(schema, columns)
	}

	if crd.Spec.Validation != nil && crd.Spec.Validation.OpenAPIV3Schema != nil {
		if fieldsSchema := modelV3ToSchema(id, crd.Spec.Validation.OpenAPIV3Schema, schemasMap); fieldsSchema != nil {
			for k, v := range staticFields {
				fieldsSchema.ResourceFields[k] = v
			}
			for k, v := range fieldsSchema.ResourceFields {
				if schema.ResourceFields == nil {
					schema.ResourceFields = map[string]schemas.Field{}
				}
				if _, ok := schema.ResourceFields[k]; !ok {
					schema.ResourceFields[k] = v
				}
			}
		}
	}
}
