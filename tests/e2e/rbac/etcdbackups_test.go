package integration

import (
	"time"

	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"
	authzv1 "k8s.io/api/authorization/v1"
)

const (
	backupsManageRole = "backups-manage"
)

// TestBackupsManageRole asserts that binding a user to the "backups-manage"
// ClusterRoleTemplate on the local cluster results in the user having access
// to "etcdbackups" resources.
func (p *RTBTestSuite) TestBackupsManageRole() {
	client := p.newSubSession()

	restrictedUser := p.createUser(client, "restricted", "user-base")

	_, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		ClusterID:      p.downstreamClusterID,
		RoleTemplateID: backupsManageRole,
		UserID:         restrictedUser.ID,
	})
	p.Require().NoError(err)

	restrictedClient, err := client.AsUser(restrictedUser)
	p.Require().NoError(err)

	// Wait until RBAC has propagated and the user can actually list etcdbackups in
	// the cluster namespace. Management-plane resources for the local cluster live
	// in the namespace that matches the cluster ID (i.e. "local").
	err = extauthz.WaitForAllowed(restrictedClient, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Namespace: p.downstreamClusterID,
			Verb:      "list",
			Group:     "management.cattle.io",
			Resource:  "etcdbackups",
		},
	})
	p.Require().NoError(err, "expected restricted user bound to backups-manage to be able to list etcdbackups")
}

// TestStandardUsersCannotAccessBackups asserts that a user with only the
// built-in "user" global role cannot access "etcdbackups" resources.
func (p *RTBTestSuite) TestStandardUsersCannotAccessBackups() {
	client := p.newSubSession()

	standardUser := p.createUser(client, "standard-user", "user")

	standardClient, err := client.AsUser(standardUser)
	p.Require().NoError(err)

	// Wait long enough for any RBAC to sync, then assert denial.
	p.Require().Eventually(func() bool {
		allowed, err := checkAccessAllowed(standardClient, p.downstreamClusterID, &authzv1.ResourceAttributes{
			Namespace: p.downstreamClusterID,
			Verb:      "list",
			Group:     "management.cattle.io",
			Resource:  "etcdbackups",
		})
		// Keep retrying only on transport errors; a clean false result is our target.
		return err == nil && !allowed
	}, 2*time.Minute, 2*time.Second, "standard 'user' global role should not grant access to etcdbackups")
}
