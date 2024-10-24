package client

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	clientbase.APIBaseClient

	NodePool                                  NodePoolOperations
	Node                                      NodeOperations
	NodeDriver                                NodeDriverOperations
	NodeTemplate                              NodeTemplateOperations
	PodSecurityAdmissionConfigurationTemplate PodSecurityAdmissionConfigurationTemplateOperations
	Project                                   ProjectOperations
	GlobalRole                                GlobalRoleOperations
	GlobalRoleBinding                         GlobalRoleBindingOperations
	RoleTemplate                              RoleTemplateOperations
	ClusterRoleTemplateBinding                ClusterRoleTemplateBindingOperations
	ProjectRoleTemplateBinding                ProjectRoleTemplateBindingOperations
	Cluster                                   ClusterOperations
	ClusterRegistrationToken                  ClusterRegistrationTokenOperations
	Group                                     GroupOperations
	GroupMember                               GroupMemberOperations
	SamlToken                                 SamlTokenOperations
	Principal                                 PrincipalOperations
	User                                      UserOperations
	AuthConfig                                AuthConfigOperations
	LdapConfig                                LdapConfigOperations
	Token                                     TokenOperations
	DynamicSchema                             DynamicSchemaOperations
	Preference                                PreferenceOperations
	ProjectNetworkPolicy                      ProjectNetworkPolicyOperations
	Setting                                   SettingOperations
	Feature                                   FeatureOperations
	ComposeConfig                             ComposeConfigOperations
	KontainerDriver                           KontainerDriverOperations
	EtcdBackup                                EtcdBackupOperations
	CloudCredential                           CloudCredentialOperations
	ManagementSecret                          ManagementSecretOperations
	ClusterTemplate                           ClusterTemplateOperations
	ClusterTemplateRevision                   ClusterTemplateRevisionOperations
	RkeK8sSystemImage                         RkeK8sSystemImageOperations
	RkeK8sServiceOption                       RkeK8sServiceOptionOperations
	RkeAddon                                  RkeAddonOperations
	FleetWorkspace                            FleetWorkspaceOperations
	RancherUserNotification                   RancherUserNotificationOperations
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
	client.PodSecurityAdmissionConfigurationTemplate = newPodSecurityAdmissionConfigurationTemplateClient(client)
	client.Project = newProjectClient(client)
	client.GlobalRole = newGlobalRoleClient(client)
	client.GlobalRoleBinding = newGlobalRoleBindingClient(client)
	client.RoleTemplate = newRoleTemplateClient(client)
	client.ClusterRoleTemplateBinding = newClusterRoleTemplateBindingClient(client)
	client.ProjectRoleTemplateBinding = newProjectRoleTemplateBindingClient(client)
	client.Cluster = newClusterClient(client)
	client.ClusterRegistrationToken = newClusterRegistrationTokenClient(client)
	client.Group = newGroupClient(client)
	client.GroupMember = newGroupMemberClient(client)
	client.SamlToken = newSamlTokenClient(client)
	client.Principal = newPrincipalClient(client)
	client.User = newUserClient(client)
	client.AuthConfig = newAuthConfigClient(client)
	client.LdapConfig = newLdapConfigClient(client)
	client.Token = newTokenClient(client)
	client.DynamicSchema = newDynamicSchemaClient(client)
	client.Preference = newPreferenceClient(client)
	client.ProjectNetworkPolicy = newProjectNetworkPolicyClient(client)
	client.Setting = newSettingClient(client)
	client.Feature = newFeatureClient(client)
	client.ComposeConfig = newComposeConfigClient(client)
	client.KontainerDriver = newKontainerDriverClient(client)
	client.EtcdBackup = newEtcdBackupClient(client)
	client.CloudCredential = newCloudCredentialClient(client)
	client.ManagementSecret = newManagementSecretClient(client)
	client.ClusterTemplate = newClusterTemplateClient(client)
	client.ClusterTemplateRevision = newClusterTemplateRevisionClient(client)
	client.RkeK8sSystemImage = newRkeK8sSystemImageClient(client)
	client.RkeK8sServiceOption = newRkeK8sServiceOptionClient(client)
	client.RkeAddon = newRkeAddonClient(client)
	client.FleetWorkspace = newFleetWorkspaceClient(client)
	client.RancherUserNotification = newRancherUserNotificationClient(client)

	return client, nil
}
