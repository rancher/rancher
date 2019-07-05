package client

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	clientbase.APIBaseClient

	NodePool                                NodePoolOperations
	Node                                    NodeOperations
	NodeDriver                              NodeDriverOperations
	NodeTemplate                            NodeTemplateOperations
	Project                                 ProjectOperations
	GlobalRole                              GlobalRoleOperations
	GlobalRoleBinding                       GlobalRoleBindingOperations
	RoleTemplate                            RoleTemplateOperations
	PodSecurityPolicyTemplate               PodSecurityPolicyTemplateOperations
	PodSecurityPolicyTemplateProjectBinding PodSecurityPolicyTemplateProjectBindingOperations
	ClusterRoleTemplateBinding              ClusterRoleTemplateBindingOperations
	ProjectRoleTemplateBinding              ProjectRoleTemplateBindingOperations
	Cluster                                 ClusterOperations
	ClusterRegistrationToken                ClusterRegistrationTokenOperations
	Catalog                                 CatalogOperations
	Template                                TemplateOperations
	CatalogTemplate                         CatalogTemplateOperations
	CatalogTemplateVersion                  CatalogTemplateVersionOperations
	TemplateVersion                         TemplateVersionOperations
	TemplateContent                         TemplateContentOperations
	Group                                   GroupOperations
	GroupMember                             GroupMemberOperations
	Principal                               PrincipalOperations
	User                                    UserOperations
	AuthConfig                              AuthConfigOperations
	LdapConfig                              LdapConfigOperations
	Token                                   TokenOperations
	DynamicSchema                           DynamicSchemaOperations
	Preference                              PreferenceOperations
	ProjectNetworkPolicy                    ProjectNetworkPolicyOperations
	ClusterLogging                          ClusterLoggingOperations
	ProjectLogging                          ProjectLoggingOperations
	ListenConfig                            ListenConfigOperations
	Setting                                 SettingOperations
	Feature                                 FeatureOperations
	ClusterAlert                            ClusterAlertOperations
	ProjectAlert                            ProjectAlertOperations
	Notifier                                NotifierOperations
	ClusterAlertGroup                       ClusterAlertGroupOperations
	ProjectAlertGroup                       ProjectAlertGroupOperations
	ClusterAlertRule                        ClusterAlertRuleOperations
	ProjectAlertRule                        ProjectAlertRuleOperations
	ComposeConfig                           ComposeConfigOperations
	ProjectCatalog                          ProjectCatalogOperations
	ClusterCatalog                          ClusterCatalogOperations
	MultiClusterApp                         MultiClusterAppOperations
	MultiClusterAppRevision                 MultiClusterAppRevisionOperations
	GlobalDNS                               GlobalDNSOperations
	GlobalDNSProvider                       GlobalDNSProviderOperations
	KontainerDriver                         KontainerDriverOperations
	EtcdBackup                              EtcdBackupOperations
	ClusterScan                             ClusterScanOperations
	MonitorMetric                           MonitorMetricOperations
	ClusterMonitorGraph                     ClusterMonitorGraphOperations
	ProjectMonitorGraph                     ProjectMonitorGraphOperations
	CloudCredential                         CloudCredentialOperations
	ManagementSecret                        ManagementSecretOperations
	ClusterTemplate                         ClusterTemplateOperations
	ClusterTemplateRevision                 ClusterTemplateRevisionOperations
	RKEK8sSystemImage                       RKEK8sSystemImageOperations
	RKEK8sServiceOption                     RKEK8sServiceOptionOperations
	RKEAddon                                RKEAddonOperations
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		APIBaseClient: baseClient,
	}

	client.NodePool = newNodePoolClient(client)
	client.Node = newNodeClient(client)
	client.NodeDriver = newNodeDriverClient(client)
	client.NodeTemplate = newNodeTemplateClient(client)
	client.Project = newProjectClient(client)
	client.GlobalRole = newGlobalRoleClient(client)
	client.GlobalRoleBinding = newGlobalRoleBindingClient(client)
	client.RoleTemplate = newRoleTemplateClient(client)
	client.PodSecurityPolicyTemplate = newPodSecurityPolicyTemplateClient(client)
	client.PodSecurityPolicyTemplateProjectBinding = newPodSecurityPolicyTemplateProjectBindingClient(client)
	client.ClusterRoleTemplateBinding = newClusterRoleTemplateBindingClient(client)
	client.ProjectRoleTemplateBinding = newProjectRoleTemplateBindingClient(client)
	client.Cluster = newClusterClient(client)
	client.ClusterRegistrationToken = newClusterRegistrationTokenClient(client)
	client.Catalog = newCatalogClient(client)
	client.Template = newTemplateClient(client)
	client.CatalogTemplate = newCatalogTemplateClient(client)
	client.CatalogTemplateVersion = newCatalogTemplateVersionClient(client)
	client.TemplateVersion = newTemplateVersionClient(client)
	client.TemplateContent = newTemplateContentClient(client)
	client.Group = newGroupClient(client)
	client.GroupMember = newGroupMemberClient(client)
	client.Principal = newPrincipalClient(client)
	client.User = newUserClient(client)
	client.AuthConfig = newAuthConfigClient(client)
	client.LdapConfig = newLdapConfigClient(client)
	client.Token = newTokenClient(client)
	client.DynamicSchema = newDynamicSchemaClient(client)
	client.Preference = newPreferenceClient(client)
	client.ProjectNetworkPolicy = newProjectNetworkPolicyClient(client)
	client.ClusterLogging = newClusterLoggingClient(client)
	client.ProjectLogging = newProjectLoggingClient(client)
	client.ListenConfig = newListenConfigClient(client)
	client.Setting = newSettingClient(client)
	client.Feature = newFeatureClient(client)
	client.ClusterAlert = newClusterAlertClient(client)
	client.ProjectAlert = newProjectAlertClient(client)
	client.Notifier = newNotifierClient(client)
	client.ClusterAlertGroup = newClusterAlertGroupClient(client)
	client.ProjectAlertGroup = newProjectAlertGroupClient(client)
	client.ClusterAlertRule = newClusterAlertRuleClient(client)
	client.ProjectAlertRule = newProjectAlertRuleClient(client)
	client.ComposeConfig = newComposeConfigClient(client)
	client.ProjectCatalog = newProjectCatalogClient(client)
	client.ClusterCatalog = newClusterCatalogClient(client)
	client.MultiClusterApp = newMultiClusterAppClient(client)
	client.MultiClusterAppRevision = newMultiClusterAppRevisionClient(client)
	client.GlobalDNS = newGlobalDNSClient(client)
	client.GlobalDNSProvider = newGlobalDNSProviderClient(client)
	client.KontainerDriver = newKontainerDriverClient(client)
	client.EtcdBackup = newEtcdBackupClient(client)
	client.ClusterScan = newClusterScanClient(client)
	client.MonitorMetric = newMonitorMetricClient(client)
	client.ClusterMonitorGraph = newClusterMonitorGraphClient(client)
	client.ProjectMonitorGraph = newProjectMonitorGraphClient(client)
	client.CloudCredential = newCloudCredentialClient(client)
	client.ManagementSecret = newManagementSecretClient(client)
	client.ClusterTemplate = newClusterTemplateClient(client)
	client.ClusterTemplateRevision = newClusterTemplateRevisionClient(client)
	client.RKEK8sSystemImage = newRKEK8sSystemImageClient(client)
	client.RKEK8sServiceOption = newRKEK8sServiceOptionClient(client)
	client.RKEAddon = newRKEAddonClient(client)

	return client, nil
}
