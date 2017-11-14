package mapper

import (
	"github.com/rancher/norman/types"
)

type Object struct {
	types.Mappers
}

func NewObject(mappers ...types.Mapper) Object {
	return Object{
		Mappers: append([]types.Mapper{
			&Embed{Field: "metadata"},
			&Embed{Field: "spec", Optional: true},
			&ReadOnly{Field: "status", Optional: true},
			Drop{"kind"},
			Drop{"apiVersion"},
			&Scope{
				IfNot: types.NamespaceScope,
				Mappers: []types.Mapper{
					&Drop{"namespace"},
				},
			},
		}, mappers...),
	}
}
