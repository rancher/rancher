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
	ClusterEvent                            ClusterEventOperations
	ClusterRegistrationToken                ClusterRegistrationTokenOperations
	Catalog                                 CatalogOperations
	Template                                TemplateOperations
	TemplateVersion                         TemplateVersionOperations
	TemplateContent                         TemplateContentOperations
	Group                                   GroupOperations
	GroupMember                             GroupMemberOperations
	Principal                               PrincipalOperations
	User                                    UserOperations
	AuthConfig                              AuthConfigOperations
	Token                                   TokenOperations
	DynamicSchema                           DynamicSchemaOperations
	Preference                              PreferenceOperations
	ProjectNetworkPolicy                    ProjectNetworkPolicyOperations
	ClusterLogging                          ClusterLoggingOperations
	ProjectLogging                          ProjectLoggingOperations
	ListenConfig                            ListenConfigOperations
	Setting                                 SettingOperations
	Notifier                                NotifierOperations
	ClusterAlert                            ClusterAlertOperations
	ProjectAlert                            ProjectAlertOperations
	ClusterPipeline                         ClusterPipelineOperations
	SourceCodeCredential                    SourceCodeCredentialOperations
	Pipeline                                PipelineOperations
	PipelineExecution                       PipelineExecutionOperations
	PipelineExecutionLog                    PipelineExecutionLogOperations
	SourceCodeRepository                    SourceCodeRepositoryOperations
	ComposeConfig                           ComposeConfigOperations
	CattleInstance                          CattleInstanceOperations
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
	client.ClusterEvent = newClusterEventClient(client)
	client.ClusterRegistrationToken = newClusterRegistrationTokenClient(client)
	client.Catalog = newCatalogClient(client)
	client.Template = newTemplateClient(client)
	client.TemplateVersion = newTemplateVersionClient(client)
	client.TemplateContent = newTemplateContentClient(client)
	client.Group = newGroupClient(client)
	client.GroupMember = newGroupMemberClient(client)
	client.Principal = newPrincipalClient(client)
	client.User = newUserClient(client)
	client.AuthConfig = newAuthConfigClient(client)
	client.Token = newTokenClient(client)
	client.DynamicSchema = newDynamicSchemaClient(client)
	client.Preference = newPreferenceClient(client)
	client.ProjectNetworkPolicy = newProjectNetworkPolicyClient(client)
	client.ClusterLogging = newClusterLoggingClient(client)
	client.ProjectLogging = newProjectLoggingClient(client)
	client.ListenConfig = newListenConfigClient(client)
	client.Setting = newSettingClient(client)
	client.Notifier = newNotifierClient(client)
	client.ClusterAlert = newClusterAlertClient(client)
	client.ProjectAlert = newProjectAlertClient(client)
	client.ClusterPipeline = newClusterPipelineClient(client)
	client.SourceCodeCredential = newSourceCodeCredentialClient(client)
	client.Pipeline = newPipelineClient(client)
	client.PipelineExecution = newPipelineExecutionClient(client)
	client.PipelineExecutionLog = newPipelineExecutionLogClient(client)
	client.SourceCodeRepository = newSourceCodeRepositoryClient(client)
	client.ComposeConfig = newComposeConfigClient(client)
	client.CattleInstance = newCattleInstanceClient(client)

	return client, nil
}
