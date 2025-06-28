package client

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	clientbase.APIBaseClient

	NodePool                                  NodePoolOperations
	Node                                      NodeOperations
	NodeDriver                                NodeDriverOperations
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
	CloudCredential                           CloudCredentialOperations
	ManagementSecret                          ManagementSecretOperations
	FleetWorkspace                            FleetWorkspaceOperations
	RancherUserNotification                   RancherUserNotificationOperations
	OIDCClient                                OIDCClientOperations
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
	client.CloudCredential = newCloudCredentialClient(client)
	client.ManagementSecret = newManagementSecretClient(client)
	client.FleetWorkspace = newFleetWorkspaceClient(client)
	client.RancherUserNotification = newRancherUserNotificationClient(client)
	client.OIDCClient = newOIDCClientClient(client)

	return client, nil
}
