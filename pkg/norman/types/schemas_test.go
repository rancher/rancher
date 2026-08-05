package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemas(t *testing.T) {
	version := APIVersion{
		Group:   "meta.cattle.io",
		Version: "v1",
		Path:    "/shire",
	}

	s := NewSchemas().
		AddSchema(Schema{
			ID:                "baggins",
			PluralName:        "bagginses",
			Version:           version,
			CollectionMethods: []string{},
			ResourceMethods:   []string{},
			ResourceFields:    map[string]Field{},
		}).
		AddSchema(Schema{
			ID:                "hobbit",
			PluralName:        "hobbits",
			Embed:             true,
			EmbedType:         "baggins",
			Version:           version,
			CollectionMethods: []string{},
			ResourceMethods:   []string{},
			ResourceFields: map[string]Field{
				"breakfasts": {Type: "int"},
				"name":       {Type: "string"},
			},
		})

	expected := []*Schema{
		{
			ID:                "hobbit",
			PluralName:        "hobbits",
			Embed:             true,
			EmbedType:         "baggins",
			Version:           version,
			CollectionMethods: []string{},
			ResourceMethods:   []string{},
			ResourceFields: map[string]Field{
				"breakfasts": {Type: "int"},
				"name":       {Type: "string"},
			},
			CodeName:       "Hobbit",
			CodeNamePlural: "Hobbits",
			BaseType:       "hobbit",
			Type:           "/meta/schemas/schema",
		},
		{
			ID:                "baggins",
			PluralName:        "bagginses",
			Version:           version,
			CollectionMethods: []string{},
			ResourceMethods:   []string{},
			ResourceFields: map[string]Field{
				"breakfasts": {Type: "int"},
				"name":       {Type: "string"},
			},
			CodeName:       "Baggins",
			CodeNamePlural: "Bagginses",
			BaseType:       "baggins",
			Type:           "/meta/schemas/schema",
		},
	}
	actual := s.Schemas()

	assert.ElementsMatch(t, expected, actual)
}
