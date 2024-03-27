package globalrolesv2

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubeapi/rbac"
	"github.com/rancher/shepherd/extensions/provisioning"
	"github.com/rancher/shepherd/extensions/provisioninginput"

	"github.com/rancher/shepherd/extensions/users"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	roleOwner          = "cluster-owner"
	roleMember         = "cluster-member"
	roleProjectOwner   = "project-owner"
	roleCrtbView       = "clusterroletemplatebindings-view"
	roleProjectsCreate = "projects-create"
	roleProjectsView   = "projects-view"
	standardUser       = "user"
	localcluster       = "local"
	crtbOwnerLabel     = "authz.management.cattle.io/grb-owner"
	namespace          = "fleet-default"
	localPrefix        = "local://"
	clusterContext     = "cluster"
	projectContext     = "project"
	bindingLabel       = "membership-binding-owner"
)

var globalRole = v3.GlobalRole{
	ObjectMeta: metav1.ObjectMeta{
		Name: "",
	},
	InheritedClusterRoles: []string{},
}

var globalRoleBinding = &v3.GlobalRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: "",
	},
	GlobalRoleName: "",
	UserName:       "",
}

func createGlobalRoleWithInheritedClusterRolesWrangler(client *rancher.Client, inheritedRoles []string) (*v3.GlobalRole, error) {
	globalRole.Name = namegen.AppendRandomString("testgr")
	globalRole.InheritedClusterRoles = inheritedRoles
	createdGlobalRole, err := client.WranglerContext.Mgmt.GlobalRole().Create(&globalRole)
	if err != nil {
		return nil, err
	}

	return createdGlobalRole, nil
}

func getGlobalRoleBindingForUserWrangler(client *rancher.Client, userID string) (string, error) {
	grblist, err := client.WranglerContext.Mgmt.GlobalRoleBinding().List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, grbs := range grblist.Items {
		if grbs.GlobalRoleName == globalRole.Name && grbs.UserName == userID {
			return grbs.Name, nil
		}
	}

	return "", nil
}

func listClusterRoleTemplateBindingsForInheritedClusterRoles(client *rancher.Client, grbOwner string, expectedCount int) (*v3.ClusterRoleTemplateBindingList, error) {
	req, err := labels.NewRequirement(crtbOwnerLabel, selection.In, []string{grbOwner})

	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(*req)

	var crtbs *v3.ClusterRoleTemplateBindingList

	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.OneMinuteTimeout, func() (done bool, pollErr error) {
		crtbs, pollErr = rbac.ListClusterRoleTemplateBindings(client, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if pollErr != nil {
			return false, pollErr
		}
		if len(crtbs.Items) == expectedCount {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}

	return crtbs, nil
}

func getCRBsForCRTBs(client *rancher.Client, crtbs *v3.ClusterRoleTemplateBindingList) (*rbacv1.ClusterRoleBindingList, error) {
	var downstreamCRBs rbacv1.ClusterRoleBindingList

	for _, crtb := range crtbs.Items {
		labelKey := fmt.Sprintf("%s_%s", crtb.ClusterName, crtb.Name)
		req, err := labels.NewRequirement(labelKey, selection.In, []string{bindingLabel})

		if err != nil {
			return nil, err
		}

		selector := labels.NewSelector().Add(*req)
		downstreamCRBsForCRTB, err := rbac.ListClusterRoleBindings(client, localcluster, metav1.ListOptions{
			LabelSelector: selector.String(),
		})

		if err != nil {
			return nil, err
		}

		downstreamCRBs.Items = append(downstreamCRBs.Items, downstreamCRBsForCRTB.Items...)
	}

	return &downstreamCRBs, nil
}

func getRBsForCRTBs(client *rancher.Client, crtbs *v3.ClusterRoleTemplateBindingList) (*rbacv1.RoleBindingList, error) {
	var downstreamRBs rbacv1.RoleBindingList

	for _, crtb := range crtbs.Items {
		roleTemplateName := crtb.RoleTemplateName

		if strings.Contains(roleTemplateName, "rt") {
			listOpt := metav1.ListOptions{
				FieldSelector: "metadata.name=" + roleTemplateName,
			}
			roleTemplateList, err := rbac.ListRoleTemplates(client, listOpt)
			if err != nil {
				return nil, err
			}
			roleTemplateName = roleTemplateList.Items[0].RoleTemplateNames[0]
		}

		nameSelector := fmt.Sprintf("metadata.name=%s-%s", crtb.Name, roleTemplateName)
		namespaceSelector := fmt.Sprintf("metadata.namespace=%s", crtb.ClusterName)
		combinedSelector := fmt.Sprintf("%s,%s", nameSelector, namespaceSelector)
		downstreamRBsForCRTB, err := rbac.ListRoleBindings(client, localcluster, "", metav1.ListOptions{
			FieldSelector: combinedSelector,
		})

		if err != nil {
			return nil, err
		}

		downstreamRBs.Items = append(downstreamRBs.Items, downstreamRBsForCRTB.Items...)
	}

	return &downstreamRBs, nil
}

func createDownstreamCluster(client *rancher.Client, clusterType string) (*management.Cluster, *v1.SteveAPIObject, *clusters.ClusterConfig, error) {
	provisioningConfig := new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, provisioningConfig)
	nodeProviders := provisioningConfig.NodeProviders[0]
	externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviders)
	testClusterConfig := clusters.ConvertConfigToClusterConfig(provisioningConfig)
	testClusterConfig.CNI = provisioningConfig.CNIs[0]

	var clusterObject *management.Cluster
	var steveObject *v1.SteveAPIObject
	var err error

	switch clusterType {
	case "RKE1":
		nodeAndRoles := []provisioninginput.NodePools{
			provisioninginput.AllRolesNodePool,
		}
		testClusterConfig.NodePools = nodeAndRoles
		testClusterConfig.KubernetesVersion = provisioningConfig.RKE1KubernetesVersions[0]
		clusterObject, _, err = provisioning.CreateProvisioningRKE1CustomCluster(client, &externalNodeProvider, testClusterConfig)
	case "RKE2":
		nodeAndRoles := []provisioninginput.MachinePools{
			provisioninginput.AllRolesMachinePool,
		}
		testClusterConfig.MachinePools = nodeAndRoles
		testClusterConfig.KubernetesVersion = provisioningConfig.RKE2KubernetesVersions[0]
		steveObject, err = provisioning.CreateProvisioningCustomCluster(client, &externalNodeProvider, testClusterConfig)
	default:
		return nil, nil, nil, fmt.Errorf("unsupported cluster type: %s", clusterType)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	return clusterObject, steveObject, testClusterConfig, nil
}

func createGlobalRole(client *rancher.Client, inheritedClusterrole []string) (*v3.GlobalRole, error) {
	globalRole.Name = namegen.AppendRandomString("testgr")
	globalRole.InheritedClusterRoles = inheritedClusterrole
	createdGlobalRole, err := rbac.CreateGlobalRole(client, &globalRole)
	return createdGlobalRole, err
}

func createGlobalRoleAndUser(client *rancher.Client, inheritedClusterrole []string) (*management.User, error) {
	globalRole, err := createGlobalRole(client, inheritedClusterrole)
	if err != nil {
		return nil, err
	}

	createdUser, err := users.CreateUserWithRole(client, users.UserConfig(), standardUser, globalRole.Name)
	if err != nil {
		return nil, err
	}

	return createdUser, err
}

func crtbStatus(client *rancher.Client, crtbName string, selector labels.Selector) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.TwoMinuteTimeout)
	defer cancel()

	err := kwait.PollUntilContextCancel(ctx, defaults.FiveHundredMillisecondTimeout, false, func(ctx context.Context) (done bool, err error) {
		crtbs, err := rbac.ListClusterRoleTemplateBindings(client, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return false, err
		}

		for _, newcrtb := range crtbs.Items {
			if crtbName == newcrtb.Name {
				return false, nil
			}
		}
		return true, nil
	})

	return err
}
