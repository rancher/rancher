package fleetworkspaces

import (
	"context"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	defaultFleetWorkspace       = "fleet-default"
	localCluster                = "local"
	userKind                    = "User"
	clusterNameAnnotationKey    = "cluster.cattle.io/name"
	ownerNamespaceAnnotationKey = "objectset.rio.cattle.io/owner-namespace"
)

func createFleetWorkspace(client *rancher.Client) (*management.FleetWorkspace, error) {
	newWorkspace := namegen.AppendRandomString("testworkspace-")
	workspacePayload := &management.FleetWorkspace{
		Annotations:     map[string]string{},
		Labels:          map[string]string{},
		Name:            newWorkspace,
		OwnerReferences: []management.OwnerReference{},
	}

	workspace, err := client.Management.FleetWorkspace.Create(workspacePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create FleetWorkspace: %w", err)
	}

	return workspace, nil
}

func getClusterFleetWorkspace(client *rancher.Client, clusterID string) (string, error) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return "", fmt.Errorf("Failed to get cluster by ID %s: %v", clusterID, err)
	}

	return cluster.FleetWorkspaceName, nil
}

func verifyClusterRoleTemplateBindingForUser(client *rancher.Client, username string, expectedCount int) error {
	crtbList, err := rbac.ListClusterRoleTemplateBindings(client, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ClusterRoleTemplateBindings: %w", err)
	}

	actualCount := 0
	for _, crtb := range crtbList.Items {
		if crtb.UserName == username {
			actualCount++
		}
	}

	if actualCount != expectedCount {
		return fmt.Errorf("expected %d ClusterRoleTemplateBindings for user %s, but found %d",
			expectedCount, username, actualCount)
	}

	return nil
}

func verifyProjectRoleTemplateBindingForUser(client *rancher.Client, username string, expectedCount int) error {
	prtbList, err := rbac.ListProjectRoleTemplateBindings(client, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ProjectRoleTemplateBindings: %w", err)
	}

	actualCount := 0
	for _, prtb := range prtbList.Items {
		if prtb.UserName == username {
			actualCount++
		}
	}

	if actualCount != expectedCount {
		return fmt.Errorf("expected %d ProjectRoleTemplateBindings for user %s, but found %d",
			expectedCount, username, actualCount)
	}

	return nil
}

func verifyRoleBindingsForUser(adminClient *rancher.Client, user *management.User, clusterID, fleetWorkspaceName string, expectedCount int) error {
	rblist, err := rbac.ListRoleBindings(adminClient, localCluster, fleetWorkspaceName, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list RoleBindings: %w", err)
	}

	userID := user.Resource.ID
	actualCount := 0

	for _, rb := range rblist.Items {
		for _, subject := range rb.Subjects {
			if subject.Kind == userKind && subject.Name == userID {
				if rb.Annotations[clusterNameAnnotationKey] == clusterID {
					actualCount++
					break
				}
			}
		}
	}

	if actualCount != expectedCount {
		return fmt.Errorf("expected %d role bindings for user %s in workspace %s, but found %d",
			expectedCount, userID, fleetWorkspaceName, actualCount)
	}

	return nil
}

func getAllRoleBindingsForCluster(client *rancher.Client, clusterID, fleetWorkspaceName string) ([]string, error) {
	rblist, err := rbac.ListRoleBindings(client, localCluster, fleetWorkspaceName, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list RoleBindings: %w", err)
	}

	roleBindings := []string{}

	for _, rb := range rblist.Items {
		if rb.Annotations[clusterNameAnnotationKey] == clusterID && rb.Annotations[ownerNamespaceAnnotationKey] == clusterID {
			roleBindings = append(roleBindings, rb.Name)
		}
	}

	return roleBindings, nil
}

func moveClusterToNewWorkspace(client *rancher.Client, cluster *management.Cluster, targetWorkspace string) (*management.Cluster, error) {
	updates := &management.Cluster{
		Name:               cluster.Name,
		FleetWorkspaceName: targetWorkspace,
	}

	_, err := client.Management.Cluster.Update(cluster, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update cluster with the new workspace: %w", err)
	}

	updatedCluster, err := waitForClusterUpdate(client, cluster.ID, targetWorkspace)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for cluster update: %w", err)
	}

	_, clusterObject, err := clusters.GetProvisioningClusterByName(client, updatedCluster.ID, targetWorkspace)
	if err != nil {
		return nil, fmt.Errorf("failed to get provisioning cluster by name: %w", err)
	}

	if clusterObject.Namespace != targetWorkspace {
		return nil, fmt.Errorf("the namespace in the cluster object does not match the new workspace: expected %s, got %s", targetWorkspace, clusterObject.Namespace)
	}

	return updatedCluster, nil
}

func waitForClusterUpdate(client *rancher.Client, clusterID string, expectedFleetWorkspaceName string) (*management.Cluster, error) {
	var updatedCluster *management.Cluster

	err := kwait.PollUntilContextTimeout(context.Background(), defaults.FiveSecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (done bool, pollErr error) {
		cluster, pollErr := client.Management.Cluster.ByID(clusterID)
		if pollErr != nil {
			return false, fmt.Errorf("failed to get cluster by ID: %w", pollErr)
		}

		if cluster.FleetWorkspaceName == expectedFleetWorkspaceName {
			updatedCluster = cluster
			return true, nil
		}
		return false, nil
	},
	)

	return updatedCluster, err
}

func createGlobalRoleWithFleetWorkspaceRules(client *rancher.Client) (*v3.GlobalRole, error) {
	globalRole := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namegen.AppendRandomString("test-fwr"),
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"fleet.cattle.io"},
				Resources: []string{"clusters"},
			},
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"management.cattle.io"},
				Resources: []string{"fleetworkspaces"},
			},
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"fleet.cattle.io"},
				Resources: []string{"gitrepos"},
			},
		},
	}

	createdGlobalRole, err := rbac.CreateGlobalRole(client, &globalRole)
	if err != nil {
		return nil, err
	}

	return createdGlobalRole, nil
}
