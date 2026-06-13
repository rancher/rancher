package integration

import (
	"context"
	"time"

	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/api/scheme"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	backupsManageRole = "backups-manage"
)

// TestBackupsManageRole asserts that binding a user to the "backups-manage"
// ClusterRoleTemplate on the local cluster results in a Kubernetes Role in the
// "local" namespace that grants access to "etcdbackups" resources.
func (p *RTBTestSuite) TestBackupsManageRole() {
	client := p.newSubSession()

	enabled := true
	pw := password.GenerateUserPassword("testpass-")
	restrictedUser, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("restricted-"),
		Password: pw,
		Name:     "restricted",
		Enabled:  &enabled,
	}, "user-base")
	p.Require().NoError(err)

	crtb, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		ClusterID:      p.downstreamClusterID,
		RoleTemplateID: backupsManageRole,
		UserID:         restrictedUser.ID,
	})
	p.Require().NoError(err)

	// Wait for the CRTB to have UserPrincipalID populated.
	p.Require().Eventually(func() bool {
		c, err := client.Management.ClusterRoleTemplateBinding.ByID(crtb.ID)
		if err != nil {
			return false
		}
		return c.UserPrincipalID != ""
	}, 2*time.Minute, 2*time.Second, "timed out waiting for CRTB UserPrincipalID")

	// Get the Kubernetes Role "backups-manage" in the "local" namespace and
	// verify it grants access to "etcdbackups" resources.
	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	var role *rbacv1.Role
	p.Require().Eventually(func() bool {
		unstructuredRole, err := dynamicClient.Resource(extrbac.RoleGroupVersionResource).
			Namespace(p.downstreamClusterID).
			Get(context.Background(), backupsManageRole, metav1.GetOptions{})
		if err != nil {
			return false
		}
		role = &rbacv1.Role{}
		return scheme.Scheme.Convert(unstructuredRole, role, unstructuredRole.GroupVersionKind()) == nil
	}, 2*time.Minute, 2*time.Second, "timed out waiting for backups-manage Role to exist in local namespace")

	p.Require().NotNil(role)
	p.Require().NotEmpty(role.Rules, "expected backups-manage role to have at least one rule")

	found := false
	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			if resource == "etcdbackups" {
				found = true
				break
			}
		}
	}
	p.True(found, "expected 'etcdbackups' in backups-manage role resources")
}

// TestStandardUsersCannotAccessBackups asserts that the built-in "user" global
// role does not grant access to "etcdbackups" resources in any of its rules.
func (p *RTBTestSuite) TestStandardUsersCannotAccessBackups() {
	client := p.newSubSession()

	userRole, err := client.Management.GlobalRole.ByID("user")
	p.Require().NoError(err)

	for _, rule := range userRole.Rules {
		for _, resource := range rule.Resources {
			p.NotEqual("etcdbackups", resource,
				"standard 'user' global role should not grant access to etcdbackups")
		}
	}
}
