package schema

import (
	"reflect"
	"strings"

	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	v3 "github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	"github.com/rancher/rancher/pkg/schemas/factory"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	Version = types.APIVersion{
		Version:          "v3",
		Group:            "cluster.cattle.io",
		Path:             "/v3/cluster",
		SubContext:       true,
		SubContextSchema: "/v3/schemas/cluster",
	}

	Schemas = factory.Schemas(&Version).
		Init(namespaceTypes).
		Init(persistentVolumeTypes).
		Init(storageClassTypes).
		Init(tokens).
		Init(apiServiceTypes)
)

func namespaceTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.NamespaceSpec{},
			&m.Drop{Field: "finalizers"},
		).
		AddMapperForType(&Version, v1.Namespace{},
			&m.AnnotationField{Field: "description"},
			&m.AnnotationField{Field: "projectId"},
			&m.AnnotationField{Field: "resourceQuota", Object: true},
			&m.AnnotationField{Field: "containerDefaultResourceLimit", Object: true},
			&m.Drop{Field: "status"},
		).
		MustImport(&Version, NamespaceResourceQuota{}).
		MustImport(&Version, ContainerResourceLimit{}).
		MustImport(&Version, v1.Namespace{}, struct {
			Description                   string `json:"description"`
			ProjectID                     string `norman:"type=reference[/v3/schemas/project],noupdate"`
			ResourceQuota                 string `json:"resourceQuota,omitempty" norman:"type=namespaceResourceQuota"`
			ContainerDefaultResourceLimit string `json:"containerDefaultResourceLimit,omitempty" norman:"type=containerResourceLimit"`
		}{}).
		MustImport(&Version, NamespaceMove{}).
		MustImportAndCustomize(&Version, v1.Namespace{}, func(schema *types.Schema) {
			schema.ResourceActions["move"] = types.Action{
				Input: "namespaceMove",
			}
		})
}

func persistentVolumeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.PersistentVolume{},
			&m.AnnotationField{Field: "description"},
		).
		AddMapperForType(&Version, v1.HostPathVolumeSource{},
			m.Move{From: "type", To: "kind"},
			m.Enum{
				Options: []string{
					"DirectoryOrCreate",
					"Directory",
					"FileOrCreate",
					"File",
					"Socket",
					"CharDevice",
					"BlockDevice",
				},
				Field: "kind",
			},
		).
		MustImport(&Version, v1.PersistentVolumeSpec{}, struct {
			StorageClassName *string `json:"storageClassName,omitempty" norman:"type=reference[storageClass]"`
		}{}).
		MustImport(&Version, v1.PersistentVolume{}, struct {
			Description string `json:"description"`
		}{}).
		MustImportAndCustomize(&Version, v1.PersistentVolume{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "hostname"
				field.Nullable = false
				field.Required = true
				return field
			})
			schema.MustCustomizeField("volumeMode", func(field types.Field) types.Field {
				field.Update = false
				return field
			})
			// All fields of PersistentVolumeSource are immutable
			val := reflect.ValueOf(v1.PersistentVolumeSource{})
			for i := 0; i < val.Type().NumField(); i++ {
				if tag, ok := val.Type().Field(i).Tag.Lookup("json"); ok {
					name := strings.Split(tag, ",")[0]
					schema.MustCustomizeField(name, func(field types.Field) types.Field {
						field.Update = false
						return field
					})
					pvSchema := schemas.Schema(&Version, val.Type().Field(i).Type.String()[4:])
					for name := range pvSchema.ResourceFields {
						pvSchema.MustCustomizeField(name, func(field types.Field) types.Field {
							field.Update = false
							return field
						})
					}
				}
			}
		})
}

func storageClassTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, storagev1.StorageClass{},
			&m.AnnotationField{Field: "description"},
		).
		MustImport(&Version, storagev1.StorageClass{}, struct {
			Description   string `json:"description"`
			ReclaimPolicy string `json:"reclaimPolicy,omitempty" norman:"type=enum,options=Recycle|Delete|Retain"`
		}{})
}

func tokens(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImportAndCustomize(&Version, v3.ClusterAuthToken{}, func(schema *types.Schema) {
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{}
		}).
		MustImportAndCustomize(&Version, v3.ClusterUserAttribute{}, func(schema *types.Schema) {
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{}
		})
}

func apiServiceTypes(Schemas *types.Schemas) *types.Schemas {
	return Schemas.
		AddMapperForType(&Version, apiregistrationv1.APIService{},
			&m.Embed{Field: "status"},
		).
		MustImport(&Version, apiregistrationv1.APIService{})
}
