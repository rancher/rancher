package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/factory"
	"github.com/rancher/types/mapper"
	"k8s.io/api/core/v1"
)

var (
	Version = types.APIVersion{
		Version: "v3",
		Group:   "management.cattle.io",
		Path:    "/v3",
		SubContexts: map[string]bool{
			"clusters": true,
		},
	}

	Schemas = factory.Schemas(&Version).
		Init(nodeTypes).
		Init(machineTypes).
		Init(authzTypes).
		Init(clusterTypes).
		Init(catalogTypes).
		Init(authnTypes).
		Init(schemaTypes)
)

func schemaTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.DynamicSchema{})
}

func catalogTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.Catalog{}).
		MustImport(&Version, v3.Template{}).
		MustImport(&Version, v3.TemplateVersion{})
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

func clusterTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.Cluster{},
			&m.Embed{Field: "status"},
		).
		AddMapperForType(&Version, v3.ClusterStatus{},
			m.Drop{"appliedSpec"},
		).
		AddMapperForType(&Version, v3.ClusterEvent{}, &m.Move{
			From: "type",
			To:   "eventType",
		}).
		MustImportAndCustomize(&Version, v3.Cluster{}, func(schema *types.Schema) {
			schema.SubContext = "clusters"
		}).
		MustImport(&Version, v3.ClusterEvent{}).
		MustImport(&Version, v3.ClusterRegistrationToken{})
}

func authzTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.Project{},
			m.DisplayName{},
		).
		AddMapperForType(&Version, v3.ProjectRoleTemplateBinding{},
			&m.Move{From: "subject/name", To: "subjectName"},
			&m.Move{From: "subject/kind", To: "subjectKind"},
			&m.Move{From: "subject/namespace", To: "subjectNamespace"},
			&m.Drop{Field: "subject"},
		).
		MustImportAndCustomize(&Version, v3.Project{}, func(schema *types.Schema) {
			schema.SubContext = "projects"
		}).
		MustImport(&Version, v3.RoleTemplate{}).
		MustImport(&Version, v3.PodSecurityPolicyTemplate{}).
		MustImport(&Version, v3.ClusterRoleTemplateBinding{}).
		MustImportAndCustomize(&Version, v3.ProjectRoleTemplateBinding{}, func(schema *types.Schema) {
			schema.MustCustomizeField("subjectKind", func(field types.Field) types.Field {
				field.Type = "enum"
				field.Options = []string{"User", "Group", "ServiceAccount"}
				field.Nullable = false
				return field
			})
		})
}

func machineTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.MachineSpec{}, &m.Embed{Field: "nodeSpec"}).
		AddMapperForType(&Version, v3.MachineStatus{},
			&m.Drop{Field: "conditions"},
			&m.Embed{Field: "nodeStatus"}).
		AddMapperForType(&Version, v3.Machine{},
			&m.Embed{Field: "status"},
			&m.Move{From: "name", To: "id"},
			&m.Move{From: "nodeName", To: "name"}).
		AddMapperForType(&Version, v3.MachineDriver{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.MachineTemplate{}, m.DisplayName{}).
		MustImport(&Version, v3.Machine{}).
		MustImport(&Version, v3.MachineDriver{}).
		MustImport(&Version, v3.MachineTemplate{})
}

func authnTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.Token{}).
		MustImport(&Version, v3.User{}).
		MustImport(&Version, v3.Group{}).
		MustImport(&Version, v3.GroupMember{}).
		MustImport(&Version, v3.Identity{}).
		MustImport(&Version, v3.LoginInput{}).
		MustImport(&Version, v3.LocalCredential{}).
		MustImport(&Version, v3.GithubCredential{})
}
