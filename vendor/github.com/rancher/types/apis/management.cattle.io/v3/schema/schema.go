package schema

import (
	"net/http"

	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/factory"
	"github.com/rancher/types/mapper"
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
		Init(tokens).
		Init(schemaTypes).
		Init(stackTypes).
		Init(userTypes).
		Init(logTypes).
		Init(globalTypes)

	TokenSchema = factory.Schemas(&Version).
			Init(tokens)
)

func schemaTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.DynamicSchema{})
}

func catalogTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.Catalog{},
			&m.Move{From: "catalogKind", To: "kind"},
		).
		MustImport(&Version, v3.Catalog{}).
		MustImport(&Version, v3.Template{}).
		MustImport(&Version, v3.TemplateVersion{})
}

func nodeTypes(schemas *types.Schemas) *types.Schemas {
	return schema.NodeTypes(&Version, schemas)
}

func clusterTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.Cluster{},
			&m.Embed{Field: "status"},
			m.DisplayName{},
		).
		AddMapperForType(&Version, v3.ClusterStatus{},
			m.Drop{Field: "serviceAccountToken"},
			m.Drop{Field: "appliedSpec"},
			m.Drop{Field: "clusterName"},
		).
		AddMapperForType(&Version, v3.ClusterEvent{}, &m.Move{
			From: "type",
			To:   "eventType",
		}).
		MustImportAndCustomize(&Version, v3.Cluster{}, func(schema *types.Schema) {
			schema.SubContext = "clusters"
		}).
		MustImport(&Version, v3.ClusterEvent{}).
		MustImport(&Version, v3.ClusterRegistrationToken{}).
		MustImportAndCustomize(&Version, v3.Cluster{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabel"
				field.Nullable = true
				field.Required = false
				return field
			})
		})
}

func authzTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.ProjectStatus{}).
		AddMapperForType(&Version, v3.Project{}, m.DisplayName{},
			&m.Embed{Field: "status"}).
		AddMapperForType(&Version, v3.GlobalRole{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.RoleTemplate{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.ProjectRoleTemplateBinding{},
			&mapper.NamespaceIDMapper{},
		).
		MustImportAndCustomize(&Version, v3.Project{}, func(schema *types.Schema) {
			schema.SubContext = "projects"
		}).
		MustImport(&Version, v3.GlobalRole{}).
		MustImport(&Version, v3.GlobalRoleBinding{}).
		MustImport(&Version, v3.RoleTemplate{}).
		MustImport(&Version, v3.PodSecurityPolicyTemplate{}).
		MustImport(&Version, v3.ClusterRoleTemplateBinding{}).
		MustImport(&Version, v3.ProjectRoleTemplateBinding{}).
		MustImport(&Version, v3.GlobalRoleBinding{})
}

func machineTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.MachineSpec{}, &m.Embed{Field: "nodeSpec"}).
		AddMapperForType(&Version, v3.MachineStatus{},
			&m.Drop{Field: "token"},
			&m.Drop{Field: "rkeNode"},
			&m.Drop{Field: "machineTemplateSpec"},
			&m.Drop{Field: "machineDriverConfig"},
			&m.Embed{Field: "nodeStatus"},
			&m.SliceMerge{From: []string{"conditions", "nodeConditions"}, To: "conditions"}).
		AddMapperForType(&Version, v3.MachineConfig{},
			&m.Drop{Field: "clusterName"}).
		AddMapperForType(&Version, v3.Machine{},
			&m.Embed{Field: "status"},
			m.DisplayName{}).
		AddMapperForType(&Version, v3.MachineDriver{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.MachineTemplate{}, m.DisplayName{}).
		MustImport(&Version, v3.Machine{}).
		MustImportAndCustomize(&Version, v3.MachineDriver{}, func(schema *types.Schema) {
			schema.ResourceActions["activate"] = types.Action{
				Output: "machineDriver",
			}
			schema.ResourceActions["deactivate"] = types.Action{
				Output: "machineDriver",
			}
		}).
		MustImport(&Version, v3.MachineTemplate{})

}

func tokens(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.LoginInput{}).
		MustImport(&Version, v3.LocalCredential{}).
		MustImport(&Version, v3.GithubCredential{}).
		MustImportAndCustomize(&Version, v3.Token{}, func(schema *types.Schema) {
			schema.CollectionActions = map[string]types.Action{
				"login": {
					Input:  "loginInput",
					Output: "token",
				},
				"logout": {},
			}
		})
}

func authnTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.User{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.Group{}, m.DisplayName{}).
		MustImport(&Version, v3.Group{}).
		MustImport(&Version, v3.GroupMember{}).
		MustImport(&Version, v3.Principal{}).
		MustImport(&Version, v3.ChangePasswordInput{}).
		MustImport(&Version, v3.SetPasswordInput{}).
		MustImportAndCustomize(&Version, v3.User{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"setpassword": {
					Input:  "setPasswordInput",
					Output: "user",
				},
			}
			schema.CollectionActions = map[string]types.Action{
				"changepassword": {
					Input: "changePasswordInput",
				},
			}
		}).
		MustImportAndCustomize(&Version, v3.AuthConfig{}, func(schema *types.Schema) {
			schema.CollectionMethods = []string{http.MethodGet}
		}).
		MustImportAndCustomize(&Version, v3.GithubConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"configureTest": {
					Input:  "githubConfigTestInput",
					Output: "githubConfig",
				},
				"testAndApply": {
					Input:  "githubConfigApplyInput",
					Output: "githubConfig",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet}
		}).
		MustImport(&Version, v3.GithubConfigTestInput{}).
		MustImport(&Version, v3.GithubConfigApplyInput{}).
		MustImportAndCustomize(&Version, v3.LocalConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet}
		})
}

func stackTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		MustImportAndCustomize(&Version, v3.Stack{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"upgrade": {
					Input: "templateVersionId",
				},
				"rollback": {
					Input: "revision",
				},
			}
		})
}

func userTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		MustImportAndCustomize(&Version, v3.Preference{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(f types.Field) types.Field {
				f.Required = true
				return f
			})
			schema.MustCustomizeField("namespaceId", func(f types.Field) types.Field {
				f.Required = false
				return f
			})
		})
}

func logTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		AddMapperForType(&Version, &v3.ClusterLogging{},
			m.DisplayName{}).
		AddMapperForType(&Version, &v3.ProjectLogging{},
			m.DisplayName{}).
		MustImport(&Version, v3.ClusterLogging{}).
		MustImport(&Version, v3.ProjectLogging{})
}

func globalTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		AddMapperForType(&Version, v3.ListenConfig{},
			m.DisplayName{},
			m.Drop{Field: "caKey"},
			m.Drop{Field: "caCert"},
		).
		MustImport(&Version, v3.ListenConfig{}).
		MustImportAndCustomize(&Version, v3.Setting{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(f types.Field) types.Field {
				f.Required = true
				return f
			})
		})
}
