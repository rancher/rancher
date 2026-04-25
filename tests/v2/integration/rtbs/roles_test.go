package integration

import (
	"context"
	"time"

	"github.com/rancher/norman/types"
	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"
	extunstructured "github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/api/scheme"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	authzv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// checkAccessAllowed performs a single SelfSubjectAccessReview and returns whether access is allowed.
func checkAccessAllowed(client *rancher.Client, clusterID string, attr *authzv1.ResourceAttributes) (bool, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return false, err
	}

	ssar := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: attr,
		},
	}

	ssarGVR := authzv1.SchemeGroupVersion.WithResource("selfsubjectaccessreviews")
	resp, err := dynamicClient.Resource(ssarGVR).Create(context.TODO(), extunstructured.MustToUnstructured(ssar), metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	result := &authzv1.SelfSubjectAccessReview{}
	if err := scheme.Scheme.Convert(resp, result, resp.GroupVersionKind()); err != nil {
		return false, err
	}

	return result.Status.Allowed, nil
}

func (p *RTBTestSuite) TestUserVsUserBaseGlobalRoleVisibility() {
	client := p.newSubSession()

	// Create user1 with the standard "user" global role.
	user1 := p.createUser(client, "testuser1", "user")

	// Create user2 with the "user-base" global role.
	user2 := p.createUser(client, "testuser2", "user-base")

	// Create 2 more users (just to pad the user count).
	for i := 0; i < 2; i++ {
		p.createUser(client, "testuser", "user")
	}

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

func (p *RTBTestSuite) TestImpersonationByClusterRole() {
	client := p.newSubSession()

	// Create user1 with standard "user" role.
	user1 := p.createUser(client, "imp-user1", "user")

	// Create user2 with standard "user" role.
	user2 := p.createUser(client, "imp-user2", "user")

	localCluster, err := client.Management.Cluster.ByID(p.downstreamClusterID)
	p.Require().NoError(err)

	// Give user1 cluster-member and user2 cluster-owner.
	err = users.AddClusterRoleToUser(client, localCluster, user1, "cluster-member", nil)
	p.Require().NoError(err)

	err = users.AddClusterRoleToUser(client, localCluster, user2, "cluster-owner", nil)
	p.Require().NoError(err)

	user1Client, err := client.AsUser(user1)
	p.Require().NoError(err)

	user2Client, err := client.AsUser(user2)
	p.Require().NoError(err)

	impersonateAttr := &authzv1.ResourceAttributes{
		Verb:     "impersonate",
		Resource: "users",
		Group:    "",
	}

	// Admin can always impersonate.
	err = extauthz.WaitForAllowed(client, p.downstreamClusterID, []*authzv1.ResourceAttributes{impersonateAttr})
	p.Require().NoError(err)

	// User1 is a cluster-member which does not grant impersonate.
	allowed, err := checkAccessAllowed(user1Client, p.downstreamClusterID, impersonateAttr)
	p.Require().NoError(err)
	p.Require().False(allowed, "cluster-member should not be able to impersonate")

	// User2 is a cluster-owner which allows impersonation.
	err = extauthz.WaitForAllowed(user2Client, p.downstreamClusterID, []*authzv1.ResourceAttributes{impersonateAttr})
	p.Require().NoError(err)

	// Create a ClusterRole allowing limited impersonation of user2 only.
	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	impRoleName := namegen.AppendRandomString("limited-impersonator-")
	impRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: impRoleName},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"users"},
				Verbs:         []string{"impersonate"},
				ResourceNames: []string{user2.ID},
			},
		},
	}

	crResource := dynamicClient.Resource(extrbac.ClusterRoleGroupVersionResource)

	var cr unstructured.Unstructured
	err = scheme.Scheme.Convert(impRole, &cr, nil)
	p.Require().NoError(err)

	_, err = crResource.Create(context.TODO(), &cr, metav1.CreateOptions{})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		err := crResource.Delete(context.TODO(), impRoleName, metav1.DeleteOptions{})
		p.Require().NoError(err)
	})

	// Create a ClusterRoleBinding binding user1 to the impersonation role.
	impBindingName := namegen.AppendRandomString("limited-impersonator-binding-")
	impBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: impBindingName},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: user1.ID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     impRoleName,
		},
	}

	crbResource := dynamicClient.Resource(extrbac.ClusterRoleBindingGroupVersionResource)
	var crb unstructured.Unstructured
	err = scheme.Scheme.Convert(impBinding, &crb, nil)
	p.Require().NoError(err)

	_, err = crbResource.Create(context.TODO(), &crb, metav1.CreateOptions{})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		err := crbResource.Delete(context.TODO(), impBindingName, metav1.DeleteOptions{})
		p.Require().NoError(err)
	})

	// User1 should now be able to impersonate user2 specifically.
	err = extauthz.WaitForAllowed(user1Client, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "impersonate",
			Resource: "users",
			Group:    "",
			Name:     user2.ID,
		},
	})
	p.Require().NoError(err)
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

func (p *RTBTestSuite) TestCannotCreateFeature() {
	client := p.newSubSession()

	// Create a standard user.
	user := p.createUser(client, "feature-user", "user")
	userClient, err := client.AsUser(user)
	p.Require().NoError(err)

	trueVal := true

	// Admin should not be able to create features (405 Method Not Allowed).
	_, err = client.Management.Feature.Create(&management.Feature{
		Name:  "testfeature",
		Value: &trueVal,
	})
	p.Require().Error(err)
	p.Require().ErrorContains(err, "405")

	// Standard user should not be able to create features (405 Method Not Allowed).
	_, err = userClient.Management.Feature.Create(&management.Feature{
		Name:  "testfeature",
		Value: &trueVal,
	})
	p.Require().Error(err)
	p.Require().ErrorContains(err, "405")
}

func (p *RTBTestSuite) TestCanListFeatures() {
	client := p.newSubSession()

	// Create a standard user.
	user := p.createUser(client, "feature-user", "user")
	userClient, err := client.AsUser(user)
	p.Require().NoError(err)

	// Standard user should be able to list features.
	userFeatures, err := userClient.Management.Feature.List(nil)
	p.Require().NoError(err)
	p.Require().NotEmpty(userFeatures.Data)

	// Admin should be able to list features.
	adminFeatures, err := client.Management.Feature.List(nil)
	p.Require().NoError(err)
	p.Require().NotEmpty(adminFeatures.Data)
}

// ensureClusterRolesExist ensures the given ClusterRoles exist in the downstream cluster,
// creating them if necessary. Returns a cleanup function that deletes only the ones it created.
func (p *RTBTestSuite) ensureClusterRolesExist(client *rancher.Client, names []string) {
	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	crResource := dynamicClient.Resource(extrbac.ClusterRoleGroupVersionResource)

	var created []string
	for _, name := range names {
		_, err := crResource.Get(context.TODO(), name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			cr := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: name},
			}
			var u unstructured.Unstructured
			convErr := scheme.Scheme.Convert(cr, &u, nil)
			p.Require().NoError(convErr)
			_, err = crResource.Create(context.TODO(), &u, metav1.CreateOptions{})
			p.Require().NoError(err)
			created = append(created, name)
		} else {
			p.Require().NoError(err)
		}
	}

	p.T().Cleanup(func() {
		for _, name := range created {
			_ = crResource.Delete(context.TODO(), name, metav1.DeleteOptions{})
		}
	})
}

// setClusterCreatorDefaults sets the given roles as cluster creator defaults. When a cluster is created these roles will be bound to the creator.
// Clears all clusterCreatorDefault flags, then sets them on the given role IDs.
// Registers a cleanup to restore original defaults.
func (p *RTBTestSuite) setClusterCreatorDefaults(client *rancher.Client, roleIDs []string) {
	rollTemplates, err := client.Management.RoleTemplate.List(nil)
	p.Require().NoError(err)

	// Save original state for cleanup.
	originals := map[string]bool{}
	for _, rt := range rollTemplates.Data {
		if rt.ClusterCreatorDefault {
			originals[rt.ID] = true
		}
	}

	// Clear all cluster creator defaults.
	for i := range rollTemplates.Data {
		rt := &rollTemplates.Data[i]
		if rt.ClusterCreatorDefault {
			_, err := client.Management.RoleTemplate.Update(rt, map[string]any{
				"clusterCreatorDefault": false,
			})
			p.Require().NoError(err)
		}
	}

	// Set the desired roles as cluster creator defaults.
	wantSet := map[string]bool{}
	for _, id := range roleIDs {
		wantSet[id] = true
		rt, err := client.Management.RoleTemplate.ByID(id)
		p.Require().NoError(err)
		_, err = client.Management.RoleTemplate.Update(rt, map[string]any{
			"clusterCreatorDefault": true,
		})
		p.Require().NoError(err)
	}

	p.T().Cleanup(func() {
		// Reset: clear all, then restore originals.
		allRTs, err := client.Management.RoleTemplate.List(nil)
		if err != nil {
			return
		}
		for i := range allRTs.Data {
			rt := &allRTs.Data[i]
			if rt.ClusterCreatorDefault && !originals[rt.ID] {
				_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{
					"clusterCreatorDefault": false,
				})
			} else if !rt.ClusterCreatorDefault && originals[rt.ID] {
				_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{
					"clusterCreatorDefault": true,
				})
			}
			// Also unlock any roles we may have locked.
			if rt.Locked && wantSet[rt.ID] {
				_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{
					"locked": false,
				})
			}
		}
	})
}

// setProjectCreatorDefaults sets the given roles as project creator defaults. When a project is created these roles will be bound to the creator.
// Clears all projectCreatorDefault flags, then sets them on the given role IDs.
// Registers a cleanup to restore original defaults.
func (p *RTBTestSuite) setProjectCreatorDefaults(client *rancher.Client, roleIDs []string) {
	rollTemplates, err := client.Management.RoleTemplate.List(nil)
	p.Require().NoError(err)

	originals := map[string]bool{}
	for _, rt := range rollTemplates.Data {
		if rt.ProjectCreatorDefault {
			originals[rt.ID] = true
		}
	}

	for i := range rollTemplates.Data {
		rt := &rollTemplates.Data[i]
		if rt.ProjectCreatorDefault {
			_, err := client.Management.RoleTemplate.Update(rt, map[string]any{
				"projectCreatorDefault": false,
			})
			p.Require().NoError(err)
		}
	}

	wantSet := map[string]bool{}
	for _, id := range roleIDs {
		wantSet[id] = true
		rt, err := client.Management.RoleTemplate.ByID(id)
		p.Require().NoError(err)
		_, err = client.Management.RoleTemplate.Update(rt, map[string]any{
			"projectCreatorDefault": true,
		})
		p.Require().NoError(err)
	}

	p.T().Cleanup(func() {
		allRTs, err := client.Management.RoleTemplate.List(nil)
		if err != nil {
			return
		}
		for i := range allRTs.Data {
			rt := &allRTs.Data[i]
			if rt.ProjectCreatorDefault && !originals[rt.ID] {
				_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{
					"projectCreatorDefault": false,
				})
			} else if !rt.ProjectCreatorDefault && originals[rt.ID] {
				_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{
					"projectCreatorDefault": true,
				})
			}
			if rt.Locked && wantSet[rt.ID] {
				_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{
					"locked": false,
				})
			}
		}
	})
}

// setGlobalRoleDefaults sets the given roles as new user defaults. When a new user is created these roles will be assigned to them.
// Clears all newUserDefault flags, then sets them on the given role IDs.
// Registers a cleanup to restore original defaults.
func (p *RTBTestSuite) setGlobalRoleDefaults(client *rancher.Client, roleIDs []string) {
	globalRoles, err := client.Management.GlobalRole.List(nil)
	p.Require().NoError(err)

	originals := map[string]bool{}
	for _, gr := range globalRoles.Data {
		if gr.NewUserDefault {
			originals[gr.ID] = true
		}
	}

	for i := range globalRoles.Data {
		gr := &globalRoles.Data[i]
		if gr.NewUserDefault {
			_, err := client.Management.GlobalRole.Update(gr, map[string]any{
				"newUserDefault": false,
			})
			p.Require().NoError(err)
		}
	}

	for _, id := range roleIDs {
		gr, err := client.Management.GlobalRole.ByID(id)
		p.Require().NoError(err)
		_, err = client.Management.GlobalRole.Update(gr, map[string]any{
			"newUserDefault": true,
		})
		p.Require().NoError(err)
	}

	p.T().Cleanup(func() {
		allGRs, err := client.Management.GlobalRole.List(nil)
		if err != nil {
			return
		}
		for i := range allGRs.Data {
			gr := &allGRs.Data[i]
			if gr.NewUserDefault && !originals[gr.ID] {
				_, _ = client.Management.GlobalRole.Update(gr, map[string]any{
					"newUserDefault": false,
				})
			} else if !gr.NewUserDefault && originals[gr.ID] {
				_, _ = client.Management.GlobalRole.Update(gr, map[string]any{
					"newUserDefault": true,
				})
			}
		}
	})
}

// TestClusterCreateDefaultRole tests that any roles that are set as cluster creator defaults get assigned to the creator when a cluster is created.
func (p *RTBTestSuite) TestClusterCreateDefaultRole() {
	client := p.newSubSession()

	p.ensureClusterRolesExist(client, []string{"monitoring-ui-view", "navlinks-view", "navlinks-manage"})

	// Set these 3 roles as cluster creator defaults.
	testRoles := []string{"projects-create", "storage-manage", "nodes-view"}
	p.setClusterCreatorDefaults(client, testRoles)

	cluster, err := client.Management.Cluster.Create(&management.Cluster{
		Name: namegen.AppendRandomString("test-cluster-"),
	})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.Cluster.Delete(cluster)
	})

	// Wait for InitialRolesPopulated condition.
	p.Require().Eventually(func() bool {
		c, err := client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return false
		}
		for _, cond := range c.Conditions {
			if cond.Type == "InitialRolesPopulated" && cond.Status == "True" {
				cluster = c
				return true
			}
		}
		return false
	}, 2*time.Minute, 2*time.Second, "waiting for InitialRolesPopulated on cluster")

	crtbs, err := client.Management.ClusterRoleTemplateBinding.List(&types.ListOpts{
		Filters: map[string]any{"clusterId": cluster.ID},
	})
	p.Require().NoError(err)
	p.Require().Len(crtbs.Data, 3)

	for _, binding := range crtbs.Data {
		var b *management.ClusterRoleTemplateBinding
		p.Require().Eventually(func() bool {
			b, err = client.Management.ClusterRoleTemplateBinding.ByID(binding.ID)
			return err == nil && b.UserPrincipalID != ""
		}, 2*time.Minute, 2*time.Second, "waiting for userPrincipalId on CRTB")

		p.Require().Contains(testRoles, b.RoleTemplateID)
		p.Require().NotEmpty(b.UserID)

		user, err := client.Management.User.ByID(b.UserID)
		p.Require().NoError(err)
		p.Require().Contains(user.PrincipalIDs, b.UserPrincipalID)
	}
}

// TestClusterCreateRoleLocked tests that if a cluster creator default role is locked, it is not bound to the creator and does not prevent other defaults from being bound.
func (p *RTBTestSuite) TestClusterCreateRoleLocked() {
	client := p.newSubSession()

	p.ensureClusterRolesExist(client, []string{"monitoring-ui-view", "navlinks-view", "navlinks-manage"})

	testRoles := []string{"projects-create", "storage-manage", "nodes-view"}
	p.setClusterCreatorDefaults(client, testRoles)

	// Lock the last role.
	lockedRole := testRoles[len(testRoles)-1]
	activeRoles := testRoles[:len(testRoles)-1]

	rt, err := client.Management.RoleTemplate.ByID(lockedRole)
	p.Require().NoError(err)
	_, err = client.Management.RoleTemplate.Update(rt, map[string]any{"locked": true})
	p.Require().NoError(err)

	cluster, err := client.Management.Cluster.Create(&management.Cluster{
		Name: namegen.AppendRandomString("test-cluster-"),
	})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.Cluster.Delete(cluster)
	})

	p.Require().Eventually(func() bool {
		c, err := client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return false
		}
		for _, cond := range c.Conditions {
			if cond.Type == "InitialRolesPopulated" && cond.Status == "True" {
				cluster = c
				return true
			}
		}
		return false
	}, 2*time.Minute, 2*time.Second, "waiting for InitialRolesPopulated on cluster")

	crtbs, err := client.Management.ClusterRoleTemplateBinding.List(&types.ListOpts{
		Filters: map[string]any{"clusterId": cluster.ID},
	})
	p.Require().NoError(err)
	p.Require().Len(crtbs.Data, 2)

	for _, binding := range crtbs.Data {
		p.Require().Contains(activeRoles, binding.RoleTemplateID)
	}
}

// TestProjectCreateDefaultRole tests that any roles that are set as project creator defaults get assigned to the creator when a project is created.
func (p *RTBTestSuite) TestProjectCreateDefaultRole() {
	client := p.newSubSession()

	p.ensureClusterRolesExist(client, []string{"monitoring-ui-view", "navlinks-view", "navlinks-manage"})

	testRoles := []string{"project-member", "workloads-view", "secrets-view"}
	p.setProjectCreatorDefaults(client, testRoles)

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-project-"),
		ClusterID: "local",
	})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.Project.Delete(project)
	})

	p.Require().Eventually(func() bool {
		prj, err := client.Management.Project.ByID(project.ID)
		if err != nil {
			return false
		}
		for _, cond := range prj.Conditions {
			if cond.Type == "InitialRolesPopulated" && cond.Status == "True" {
				project = prj
				return true
			}
		}
		return false
	}, 2*time.Minute, 2*time.Second, "waiting for InitialRolesPopulated on project")

	prtbs, err := client.Management.ProjectRoleTemplateBinding.List(&types.ListOpts{
		Filters: map[string]any{"projectId": project.ID},
	})
	p.Require().NoError(err)
	p.Require().Len(prtbs.Data, 3)

	for _, binding := range prtbs.Data {
		var b *management.ProjectRoleTemplateBinding
		p.Require().Eventually(func() bool {
			b, err = client.Management.ProjectRoleTemplateBinding.ByID(binding.ID)
			return err == nil && b.UserPrincipalID != ""
		}, 2*time.Minute, 2*time.Second, "waiting for userPrincipalId on PRTB")

		p.Require().Contains(testRoles, b.RoleTemplateID)
		p.Require().NotEmpty(b.UserID)

		user, err := client.Management.User.ByID(b.UserID)
		p.Require().NoError(err)
		p.Require().Contains(user.PrincipalIDs, b.UserPrincipalID)
	}
}

// TestProjectCreateRoleLocked tests that if a project creator default role is locked, it is not bound to the creator and does not prevent other defaults from being bound.
func (p *RTBTestSuite) TestProjectCreateRoleLocked() {
	client := p.newSubSession()

	p.ensureClusterRolesExist(client, []string{"monitoring-ui-view", "navlinks-view", "navlinks-manage"})

	testRoles := []string{"project-member", "workloads-view", "secrets-view"}
	p.setProjectCreatorDefaults(client, testRoles)

	// Lock the last role.
	lockedRole := testRoles[len(testRoles)-1]
	activeRoles := testRoles[:len(testRoles)-1]

	rt, err := client.Management.RoleTemplate.ByID(lockedRole)
	p.Require().NoError(err)
	_, err = client.Management.RoleTemplate.Update(rt, map[string]any{"locked": true})
	p.Require().NoError(err)

	// Wait for the lock to take effect.
	p.Require().Eventually(func() bool {
		updated, err := client.Management.RoleTemplate.ByID(lockedRole)
		return err == nil && updated.Locked
	}, 2*time.Minute, 2*time.Second, "waiting for role to be locked")

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-project-"),
		ClusterID: "local",
	})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.Project.Delete(project)
	})

	p.Require().Eventually(func() bool {
		prj, err := client.Management.Project.ByID(project.ID)
		if err != nil {
			return false
		}
		for _, cond := range prj.Conditions {
			if cond.Type == "InitialRolesPopulated" && cond.Status == "True" {
				project = prj
				return true
			}
		}
		return false
	}, 2*time.Minute, 2*time.Second, "waiting for InitialRolesPopulated on project")

	prtbs, err := client.Management.ProjectRoleTemplateBinding.List(&types.ListOpts{
		Filters: map[string]any{"projectId": project.ID},
	})
	p.Require().NoError(err)
	p.Require().Len(prtbs.Data, 2)

	for _, binding := range prtbs.Data {
		p.Require().Contains(activeRoles, binding.RoleTemplateID)
	}
}

// TestUserCreateDefaultRole tests that any roles that are set as new user defaults get assigned to a new user when they are created.
func (p *RTBTestSuite) TestUserCreateDefaultRole() {
	client := p.newSubSession()

	p.ensureClusterRolesExist(client, []string{"monitoring-ui-view", "navlinks-view", "navlinks-manage"})

	testRoles := []string{"user-base", "settings-manage"}
	p.setGlobalRoleDefaults(client, testRoles)

	principal := "local://fakeuser"

	// Creating a CRTB with a fake principal triggers user creation via usermanager.EnsureUser.
	crtb, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		ClusterID:       "local",
		RoleTemplateID:  "cluster-owner",
		UserPrincipalID: principal,
	})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.ClusterRoleTemplateBinding.Delete(crtb)
	})

	// Wait for the CRTB to have userId populated.
	p.Require().Eventually(func() bool {
		c, err := client.Management.ClusterRoleTemplateBinding.ByID(crtb.ID)
		if err != nil {
			return false
		}
		crtb = c
		return crtb.UserID != ""
	}, 2*time.Minute, 2*time.Second, "waiting for userId on CRTB")

	user, err := client.Management.User.ByID(crtb.UserID)
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		_ = client.Management.User.Delete(user)
	})

	// Wait for InitialRolesPopulated condition on the user.
	p.Require().Eventually(func() bool {
		u, err := client.Management.User.ByID(user.ID)
		if err != nil {
			return false
		}
		for _, cond := range u.Conditions {
			if cond.Type == "InitialRolesPopulated" && cond.Status == "True" {
				user = u
				return true
			}
		}
		return false
	}, 2*time.Minute, 2*time.Second, "waiting for InitialRolesPopulated on user")

	grbs, err := client.Management.GlobalRoleBinding.List(&types.ListOpts{
		Filters: map[string]any{"userId": user.ID},
	})
	p.Require().NoError(err)
	p.Require().Len(grbs.Data, 2)

	for _, binding := range grbs.Data {
		p.Require().Contains(testRoles, binding.GlobalRoleID)
	}
}

// TestDefaultSystemProjectRole tests that the default and system projects have the correct roles assigned.
func (p *RTBTestSuite) TestDefaultSystemProjectRole() {
	client := p.newSubSession()

	projects, err := client.Management.Project.List(&types.ListOpts{
		Filters: map[string]any{"clusterId": "local"},
	})
	p.Require().NoError(err)

	systemProjectLabel := "authz.management.cattle.io/system-project"
	defaultProjectLabel := "authz.management.cattle.io/default-project"

	requiredProjects := map[string]string{
		"Default": defaultProjectLabel,
		"System":  systemProjectLabel,
	}

	var foundProjects []*management.Project
	for i := range projects.Data {
		prj := &projects.Data[i]
		if label, ok := requiredProjects[prj.Name]; ok {
			reloaded, err := client.Management.Project.ByID(prj.ID)
			p.Require().NoError(err)
			p.Require().Equal("true", reloaded.Labels[label])
			foundProjects = append(foundProjects, reloaded)
		}
	}

	p.Require().Len(foundProjects, len(requiredProjects))

	testRoles := []string{"project-owner"}
	for _, project := range foundProjects {
		prtbs, err := client.Management.ProjectRoleTemplateBinding.List(&types.ListOpts{
			Filters: map[string]any{"projectId": project.ID},
		})
		p.Require().NoError(err)

		for _, binding := range prtbs.Data {
			p.Require().Contains(testRoles, binding.RoleTemplateID)
		}
	}
}
