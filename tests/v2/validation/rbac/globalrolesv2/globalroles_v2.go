package globalrolesv2

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	rbacapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"

	"github.com/rancher/shepherd/extensions/users"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	localcluster        = "local"
	ownerLabel          = "authz.management.cattle.io/grb-owner"
	namespace           = "fleet-default"
	localPrefix         = "local://"
	clusterContext      = "cluster"
	projectContext      = "project"
	bindingLabel        = "membership-binding-owner"
	globalDataNamespace = "cattle-global-data"
	defaultNamespace    = "default"
)

var (
	globalRole = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		InheritedClusterRoles: []string{},
	}

	globalRoleBinding = &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		GlobalRoleName: "",
		UserName:       "",
	}

	readSecretsPolicy = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"secrets"},
	}

	readCRTBsPolicy = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{"management.cattle.io"},
		Resources: []string{"clusterroletemplatebindings"},
	}

	readPods = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	}

	readAllResourcesPolicy = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{"*"},
		Resources: []string{"*"},
	}

	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: namegen.AppendRandomString("secret-"),
		},
		Data: map[string][]byte{
			"key": []byte(namegen.RandStringLower(5)),
		},
	}
)

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
	req, err := labels.NewRequirement(ownerLabel, selection.In, []string{grbOwner})

	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(*req)

	var crtbs *v3.ClusterRoleTemplateBindingList

	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.OneMinuteTimeout, func() (done bool, pollErr error) {
		crtbs, pollErr = rbacapi.ListClusterRoleTemplateBindings(client, metav1.ListOptions{
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
		downstreamCRBsForCRTB, err := rbacapi.ListClusterRoleBindings(client, localcluster, metav1.ListOptions{
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
			roleTemplateList, err := rbacapi.ListRoleTemplates(client, listOpt)
			if err != nil {
				return nil, err
			}
			roleTemplateName = roleTemplateList.Items[0].RoleTemplateNames[0]
		}

		nameSelector := fmt.Sprintf("metadata.name=%s-%s", crtb.Name, roleTemplateName)
		namespaceSelector := fmt.Sprintf("metadata.namespace=%s", crtb.ClusterName)
		combinedSelector := fmt.Sprintf("%s,%s", nameSelector, namespaceSelector)
		downstreamRBsForCRTB, err := rbacapi.ListRoleBindings(client, localcluster, "", metav1.ListOptions{
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

func createGlobalRoleAndUser(client *rancher.Client, inheritedClusterrole []string) (*management.User, error) {
	globalRole, err := createGlobalRoleWithInheritedClusterRolesWrangler(client, inheritedClusterrole)
	if err != nil {
		return nil, err
	}

	createdUser, err := users.CreateUserWithRole(client, users.UserConfig(), rbac.StandardUser.String(), globalRole.Name)
	if err != nil {
		return nil, err
	}

	return createdUser, err
}

func crtbStatus(client *rancher.Client, crtbName string, selector labels.Selector) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.TwoMinuteTimeout)
	defer cancel()

	err := kwait.PollUntilContextCancel(ctx, defaults.FiveHundredMillisecondTimeout, false, func(ctx context.Context) (done bool, err error) {
		crtbs, err := rbacapi.ListClusterRoleTemplateBindings(client, metav1.ListOptions{
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

func createGlobalRoleWithNamespacedRules(client *rancher.Client, namespacedRules map[string][]rbacv1.PolicyRule) (*v3.GlobalRole, error) {
	globalRole.Name = namegen.AppendRandomString("test-nsr")
	globalRole.NamespacedRules = namespacedRules
	createdGlobalRole, err := rbacapi.CreateGlobalRole(client, &globalRole)
	if err != nil {
		return nil, err
	}
	return createdGlobalRole, nil
}

func createProjectAndAddANamespace(client *rancher.Client, nsPrefix string) (string, error) {
	project := projects.NewProjectTemplate(localcluster)
	customProject, err := client.WranglerContext.Mgmt.Project().Create(project)
	if err != nil {
		return "", err
	}
	customNS1, err := namespaces.CreateNamespace(client, localcluster, customProject.Name, namegen.AppendRandomString(nsPrefix), "", nil, nil)
	return customNS1.Name, err
}
