package client

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	clientbase.APIBaseClient

	Node                       NodeOperations
	Machine                    MachineOperations
	MachineDriver              MachineDriverOperations
	MachineTemplate            MachineTemplateOperations
	Project                    ProjectOperations
	RoleTemplate               RoleTemplateOperations
	PodSecurityPolicyTemplate  PodSecurityPolicyTemplateOperations
	ClusterRoleTemplateBinding ClusterRoleTemplateBindingOperations
	ProjectRoleTemplateBinding ProjectRoleTemplateBindingOperations
	Cluster                    ClusterOperations
	ClusterEvent               ClusterEventOperations
	ClusterRegistrationToken   ClusterRegistrationTokenOperations
	Catalog                    CatalogOperations
	Template                   TemplateOperations
	TemplateVersion            TemplateVersionOperations
	Token                      TokenOperations
	User                       UserOperations
	Group                      GroupOperations
	GroupMember                GroupMemberOperations
	Principal                  PrincipalOperations
	DynamicSchema              DynamicSchemaOperations
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		APIBaseClient: baseClient,
	}

	client.Node = newNodeClient(client)
	client.Machine = newMachineClient(client)
	client.MachineDriver = newMachineDriverClient(client)
	client.MachineTemplate = newMachineTemplateClient(client)
	client.Project = newProjectClient(client)
	client.RoleTemplate = newRoleTemplateClient(client)
	client.PodSecurityPolicyTemplate = newPodSecurityPolicyTemplateClient(client)
	client.ClusterRoleTemplateBinding = newClusterRoleTemplateBindingClient(client)
	client.ProjectRoleTemplateBinding = newProjectRoleTemplateBindingClient(client)
	client.Cluster = newClusterClient(client)
	client.ClusterEvent = newClusterEventClient(client)
	client.ClusterRegistrationToken = newClusterRegistrationTokenClient(client)
	client.Catalog = newCatalogClient(client)
	client.Template = newTemplateClient(client)
	client.TemplateVersion = newTemplateVersionClient(client)
	client.Token = newTokenClient(client)
	client.User = newUserClient(client)
	client.Group = newGroupClient(client)
	client.GroupMember = newGroupMemberClient(client)
	client.Principal = newPrincipalClient(client)
	client.DynamicSchema = newDynamicSchemaClient(client)

	return client, nil
}
