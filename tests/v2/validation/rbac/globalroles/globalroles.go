package globalroles

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/rbac"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	webhookErrorMessagePrefix = `admission webhook "rancher.cattle.io.globalroles.management.cattle.io" denied the request: globalrole: Forbidden:`
)

var (
	customGlobalRole = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{"*"},
				Resources: []string{"*"},
			},
		},
	}

	globalRoleBinding = &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		GlobalRoleName: "",
		UserName:       "",
	}
)

func createCustomGlobalRole(client *rancher.Client) (*v3.GlobalRole, error) {
	customGlobalRole.Name = namegen.AppendRandomString("testgr")
	createdGlobalRole, err := client.WranglerContext.Mgmt.GlobalRole().Create(&customGlobalRole)
	if err != nil {
		return nil, err
	}

	createdGlobalRole, err = rbac.GetGlobalRoleByName(client, createdGlobalRole.Name)
	if err != nil {
		return nil, err
	}

	return createdGlobalRole, err
}

func createUserWithBuiltinRole(client *rancher.Client, builtinGlobalRole rbac.Role) (*management.User, error) {
	createdUser, err := users.CreateUserWithRole(client, users.UserConfig(), builtinGlobalRole.String())
	if err != nil {
		return nil, err
	}

	return createdUser, err
}

func createCustomGlobalRoleAndUser(client *rancher.Client) (*v3.GlobalRole, *management.User, error) {
	createdGlobalRole, err := createCustomGlobalRole(client)

	createdUser, err := users.CreateUserWithRole(client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	if err != nil {
		return nil, nil, err
	}

	return createdGlobalRole, createdUser, err
}
