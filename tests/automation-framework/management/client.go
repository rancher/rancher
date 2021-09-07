package management

import (
	"github.com/rancher/norman/clientbase"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/tests/automation-framework/cloudcredentials"
)

type Client struct {
	APIBaseClient                           clientbase.APIBaseClient
	NodePool                                managementClient.NodePoolOperations
	Node                                    managementClient.NodeOperations
	NodeDriver                              managementClient.NodeDriverOperations
	NodeTemplate                            managementClient.NodeTemplateOperations
	Project                                 managementClient.ProjectOperations
	GlobalRole                              managementClient.GlobalRoleOperations
	GlobalRoleBinding                       managementClient.GlobalRoleBindingOperations
	RoleTemplate                            managementClient.RoleTemplateOperations
	PodSecurityPolicyTemplate               managementClient.PodSecurityPolicyTemplateOperations
	PodSecurityPolicyTemplateProjectBinding managementClient.PodSecurityPolicyTemplateProjectBindingOperations
	ClusterRoleTemplateBinding              managementClient.ClusterRoleTemplateBindingOperations
	ProjectRoleTemplateBinding              managementClient.ProjectRoleTemplateBindingOperations
	Cluster                                 managementClient.ClusterOperations
	ClusterRegistrationToken                managementClient.ClusterRegistrationTokenOperations
	Catalog                                 managementClient.CatalogOperations
	Template                                managementClient.TemplateOperations
	CatalogTemplate                         managementClient.CatalogTemplateOperations
	CatalogTemplateVersion                  managementClient.CatalogTemplateVersionOperations
	TemplateVersion                         managementClient.TemplateVersionOperations
	TemplateContent                         managementClient.TemplateContentOperations
	Group                                   managementClient.GroupOperations
	GroupMember                             managementClient.GroupMemberOperations
	SamlToken                               managementClient.SamlTokenOperations
	Principal                               managementClient.PrincipalOperations
	User                                    managementClient.UserOperations
	AuthConfig                              managementClient.AuthConfigOperations
	LdapConfig                              managementClient.LdapConfigOperations
	Token                                   managementClient.TokenOperations
	DynamicSchema                           managementClient.DynamicSchemaOperations
	Preference                              managementClient.PreferenceOperations
	ProjectNetworkPolicy                    managementClient.ProjectNetworkPolicyOperations
	ClusterLogging                          managementClient.ClusterLoggingOperations
	ProjectLogging                          managementClient.ProjectLoggingOperations
	Setting                                 managementClient.SettingOperations
	Feature                                 managementClient.FeatureOperations
	ClusterAlert                            managementClient.ClusterAlertOperations
	ProjectAlert                            managementClient.ProjectAlertOperations
	Notifier                                managementClient.NotifierOperations
	ClusterAlertGroup                       managementClient.ClusterAlertGroupOperations
	ProjectAlertGroup                       managementClient.ProjectAlertGroupOperations
	ClusterAlertRule                        managementClient.ClusterAlertRuleOperations
	ProjectAlertRule                        managementClient.ProjectAlertRuleOperations
	ComposeConfig                           managementClient.ComposeConfigOperations
	ProjectCatalog                          managementClient.ProjectCatalogOperations
	ClusterCatalog                          managementClient.ClusterCatalogOperations
	MultiClusterApp                         managementClient.MultiClusterAppOperations
	MultiClusterAppRevision                 managementClient.MultiClusterAppRevisionOperations
	GlobalDns                               managementClient.GlobalDnsOperations
	GlobalDnsProvider                       managementClient.GlobalDnsProviderOperations
	KontainerDriver                         managementClient.KontainerDriverOperations
	EtcdBackup                              managementClient.EtcdBackupOperations
	ClusterScan                             managementClient.ClusterScanOperations
	MonitorMetric                           managementClient.MonitorMetricOperations
	ClusterMonitorGraph                     managementClient.ClusterMonitorGraphOperations
	ProjectMonitorGraph                     managementClient.ProjectMonitorGraphOperations
	CloudCredential                         cloudcredentials.CloudCredentialOperations
	ManagementSecret                        managementClient.ManagementSecretOperations
	ClusterTemplate                         managementClient.ClusterTemplateOperations
	ClusterTemplateRevision                 managementClient.ClusterTemplateRevisionOperations
	RkeK8sSystemImage                       managementClient.RkeK8sSystemImageOperations
	RkeK8sServiceOption                     managementClient.RkeK8sServiceOptionOperations
	RkeAddon                                managementClient.RkeAddonOperations
	CisConfig                               managementClient.CisConfigOperations
	CisBenchmarkVersion                     managementClient.CisBenchmarkVersionOperations
	FleetWorkspace                          managementClient.FleetWorkspaceOperations
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	client, err := managementClient.NewClient(opts)
	if err != nil {
		return nil, err
	}

	cloudClient, err := cloudcredentials.NewClient(opts)
	if err != nil {
		return nil, err
	}

	managementClient := &Client{}

	managementClient.APIBaseClient = client.APIBaseClient
	managementClient.NodePool = client.NodePool
	managementClient.Node = client.Node
	managementClient.NodeDriver = client.NodeDriver
	managementClient.NodeTemplate = client.NodeTemplate
	managementClient.Project = client.Project
	managementClient.GlobalRole = client.GlobalRole
	managementClient.GlobalRoleBinding = client.GlobalRoleBinding
	managementClient.RoleTemplate = client.RoleTemplate
	managementClient.PodSecurityPolicyTemplate = client.PodSecurityPolicyTemplate
	managementClient.PodSecurityPolicyTemplateProjectBinding = client.PodSecurityPolicyTemplateProjectBinding
	managementClient.ClusterRoleTemplateBinding = client.ClusterRoleTemplateBinding
	managementClient.ProjectRoleTemplateBinding = client.ProjectRoleTemplateBinding
	managementClient.Cluster = client.Cluster
	managementClient.ClusterRegistrationToken = client.ClusterRegistrationToken
	managementClient.Catalog = client.Catalog
	managementClient.Template = client.Template
	managementClient.CatalogTemplate = client.CatalogTemplate
	managementClient.CatalogTemplateVersion = client.CatalogTemplateVersion
	managementClient.TemplateVersion = client.TemplateVersion
	managementClient.TemplateContent = client.TemplateContent
	managementClient.Group = client.Group
	managementClient.GroupMember = client.GroupMember
	managementClient.SamlToken = client.SamlToken
	managementClient.Principal = client.Principal
	managementClient.User = client.User
	managementClient.AuthConfig = client.AuthConfig
	managementClient.LdapConfig = client.LdapConfig
	managementClient.Token = client.Token
	managementClient.DynamicSchema = client.DynamicSchema
	managementClient.Preference = client.Preference
	managementClient.ProjectNetworkPolicy = client.ProjectNetworkPolicy
	managementClient.ClusterLogging = client.ClusterLogging
	managementClient.ProjectLogging = client.ProjectLogging
	managementClient.Setting = client.Setting
	managementClient.Feature = client.Feature
	managementClient.ClusterAlert = client.ClusterAlert
	managementClient.ProjectAlert = client.ProjectAlert
	managementClient.Notifier = client.Notifier
	managementClient.ClusterAlertGroup = client.ClusterAlertGroup
	managementClient.ProjectAlertGroup = client.ProjectAlertGroup
	managementClient.ClusterAlertRule = client.ClusterAlertRule
	managementClient.ProjectAlertRule = client.ProjectAlertRule
	managementClient.ComposeConfig = client.ComposeConfig
	managementClient.ProjectCatalog = client.ProjectCatalog
	managementClient.ClusterCatalog = client.ClusterCatalog
	managementClient.MultiClusterApp = client.MultiClusterApp
	managementClient.MultiClusterAppRevision = client.MultiClusterAppRevision
	managementClient.GlobalDns = client.GlobalDns
	managementClient.GlobalDnsProvider = client.GlobalDnsProvider
	managementClient.KontainerDriver = client.KontainerDriver
	managementClient.EtcdBackup = client.EtcdBackup
	managementClient.ClusterScan = client.ClusterScan
	managementClient.MonitorMetric = client.MonitorMetric
	managementClient.ClusterMonitorGraph = client.ClusterMonitorGraph
	managementClient.ProjectMonitorGraph = client.ProjectMonitorGraph
	managementClient.CloudCredential = cloudClient
	managementClient.ManagementSecret = client.ManagementSecret
	managementClient.ClusterTemplate = client.ClusterTemplate
	managementClient.ClusterTemplateRevision = client.ClusterTemplateRevision
	managementClient.RkeK8sSystemImage = client.RkeK8sSystemImage
	managementClient.RkeK8sServiceOption = client.RkeK8sServiceOption
	managementClient.RkeAddon = client.RkeAddon
	managementClient.CisConfig = client.CisConfig
	managementClient.CisBenchmarkVersion = client.CisBenchmarkVersion
	managementClient.FleetWorkspace = client.FleetWorkspace

	return managementClient, nil
}
