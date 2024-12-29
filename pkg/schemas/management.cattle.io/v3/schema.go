package schema

import (
	"net/http"

	rketypes "github.com/rancher/rke/types"

	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/schemas/factory"
	"github.com/rancher/rancher/pkg/schemas/mapper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/apis/apiserver"
)

var (
	Version = types.APIVersion{
		Version: "v3",
		Group:   "management.cattle.io",
		Path:    "/v3",
	}

	AuthSchemas = factory.Schemas(&Version).
			Init(authnTypes).
			Init(tokens).
			Init(userTypes)

	Schemas = factory.Schemas(&Version).
		Init(nativeNodeTypes).
		Init(nodeTypes).
		Init(podSecurityAdmissionTypes).
		Init(authzTypes).
		Init(clusterTypes).
		Init(authnTypes).
		Init(tokens).
		Init(schemaTypes).
		Init(userTypes).
		Init(projectNetworkPolicyTypes).
		Init(globalTypes).
		Init(rkeTypes).
		Init(composeType).
		Init(kontainerTypes).
		Init(etcdBackupTypes).
		Init(credTypes).
		Init(mgmtSecretTypes).
		Init(clusterTemplateTypes).
		Init(driverMetadataTypes).
		Init(encryptionTypes).
		Init(fleetTypes).
		Init(notificationTypes)

	TokenSchemas = factory.Schemas(&Version).
			Init(tokens)
)

func fleetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, v3.FleetWorkspace{})
}

func rkeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.AddMapperForType(&Version, rketypes.BaseService{}, m.Drop{Field: "image"}).
		AddMapperForType(&Version, v1.Taint{},
			m.Enum{Field: "effect", Options: []string{
				string(v1.TaintEffectNoSchedule),
				string(v1.TaintEffectPreferNoSchedule),
				string(v1.TaintEffectNoExecute),
			}},
			m.Required{Fields: []string{
				"effect",
				"value",
				"key",
			}},
			m.ReadOnly{Field: "timeAdded"},
		).
		MustImport(&Version, rketypes.ExtraEnv{}).
		MustImport(&Version, rketypes.ExtraVolume{}).
		MustImport(&Version, rketypes.ExtraVolumeMount{}).
		MustImport(&Version, rketypes.LinearAutoscalerParams{}).
		MustImport(&Version, rketypes.DeploymentStrategy{}).
		MustImport(&Version, rketypes.DaemonSetUpdateStrategy{})
}

func schemaTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.DynamicSchema{})
}

func credTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.CloudCredential{},
			&m.DisplayName{},
			&mapper.CredentialMapper{},
			&m.AnnotationField{Field: "name"},
			&m.AnnotationField{Field: "description"},
			&m.Drop{Field: "namespaceId"}).
		MustImport(&Version, v3.CloudCredential{})
}

func mgmtSecretTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImportAndCustomize(&Version, v1.Secret{}, func(schema *types.Schema) {
		schema.ID = "managementSecret"
		schema.PluralName = "managementSecrets"
		schema.CodeName = "ManagementSecret"
		schema.CodeNamePlural = "ManagementSecrets"
		schema.MustCustomizeField("name", func(field types.Field) types.Field {
			field.Type = "hostname"
			field.Nullable = false
			field.Required = true
			return field
		})
	})
}

func driverMetadataTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.RkeK8sSystemImage{}, m.Drop{Field: "namespaceId"}).
		AddMapperForType(&Version, v3.RkeK8sServiceOption{}, m.Drop{Field: "namespaceId"}).
		AddMapperForType(&Version, v3.RkeAddon{}, m.Drop{Field: "namespaceId"}).
		MustImport(&Version, v3.RkeK8sSystemImage{}).
		MustImport(&Version, v3.RkeK8sServiceOption{}).
		MustImport(&Version, v3.RkeAddon{})
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
			&m.Move{From: "podCIDRs", To: "podCidrs"},
			m.Access{Fields: map[string]string{
				"podCidr":       "r",
				"podCidrs":      "r",
				"providerId":    "r",
				"taints":        "ru",
				"unschedulable": "ru",
			}}).
		AddMapperForType(&Version, v1.Node{},
			&mapper.NodeAddressAnnotationMapper{}).
		MustImportAndCustomize(&Version, v1.NodeSpec{}, func(schema *types.Schema) {
			schema.CodeName = "InternalNodeSpec"
			schema.CodeNamePlural = "InternalNodeSpecs"
		}).
		MustImportAndCustomize(&Version, v1.NodeStatus{}, func(schema *types.Schema) {
			schema.CodeName = "InternalNodeStatus"
			schema.CodeNamePlural = "InternalNodeStatuses"
		}, struct {
			IPAddress         string
			ExternalIPAddress string `json:"externalIpAddress,omitempty"`
			Hostname          string
			Info              NodeInfo
		}{})
}

func clusterTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.Cluster{},
			&m.Embed{Field: "status"},
			mapper.NewDropFromSchema("genericEngineConfig"),
			mapper.NewDropFromSchema("googleKubernetesEngineConfig"),
			mapper.NewDropFromSchema("azureKubernetesServiceConfig"),
			mapper.NewDropFromSchema("amazonElasticContainerServiceConfig"),
			m.DisplayName{},
		).
		AddMapperForType(&Version, v3.ClusterStatus{},
			m.Drop{Field: "serviceAccountToken"},
		).
		AddMapperForType(&Version, v3.ClusterRegistrationToken{},
			&m.Embed{Field: "status"},
		).
		AddMapperForType(&Version, rketypes.RancherKubernetesEngineConfig{},
			m.Drop{Field: "systemImages"},
		).
		MustImport(&Version, v3.Cluster{}).
		MustImport(&Version, v3.ClusterRegistrationToken{}).
		MustImport(&Version, v3.GenerateKubeConfigOutput{}).
		MustImport(&Version, v3.ImportClusterYamlInput{}).
		MustImport(&Version, v3.RotateCertificateInput{}).
		MustImport(&Version, v3.RotateCertificateOutput{}).
		MustImport(&Version, v3.RotateEncryptionKeyOutput{}).
		MustImport(&Version, v3.ImportYamlOutput{}).
		MustImport(&Version, v3.ExportOutput{}).
		MustImport(&Version, v3.RestoreFromEtcdBackupInput{}).
		MustImport(&Version, v3.SaveAsTemplateInput{}).
		MustImport(&Version, v3.SaveAsTemplateOutput{}).
		AddMapperForType(&Version, v1.EnvVar{},
			&m.Move{
				From: "envVar",
				To:   "agentEnvVar",
			}).
		MustImportAndCustomize(&Version, rketypes.ETCDService{}, func(schema *types.Schema) {
			schema.MustCustomizeField("extraArgs", func(field types.Field) types.Field {
				field.Default = map[string]interface{}{
					"election-timeout":   "5000",
					"heartbeat-interval": "500"}
				return field
			})
		}).
		MustImportAndCustomize(&Version, v3.Cluster{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabel"
				field.Nullable = true
				field.Required = false
				return field
			})
			schema.ResourceActions[v3.ClusterActionGenerateKubeconfig] = types.Action{
				Output: "generateKubeConfigOutput",
			}
			schema.ResourceActions[v3.ClusterActionImportYaml] = types.Action{
				Input:  "importClusterYamlInput",
				Output: "importYamlOutput",
			}
			schema.ResourceActions[v3.ClusterActionExportYaml] = types.Action{
				Output: "exportOutput",
			}
			schema.ResourceActions[v3.ClusterActionBackupEtcd] = types.Action{}
			schema.ResourceActions[v3.ClusterActionRestoreFromEtcdBackup] = types.Action{
				Input: "restoreFromEtcdBackupInput",
			}
			schema.ResourceActions[v3.ClusterActionRotateCertificates] = types.Action{
				Input:  "rotateCertificateInput",
				Output: "rotateCertificateOutput",
			}
			schema.ResourceActions[v3.ClusterActionRotateEncryptionKey] = types.Action{
				Output: "rotateEncryptionKeyOutput",
			}
			schema.ResourceActions[v3.ClusterActionSaveAsTemplate] = types.Action{
				Input:  "saveAsTemplateInput",
				Output: "saveAsTemplateOutput",
			}
		})
}

func podSecurityAdmissionTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, v3.PodSecurityAdmissionConfigurationTemplate{})
}

func authzTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v3.ProjectStatus{}).
		AddMapperForType(&Version, v3.Project{},
			m.DisplayName{},
			&m.Embed{Field: "status"},
		).
		AddMapperForType(&Version, v3.GlobalRole{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.RoleTemplate{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.ProjectRoleTemplateBinding{},
			&mapper.NamespaceIDMapper{},
		).
		MustImport(&Version, v3.ImportYamlOutput{}).
		MustImportAndCustomize(&Version, v3.Project{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"exportYaml": {},
			}
		}).
		MustImportAndCustomize(&Version, v3.GlobalRole{}, func(s *types.Schema) {
			s.MustCustomizeField("status", func(field types.Field) types.Field {
				field.Nullable = false
				return field
			})
		}).
		MustImport(&Version, v3.GlobalRoleBinding{}).
		MustImport(&Version, v3.RoleTemplate{}).
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
			&m.Drop{Field: "config"},
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
			&m.Drop{Field: "desiredNodeTaints"},
			&m.Drop{Field: "metadataUpdate"},
			&m.Drop{Field: "updateTaintsFromAPI"},
			&m.Drop{Field: "desiredNodeUnschedulable"},
			&m.Drop{Field: "nodeDrainInput"},
			&m.AnnotationField{Field: "publicEndpoints", List: true},
			m.Copy{From: "namespaceId", To: "clusterName"},
			m.DisplayName{}).
		AddMapperForType(&Version, v3.NodeDriver{}, m.DisplayName{}).
		AddMapperForType(&Version, v3.NodeTemplate{}, m.DisplayName{}).
		MustImport(&Version, v3.PublicEndpoint{}).
		MustImportAndCustomize(&Version, v3.NodePool{}, func(schema *types.Schema) {
			schema.ResourceFields["driver"] = types.Field{
				Type:     "string",
				CodeName: "Driver",
				Create:   false,
				Update:   false,
			}
		}).
		MustImport(&Version, v3.NodeDrainInput{}).
		MustImportAndCustomize(&Version, v3.Node{}, func(schema *types.Schema) {
			labelField := schema.ResourceFields["labels"]
			labelField.Create = true
			labelField.Update = true
			schema.ResourceFields["labels"] = labelField
			annotationField := schema.ResourceFields["annotations"]
			annotationField.Create = true
			annotationField.Update = true
			schema.ResourceFields["annotations"] = annotationField
			unschedulable := schema.ResourceFields["unschedulable"]
			unschedulable.Create = false
			unschedulable.Update = false
			schema.ResourceFields["unschedulable"] = unschedulable
			clusterField := schema.ResourceFields["clusterId"]
			clusterField.Type = "reference[cluster]"
			schema.ResourceFields["clusterId"] = clusterField
			schema.ResourceActions["cordon"] = types.Action{}
			schema.ResourceActions["uncordon"] = types.Action{}
			schema.ResourceActions["stopDrain"] = types.Action{}
			schema.ResourceActions["scaledown"] = types.Action{}
			schema.ResourceActions["drain"] = types.Action{
				Input: "nodeDrainInput",
			}
		}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{}).
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
				"logoutAll": {
					Input:  "samlConfigLogoutInput",
					Output: "samlConfigLogoutOutput",
				},
			}
		}).
		MustImport(&Version, v3.SamlConfigLogoutInput{}).
		MustImport(&Version, v3.SamlConfigLogoutOutput{})
}

func authnTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.User{}, m.DisplayName{},
			&m.Embed{Field: "status"}).
		AddMapperForType(&Version, v3.Group{}, m.DisplayName{}).
		MustImport(&Version, v3.Group{}).
		MustImport(&Version, v3.GroupMember{}).
		MustImport(&Version, v3.SamlToken{}).
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
				"refreshauthprovideraccess": {},
			}
			schema.CollectionActions = map[string]types.Action{
				"changepassword": {
					Input: "changePasswordInput",
				},
				"refreshauthprovideraccess": {},
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
		//AzureAD Config
		MustImportAndCustomize(&Version, v3.AzureADConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"configureTest": {
					Input:  "azureADConfig",
					Output: "azureADConfigTestOutput",
				},
				"testAndApply": {
					Input: "azureADConfigApplyInput",
				},
				"upgrade": {},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		}).
		MustImport(&Version, v3.AzureADConfigTestOutput{}).
		MustImport(&Version, v3.AzureADConfigApplyInput{}).
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
		MustImport(&Version, v3.ActiveDirectoryTestAndApplyInput{}).
		// OpenLdap Config
		MustImportAndCustomize(&Version, v3.OpenLdapConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"testAndApply": {
					Input: "openLdapTestAndApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		}).
		MustImport(&Version, v3.OpenLdapTestAndApplyInput{}).
		// FreeIpa Config
		AddMapperForType(&Version, v3.FreeIpaConfig{}, m.Drop{Field: "nestedGroupMembershipEnabled"}).
		MustImportAndCustomize(&Version, v3.FreeIpaConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"testAndApply": {
					Input: "freeIpaTestAndApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
			schema.MustCustomizeField("groupObjectClass", func(f types.Field) types.Field {
				f.Default = "groupofnames"
				return f
			})
			schema.MustCustomizeField("userNameAttribute", func(f types.Field) types.Field {
				f.Default = "givenName"
				return f
			})
			schema.MustCustomizeField("userObjectClass", func(f types.Field) types.Field {
				f.Default = "inetorgperson"
				return f
			})
			schema.MustCustomizeField("groupDNAttribute", func(f types.Field) types.Field {
				f.Default = "entrydn"
				return f
			})
			schema.MustCustomizeField("groupMemberUserAttribute", func(f types.Field) types.Field {
				f.Default = "entrydn"
				return f
			})
		}).
		MustImport(&Version, v3.FreeIpaTestAndApplyInput{}).
		// Saml Configs (ping adfs, keycloak, okta, shibboleth)
		MustImportAndCustomize(&Version, v3.PingConfig{}, configSchema).
		MustImportAndCustomize(&Version, v3.ADFSConfig{}, configSchema).
		MustImportAndCustomize(&Version, v3.KeyCloakConfig{}, configSchema).
		MustImportAndCustomize(&Version, v3.OKTAConfig{}, configSchema).
		MustImportAndCustomize(&Version, v3.ShibbolethConfig{}, configSchema).
		MustImport(&Version, v3.SamlConfigTestInput{}).
		MustImport(&Version, v3.SamlConfigTestOutput{}).
		//GoogleOAuth Config
		MustImportAndCustomize(&Version, v3.GoogleOauthConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"configureTest": {
					Input:  "googleOauthConfig",
					Output: "googleOauthConfigTestOutput",
				},
				"testAndApply": {
					Input: "googleOauthConfigApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		}).
		MustImport(&Version, v3.GoogleOauthConfigApplyInput{}).
		MustImport(&Version, v3.GoogleOauthConfigTestOutput{}).
		//OIDC Config
		MustImportAndCustomize(&Version, v3.OIDCConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"configureTest": {
					Input:  "oidcConfig",
					Output: "oidcTestOutput",
				},
				"testAndApply": {
					Input: "oidcApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		}).
		MustImport(&Version, v3.OIDCApplyInput{}).
		MustImport(&Version, v3.OIDCTestOutput{}).
		MustImportAndCustomize(&Version, v3.GenericOIDCConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"configureTest": {
					Input:  "genericOIDCConfig",
					Output: "genericOIDCTestOutput",
				},
				"testAndApply": {
					Input: "genericOIDCApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		}).
		MustImport(&Version, v3.GenericOIDCApplyInput{}).
		MustImport(&Version, v3.GenericOIDCTestOutput{}).
		//KeyCloakOIDC Config
		MustImportAndCustomize(&Version, v3.KeyCloakOIDCConfig{}, func(schema *types.Schema) {
			schema.BaseType = "authConfig"
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"configureTest": {
					Input:  "keyCloakOidcConfig",
					Output: "keyCloakOidcTestOutput",
				},
				"testAndApply": {
					Input: "keyCloakOidcApplyInput",
				},
			}
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
		})
}

// Common SAML schema configuration
func configSchema(schema *types.Schema) {
	schema.BaseType = "authConfig"
	schema.ResourceActions = map[string]types.Action{
		"disable": {},
		"testAndEnable": {
			Input:  "samlConfigTestInput",
			Output: "samlConfigTestOutput",
		},
	}
	schema.CollectionMethods = []string{}
	schema.ResourceMethods = []string{http.MethodGet, http.MethodPut}
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
		}).
		MustImportAndCustomize(&Version, v3.UserAttribute{}, func(schema *types.Schema) {
			schema.CollectionMethods = []string{}
			schema.ResourceMethods = []string{}
			// UserAttribute is currently unstructured and norman is unaware of the Duration type
			// which requires us to explicitly customize user retention fields
			// to be treated as strings.
			// The validation of these fields is done in the webhook.
			// Once transitioned to the structured UserAttribute, this should be removed.
			schema.MustCustomizeField("disableAfter", func(f types.Field) types.Field {
				f.Type = "string"
				return f
			})
			schema.MustCustomizeField("deleteAfter", func(f types.Field) types.Field {
				f.Type = "string"
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

func globalTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		MustImportAndCustomize(&Version, v3.Setting{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(f types.Field) types.Field {
				f.Required = true
				return f
			})
		}).
		MustImportAndCustomize(&Version, v3.Feature{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(f types.Field) types.Field {
				f.Required = true
				return f
			})
		})
}

func composeType(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, v3.ComposeConfig{})
}

func kontainerTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.KontainerDriver{},
			&m.Embed{Field: "status"},
			m.DisplayName{},
		).
		MustImportAndCustomize(&Version, v3.KontainerDriver{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"activate":   {},
				"deactivate": {},
			}
			schema.CollectionActions = map[string]types.Action{
				"refresh": {},
			}
		})
}

func etcdBackupTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, v3.EtcdBackup{})
}

func clusterTemplateTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		TypeName("clusterTemplate", v3.ClusterTemplate{}).
		TypeName("clusterTemplateRevision", v3.ClusterTemplateRevision{}).
		AddMapperForType(&Version, v3.ClusterTemplate{}, m.Drop{Field: "namespaceId"}, m.DisplayName{}).
		AddMapperForType(&Version, v3.ClusterTemplateRevision{},
			m.Drop{Field: "namespaceId"},
			&m.Embed{Field: "status"},
			m.DisplayName{}).
		MustImport(&Version, v3.ClusterTemplateQuestionsOutput{}).
		MustImport(&Version, v3.ClusterTemplate{}).
		MustImportAndCustomize(&Version, v3.ClusterTemplateRevision{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"disable": {},
				"enable":  {},
			}
			schema.CollectionActions = map[string]types.Action{
				"listquestions": {
					Output: "clusterTemplateQuestionsOutput",
				},
			}
		})

}

func encryptionTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, rketypes.SecretsEncryptionConfig{}).
		MustImport(&Version, apiserver.Key{}, struct {
			Secret string `norman:"type=password"`
		}{}).MustImport(&Version, apiserver.KMSConfiguration{}, struct {
		Timeout string
	}{})
}

func notificationTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, v3.RancherUserNotification{})
}
