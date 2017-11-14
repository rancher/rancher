package mapper

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
)

func NewWorkloadTypeMapper() types.Mapper {
	return &types.Mappers{
		&m.Move{From: "labels", To: "workloadLabels"},
		&m.Move{From: "annotations", To: "workloadAnnotations"},
		&m.Move{From: "metadata/labels", To: "labels", NoDeleteFromField: true},
		&m.Move{From: "metadata/annotations", To: "annotations", NoDeleteFromField: true},
		&m.Drop{Field: "metadata"},
		&NamespaceIDMapper{},
	}
}
