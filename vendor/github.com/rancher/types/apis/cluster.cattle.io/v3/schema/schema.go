package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/factory"
	"github.com/rancher/types/mapper"
	"k8s.io/api/core/v1"
)

var (
	Version = types.APIVersion{
		Version: "v3",
		Group:   "cluster.cattle.io",
		Path:    "/v3/clusters",
	}

	Schemas = factory.Schemas(&Version).
		Init(namespaceTypes).
		Init(nodeTypes)
)

func namespaceTypes(schemas *types.Schemas) *types.Schemas {
	return schema.NamespaceTypes(&Version, schemas)
}

func nodeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.NodeStatus{},
			&mapper.NodeAddressMapper{},
			&mapper.OSInfo{},
			&m.Drop{Field: "addresses"},
			&m.Drop{Field: "daemonEndpoints"},
			&m.Drop{Field: "images"},
			&m.Drop{Field: "nodeInfo"},
			&m.SliceToMap{Field: "volumesAttached", Key: "devicePath"},
		).
		AddMapperForType(&Version, v1.NodeSpec{},
			&m.Move{From: "externalID", To: "externalId"}).
		AddMapperForType(&Version, v1.Node{},
			&m.Embed{Field: "status"},
			&m.Drop{Field: "conditions"},
		).
		MustImport(&Version, v1.NodeStatus{}, struct {
			IPAddress string
			Hostname  string
			Info      NodeInfo
		}{}).
		MustImport(&Version, v1.Node{})
}
