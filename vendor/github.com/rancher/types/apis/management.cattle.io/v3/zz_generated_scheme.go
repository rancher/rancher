package v3

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "management.cattle.io"
	Version   = "v3"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// Kind takes an unqualified kind and returns a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	// TODO this gets cleaned up when the types are fixed
	scheme.AddKnownTypes(SchemeGroupVersion,

		&NodePool{},
		&NodePoolList{},
		&Node{},
		&NodeList{},
		&NodeDriver{},
		&NodeDriverList{},
		&NodeTemplate{},
		&NodeTemplateList{},
		&Project{},
		&ProjectList{},
		&GlobalRole{},
		&GlobalRoleList{},
		&GlobalRoleBinding{},
		&GlobalRoleBindingList{},
		&RoleTemplate{},
		&RoleTemplateList{},
		&PodSecurityPolicyTemplate{},
		&PodSecurityPolicyTemplateList{},
		&PodSecurityPolicyTemplateProjectBinding{},
		&PodSecurityPolicyTemplateProjectBindingList{},
		&ClusterRoleTemplateBinding{},
		&ClusterRoleTemplateBindingList{},
		&ProjectRoleTemplateBinding{},
		&ProjectRoleTemplateBindingList{},
		&Cluster{},
		&ClusterList{},
		&ClusterEvent{},
		&ClusterEventList{},
		&ClusterRegistrationToken{},
		&ClusterRegistrationTokenList{},
		&Catalog{},
		&CatalogList{},
		&Template{},
		&TemplateList{},
		&TemplateVersion{},
		&TemplateVersionList{},
		&TemplateContent{},
		&TemplateContentList{},
		&Group{},
		&GroupList{},
		&GroupMember{},
		&GroupMemberList{},
		&Principal{},
		&PrincipalList{},
		&User{},
		&UserList{},
		&AuthConfig{},
		&AuthConfigList{},
		&OpenLdapTestAndApplyInput{},
		&OpenLdapTestAndApplyInputList{},
		&FreeIpaTestAndApplyInput{},
		&FreeIpaTestAndApplyInputList{},
		&Token{},
		&TokenList{},
		&DynamicSchema{},
		&DynamicSchemaList{},
		&Preference{},
		&PreferenceList{},
		&ProjectNetworkPolicy{},
		&ProjectNetworkPolicyList{},
		&ClusterLogging{},
		&ClusterLoggingList{},
		&ProjectLogging{},
		&ProjectLoggingList{},
		&ListenConfig{},
		&ListenConfigList{},
		&Setting{},
		&SettingList{},
		&Notifier{},
		&NotifierList{},
		&ClusterAlert{},
		&ClusterAlertList{},
		&ProjectAlert{},
		&ProjectAlertList{},
		&ClusterPipeline{},
		&ClusterPipelineList{},
		&SourceCodeCredential{},
		&SourceCodeCredentialList{},
		&Pipeline{},
		&PipelineList{},
		&PipelineExecution{},
		&PipelineExecutionList{},
		&PipelineExecutionLog{},
		&PipelineExecutionLogList{},
		&SourceCodeRepository{},
		&SourceCodeRepositoryList{},
		&ComposeConfig{},
		&ComposeConfigList{},
	)
	return nil
}
