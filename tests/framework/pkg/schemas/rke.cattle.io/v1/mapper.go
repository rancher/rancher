package schema

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/factory"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/rancher/pkg/schemas/mapper"
)

func RKESchemas(version *types.APIVersion) *types.Schemas {
	schemas := factory.Schemas(version)
	schemas.DefaultMappers = func() []types.Mapper {
		mappers := []types.Mapper{
			&m.APIGroup{},
			&m.SelfLink{},
			&m.ReadOnly{Field: "status", Optional: true, SubFields: true},
			m.Drop{Field: "kind"},
			m.Drop{Field: "apiVersion"},
		}
		return mappers
	}
	basePostFunc := schemas.DefaultPostMappers
	schemas.DefaultPostMappers = func() []types.Mapper {
		return append(basePostFunc(), &mapper.Creator{})
	}
	return schemas
}
