package integration

import (
	"errors"
	"net/http"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/pkg/clientbase"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
)

func (p *RTBTestSuite) TestUserVsUserBaseGlobalRoleVisibility() {
	client := p.newSubSession()

	// Create user1 with the standard "user" global role.
	user1 := p.createUser(client, "testuser1", "user")

	// Create user2 with the "user-base" global role.
	user2 := p.createUser(client, "testuser2", "user-base")

	// Create 2 more users (just to pad the user count).
	p.createUser(client, "testuser", "user")
	p.createUser(client, "testuser", "user")

	// Admin should see at least 5 users.
	adminUsers, err := client.Management.User.List(nil)
	p.Require().NoError(err)
	p.Require().GreaterOrEqual(len(adminUsers.Data), 5)

	user1Client, err := client.AsUser(user1)
	p.Require().NoError(err)

	user2Client, err := client.AsUser(user2)
	p.Require().NoError(err)

	// user1 (standard "user" role) should only see themselves.
	user1Users, err := user1Client.Management.User.List(nil)
	p.Require().NoError(err)
	p.Require().Len(user1Users.Data, 1, "user should only see themselves")

	// user1 can see all roleTemplates.
	user1RTs, err := user1Client.Management.RoleTemplate.List(nil)
	p.Require().NoError(err)
	p.Require().NotEmpty(user1RTs.Data, "user should be able to see all roleTemplates")

	// user2 (user-base role) should only see themselves.
	user2Users, err := user2Client.Management.User.List(nil)
	p.Require().NoError(err)
	p.Require().Len(user2Users.Data, 1, "user should only see themselves")

	// user2 should not see any role templates.
	user2RTs, err := user2Client.Management.RoleTemplate.List(nil)
	p.Require().NoError(err)
	p.Require().Empty(user2RTs.Data, "user2 does not have permission to view roleTemplates")
}

func (p *RTBTestSuite) TestKontainerDriverVisibilityByGlobalRole() {
	client := p.newSubSession()

	createUserWithRole := func(role string) *rancher.Client {
		u := p.createUser(client, "kd-user", role)
		c, err := client.AsUser(u)
		p.Require().NoError(err)
		return c
	}

	// Standard "user" role can see kontainer drivers.
	kds, err := createUserWithRole("user").Management.KontainerDriver.List(nil)
	p.Require().NoError(err)
	p.Require().Len(kds.Data, 3)

	// "clusters-create" role can see kontainer drivers.
	kds, err = createUserWithRole("clusters-create").Management.KontainerDriver.List(nil)
	p.Require().NoError(err)
	p.Require().Len(kds.Data, 3)

	// "kontainerdrivers-manage" role can see kontainer drivers.
	kds, err = createUserWithRole("kontainerdrivers-manage").Management.KontainerDriver.List(nil)
	p.Require().NoError(err)
	p.Require().Len(kds.Data, 3)

	// "settings-manage" role cannot see kontainer drivers.
	kds, err = createUserWithRole("settings-manage").Management.KontainerDriver.List(nil)
	p.Require().NoError(err)
	p.Require().Empty(kds.Data)
}

// TestBuiltinGlobalRoleOnlyNewUserDefaultEditable tests that admins can only edit
// a builtin global role's newUserDefault field.
func (p *RTBTestSuite) TestBuiltinGlobalRoleOnlyNewUserDefaultEditable() {
	client := p.newSubSession()

	gr, err := client.Management.GlobalRole.ByID("admin")
	p.Require().NoError(err)
	p.Require().True(gr.Builtin)
	_, hasRemove := gr.Links["remove"]
	p.Require().False(hasRemove, "builtin global role should not have a remove link")
	p.Require().False(gr.NewUserDefault)

	// Attempt to update multiple fields; only newUserDefault should change.
	updated, err := client.Management.GlobalRole.Update(gr, map[string]any{
		"name":           "gr-test",
		"description":    "asdf",
		"rules":          nil,
		"newUserDefault": true,
		"builtin":        true,
	})
	p.Require().NoError(err)

	// Revert newUserDefault after test.
	p.T().Cleanup(func() {
		_, _ = client.Management.GlobalRole.Update(updated, map[string]any{
			"newUserDefault": false,
		})
	})

	// Name should remain unchanged.
	p.Require().Equal(gr.Name, updated.Name)
	// Rules should not have been wiped out.
	p.Require().NotEmpty(updated.Rules)
	// Builtin should still be true.
	p.Require().True(updated.Builtin)
	// newUserDefault is the only field that should have changed.
	p.Require().True(updated.NewUserDefault)
}

// TestOnlyAdminCanCRUDGlobalRoles tests that only admins can create, get, update,
// and delete non-builtin global roles.
func (p *RTBTestSuite) TestOnlyAdminCanCRUDGlobalRoles() {
	client := p.newSubSession()

	user := p.createUser(client, "gr-user", "user")
	userClient, err := client.AsUser(user)
	p.Require().NoError(err)

	// Admin can create a global role.
	gr, err := client.Management.GlobalRole.Create(&management.GlobalRole{
		Name: namegen.AppendRandomString("gr-"),
	})
	p.Require().NoError(err)

	// Admin can update a global role.
	_, err = client.Management.GlobalRole.Update(gr, map[string]any{
		"annotations": map[string]string{"test": "asdf"},
	})
	p.Require().NoError(err)

	// Admin can list global roles.
	grList, err := client.Management.GlobalRole.List(nil)
	p.Require().NoError(err)
	p.Require().NotEmpty(grList.Data)

	// Admin can delete a global role.
	err = client.Management.GlobalRole.Delete(gr)
	p.Require().NoError(err)

	// Standard user cannot create a global role (403).
	_, err = userClient.Management.GlobalRole.Create(&management.GlobalRole{
		Name: namegen.AppendRandomString("gr-"),
	})
	var apiErr *clientbase.APIError
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusForbidden, apiErr.StatusCode)

	// Admin creates another global role for subsequent user tests.
	gr2, err := client.Management.GlobalRole.Create(&management.GlobalRole{
		Name: namegen.AppendRandomString("gr-"),
	})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.GlobalRole.Delete(gr2)
	})

	// Standard user cannot update the global role (403).
	_, err = userClient.Management.GlobalRole.Update(gr2, map[string]any{
		"annotations": map[string]string{"test2": "jkl"},
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusForbidden, apiErr.StatusCode)

	// Standard user sees no global roles when listing.
	userGRList, err := userClient.Management.GlobalRole.List(nil)
	p.Require().NoError(err)
	p.Require().Empty(userGRList.Data)

	// Standard user cannot delete the global role (403).
	err = userClient.Management.GlobalRole.Delete(gr2)
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusForbidden, apiErr.StatusCode)
}

// TestAdminCannotDeleteBuiltinGlobalRole tests that admins can edit builtin global
// roles but cannot delete them.
func (p *RTBTestSuite) TestAdminCannotDeleteBuiltinGlobalRole() {
	client := p.newSubSession()

	gr, err := client.Management.GlobalRole.ByID("admin")
	p.Require().NoError(err)
	p.Require().True(gr.Builtin)
	_, hasRemove := gr.Links["remove"]
	p.Require().False(hasRemove, "builtin global role should not have a remove link")

	// Admin creates a global role with builtin=true; it should be ignored.
	gr2, err := client.Management.GlobalRole.Create(&management.GlobalRole{
		Name:    namegen.AppendRandomString("gr-"),
		Builtin: true,
	})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.GlobalRole.Delete(gr2)
	})
	// Builtin cannot be set by admin on creation.
	p.Require().False(gr2.Builtin)

	// Admin can update the builtin role (no error).
	_, err = client.Management.GlobalRole.Update(gr, map[string]any{})
	p.Require().NoError(err)

	// Admin cannot delete the builtin role (403).
	err = client.Management.GlobalRole.Delete(gr)
	var apiErr *clientbase.APIError
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusForbidden, apiErr.StatusCode)
	p.Require().Contains(apiErr.Body, "cannot delete builtin global roles")
}
