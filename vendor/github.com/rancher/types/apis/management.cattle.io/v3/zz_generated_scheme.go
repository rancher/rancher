package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		&ClusterRegistrationToken{},
		&ClusterRegistrationTokenList{},
		&Catalog{},
		&CatalogList{},
		&Template{},
		&TemplateList{},
		&CatalogTemplate{},
		&CatalogTemplateList{},
		&CatalogTemplateVersion{},
		&CatalogTemplateVersionList{},
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
		&LdapConfig{},
		&LdapConfigList{},
		&Token{},
		&TokenList{},
		&DynamicSchema{},
		&DynamicSchemaList{},
		&Preference{},
		&PreferenceList{},
		&UserAttribute{},
		&ProjectNetworkPolicy{},
		&ProjectNetworkPolicyList{},
		&ClusterLogging{},
		&ClusterLoggingList{},
		&ProjectLogging{},
		&ProjectLoggingList{},
		&Setting{},
		&SettingList{},
		&Feature{},
		&FeatureList{},
		&ClusterAlert{},
		&ClusterAlertList{},
		&ProjectAlert{},
		&ProjectAlertList{},
		&Notifier{},
		&NotifierList{},
		&ClusterAlertGroup{},
		&ClusterAlertGroupList{},
		&ProjectAlertGroup{},
		&ProjectAlertGroupList{},
		&ClusterAlertRule{},
		&ClusterAlertRuleList{},
		&ProjectAlertRule{},
		&ProjectAlertRuleList{},
		&ComposeConfig{},
		&ComposeConfigList{},
		&ProjectCatalog{},
		&ProjectCatalogList{},
		&ClusterCatalog{},
		&ClusterCatalogList{},
		&MultiClusterApp{},
		&MultiClusterAppList{},
		&MultiClusterAppRevision{},
		&MultiClusterAppRevisionList{},
		&GlobalDNS{},
		&GlobalDNSList{},
		&GlobalDNSProvider{},
		&GlobalDNSProviderList{},
		&KontainerDriver{},
		&KontainerDriverList{},
		&EtcdBackup{},
		&EtcdBackupList{},
		&ClusterScan{},
		&ClusterScanList{},
		&MonitorMetric{},
		&MonitorMetricList{},
		&ClusterMonitorGraph{},
		&ClusterMonitorGraphList{},
		&ProjectMonitorGraph{},
		&ProjectMonitorGraphList{},
		&CloudCredential{},
		&CloudCredentialList{},
		&ClusterTemplate{},
		&ClusterTemplateList{},
		&ClusterTemplateRevision{},
		&ClusterTemplateRevisionList{},
		&RKEK8sSystemImage{},
		&RKEK8sSystemImageList{},
		&RKEK8sServiceOption{},
		&RKEK8sServiceOptionList{},
		&RKEAddon{},
		&RKEAddonList{},
		&CisConfig{},
		&CisConfigList{},
		&CisBenchmarkVersion{},
		&CisBenchmarkVersionList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
