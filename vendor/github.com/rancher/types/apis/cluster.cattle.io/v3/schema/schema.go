package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/factory"
	"k8s.io/api/core/v1"
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
		Init(volumeTypes)
)

func namespaceTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.NamespaceSpec{},
			&m.Drop{Field: "finalizers"},
		).
		AddMapperForType(&Version, v1.Namespace{},
			&m.AnnotationField{Field: "description"},
			&m.AnnotationField{Field: "projectId"},
			&m.Drop{Field: "status"},
		).
		MustImport(&Version, v1.Namespace{}, struct {
			Description string `json:"description"`
			ProjectID   string `norman:"type=reference[/v3/schemas/project]"`
		}{})
}

func volumeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v1.PersistentVolume{})
}
