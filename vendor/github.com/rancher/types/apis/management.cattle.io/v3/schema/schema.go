package schema

import (
	"net/http"

	"k8s.io/api/core/v1"

	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/factory"
	"github.com/rancher/types/mapper"
)

var (
	Version = types.APIVersion{
		Version: "v3",
		Group:   "management.cattle.io",
		Path:    "/v3",
	}

	Schemas = factory.Schemas(&Version).
		Init(nativeNodeTypes).
		Init(nodeTypes).
		Init(authzTypes).
		Init(clusterTypes).
		Init(catalogTypes).
		Init(authnTypes).
		Init(tokens).
		Init(schemaTypes).
		Init(userTypes).
		Init(projectNetworkPolicyTypes).
		Init(logTypes).
		Init(globalTypes).
		Init(rkeTypes).
		Init(alertTypes).
		Init(pipelineTypes)

	TokenSchemas = factory.Schemas(&Version).
			Init(tokens)
)

func rkeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.AddMapperForType(&Version, v3.BaseService{}, m.Drop{Field: "image"})
}

func schemaTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.DynamicSchema{})
}

func catalogTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.Catalog{},
			&m.Move{From: "catalogKind", To: "kind"},
			&m.Embed{Field: "status"},
			&m.Drop{Field: "helmVersionCommits"},
		).
		MustImportAndCustomize(&Version, v3.Catalog{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"refresh": {},
			}
			schema.CollectionActions = map[string]types.Action{
				"refresh": {},
			}
		}).
		MustImport(&Version, v3.Template{}).
		MustImport(&Version, v3.TemplateVersion{})
}

func nativeNodeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		TypeName("internalNodeStatus", v1.NodeStatus{}).
		TypeName("internalNodeSpec", v1.NodeSpec{}).
		AddMapperForType(&Version, v1.NodeStatus{},
			&mapper.NodeAddressMapper{},
			&mapper.OSInfo{},
			&m.Drop{Field: "addresses"},
			&m.Drop{Field: "daemonEndpoints"},
			&m.Drop{Field: "images"},
			&m.Drop{Field: "nodeInfo"},
			&m.Move{From: "conditions", To: "nodeConditions"},
			&m.Drop{Field: "phase"},
			&m.SliceToMap{Field: "volumesAttached", Key: "devicePath"},
		).
		AddMapperForType(&Version, v1.NodeSpec{},
			&m.Drop{Field: "externalID"},
			&m.Drop{Field: "configSource"},
			&m.Move{From: "providerID", To: "providerId"},
			&m.Move{From: "podCIDR", To: "podCidr"},
			m.Access{Fields: map[string]string{
				"podCidr":       "r",
				"providerId":    "r",
				"taints":        "ru",
				"unschedulable": "ru",
			}}).
		MustImportAndCustomize(&Version, v1.NodeSpec{}, func(schema *types.Schema) {
			schema.CodeName = "InternalNodeSpec"
			schema.CodeNamePlural = "InternalNodeSpecs"
		}).
		MustImportAndCustomize(&Version, v1.NodeStatus{}, func(schema *types.Schema) {
			schema.CodeName = "InternalNodeStatus"
			schema.CodeNamePlural = "InternalNodeStatuses"
		}, struct {
			IPAddress string
			Hostname  string
			Info      NodeInfo
		}{})
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
		AddMapperForType(&Version, v3.ClusterRegistrationToken{},
			&m.Embed{Field: "status"},
		).
		AddMapperForType(&Version, v3.RancherKubernetesEngineConfig{},
			m.Drop{Field: "systemImages"},
		).
		MustImport(&Version, v3.Cluster{}).
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
		MustImport(&Version, v3.Project{}).
		MustImport(&Version, v3.GlobalRole{}).
		MustImport(&Version, v3.GlobalRoleBinding{}).
		MustImport(&Version, v3.RoleTemplate{}).
		MustImport(&Version, v3.PodSecurityPolicyTemplate{}).
		MustImport(&Version, v3.ClusterRoleTemplateBinding{}).
		MustImport(&Version, v3.ProjectRoleTemplateBinding{}).
		MustImport(&Version, v3.GlobalRoleBinding{})
}

func nodeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.NodeSpec{}, &m.Embed{Field: "internalNodeSpec"}).
		AddMapperForType(&Version, v3.NodeStatus{},
			&m.Drop{Field: "nodeTemplateSpec"},
			&m.Embed{Field: "internalNodeStatus"},
			&m.SliceMerge{From: []string{"conditions", "nodeConditions"}, To: "conditions"}).
		AddMapperForType(&Version, v3.Node{},
			&m.Embed{Field: "status"},
			&m.Move{From: "rkeNode/user", To: "sshUser"},
			&m.ReadOnly{Field: "sshUser"},
			&m.Drop{Field: "rkeNode"},
			&m.Drop{Field: "labels"},
			&m.Drop{Field: "annotations"},
			&m.Move{From: "nodeLabels", To: "labels"},
			&m.Move{From: "nodeAnnotations", To: "annotations"},
			&m.Drop{Field: "desiredNodeLabels"},
			&m.Drop{Field: "desiredNodeAnnotations"},
			m.DisplayName{}).
		AddMapperForType(&Version, v3.NodeDriver{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.NodeTemplate{}, m.DisplayName{}).
		MustImport(&Version, v3.NodePool{}).
		MustImportAndCustomize(&Version, v3.Node{}, func(schema *types.Schema) {
			labelField := schema.ResourceFields["labels"]
			labelField.Create = true
			labelField.Update = true
			schema.ResourceFields["labels"] = labelField
		}).
		MustImportAndCustomize(&Version, v3.NodeDriver{}, func(schema *types.Schema) {
			schema.ResourceActions["activate"] = types.Action{
				Output: "nodeDriver",
			}
			schema.ResourceActions["deactivate"] = types.Action{
				Output: "nodeDriver",
			}
		}).
		MustImportAndCustomize(&Version, v3.NodeTemplate{}, func(schema *types.Schema) {
			delete(schema.ResourceFields, "namespaceId")
		})
}

func tokens(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImportAndCustomize(&Version, v3.Token{}, func(schema *types.Schema) {
			schema.CollectionActions = map[string]types.Action{
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
		AddMapperForType(&Version, v3.Principal{}, m.DisplayName{}).
		MustImportAndCustomize(&Version, v3.Principal{}, func(schema *types.Schema) {
			schema.CollectionMethods = []string{http.MethodGet}
			schema.ResourceMethods = []string{http.MethodGet}
			schema.CollectionActions = map[string]types.Action{
				"search": {
					Input:  "searchPrincipalsInput",
					Output: "collection",
				},
			}
		}).
		MustImport(&Version, v3.SearchPrincipalsInput{}).
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
		// Local Config
		MustImportAndCustomize(&Version, v3.LocalConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet}
		}).
		//Github Config
		MustImportAndCustomize(&Version, v3.GithubConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"configureTest": {
					Input:  "githubConfig",
					Output: "githubConfigTestOutput",
				},
				"testAndApply": {
					Input: "githubConfigApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		}).
		MustImport(&Version, v3.GithubConfigTestOutput{}).
		MustImport(&Version, v3.GithubConfigApplyInput{}).
		// Active Directory Config
		MustImportAndCustomize(&Version, v3.ActiveDirectoryConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"testAndApply": {
					Input: "activeDirectoryTestAndApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		}).
		MustImport(&Version, v3.ActiveDirectoryTestAndApplyInput{})

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

func projectNetworkPolicyTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		MustImportAndCustomize(&Version, v3.ProjectNetworkPolicy{}, func(schema *types.Schema) {
			schema.CollectionMethods = []string{http.MethodGet}
			schema.ResourceMethods = []string{http.MethodGet}
		})
}

func logTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		AddMapperForType(&Version, v3.ClusterLogging{},
			m.DisplayName{}).
		AddMapperForType(&Version, v3.ProjectLogging{},
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

func alertTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		AddMapperForType(&Version, v3.Notifier{},
			m.DisplayName{}).
		AddMapperForType(&Version, v3.ClusterAlert{},
			&m.Embed{Field: "status"},
			m.DisplayName{}).
		AddMapperForType(&Version, v3.ProjectAlert{},
			&m.Embed{Field: "status"},
			m.DisplayName{}).
		MustImport(&Version, v3.Notification{}).
		MustImportAndCustomize(&Version, v3.Notifier{}, func(schema *types.Schema) {
			schema.CollectionActions = map[string]types.Action{
				"send": {
					Input: "notification",
				},
			}
			schema.ResourceActions = map[string]types.Action{
				"send": {
					Input: "notification",
				},
			}
		}).
		MustImportAndCustomize(&Version, v3.ClusterAlert{}, func(schema *types.Schema) {

			schema.ResourceActions = map[string]types.Action{
				"activate":   {},
				"deactivate": {},
				"mute":       {},
				"unmute":     {},
			}
		}).
		MustImportAndCustomize(&Version, v3.ProjectAlert{}, func(schema *types.Schema) {

			schema.ResourceActions = map[string]types.Action{
				"activate":   {},
				"deactivate": {},
				"mute":       {},
				"unmute":     {},
			}
		})

}

func pipelineTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		AddMapperForType(&Version, v3.ClusterPipeline{}).
		AddMapperForType(&Version, v3.Pipeline{},
			&m.Embed{Field: "status"},
			m.DisplayName{}).
		AddMapperForType(&Version, v3.PipelineExecution{},
			&m.Embed{Field: "status"}).
		AddMapperForType(&Version, v3.SourceCodeCredential{}).
		AddMapperForType(&Version, v3.SourceCodeRepository{}).
		AddMapperForType(&Version, v3.PipelineExecutionLog{}).
		MustImport(&Version, v3.AuthAppInput{}).
		MustImport(&Version, v3.AuthUserInput{}).
		MustImportAndCustomize(&Version, v3.SourceCodeCredential{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"refreshrepos": {},
			}
		}).
		MustImportAndCustomize(&Version, v3.ClusterPipeline{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"deploy":  {},
				"destroy": {},
				"authapp": {
					Input:  "authAppInput",
					Output: "cluterPipeline",
				},
				"revokeapp": {},
				"authuser": {
					Input:  "authUserInput",
					Output: "sourceCodeCredential",
				},
			}
		}).
		MustImportAndCustomize(&Version, v3.Pipeline{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"activate":   {},
				"deactivate": {},
				"run":        {},
			}
		}).
		MustImportAndCustomize(&Version, v3.PipelineExecution{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"stop":  {},
				"rerun": {},
			}
		}).
		MustImport(&Version, v3.SourceCodeRepository{}).
		MustImport(&Version, v3.PipelineExecutionLog{})

}
