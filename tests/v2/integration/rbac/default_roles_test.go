package integration

import (
	"context"
	"time"

	"github.com/rancher/norman/types"
	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/pkg/api/scheme"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ensureClusterRolesExist ensures the given ClusterRoles exist in the downstream cluster,
// creating them if necessary, and registers test cleanup to delete any created roles.
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
	roleTemplates, err := client.Management.RoleTemplate.List(nil)
	p.Require().NoError(err)

	// Save original state for cleanup.
	originals := map[string]bool{}
	for _, rt := range roleTemplates.Data {
		if rt.ClusterCreatorDefault {
			originals[rt.ID] = true
		}
	}

	// Clear all cluster creator defaults.
	for i := range roleTemplates.Data {
		rt := &roleTemplates.Data[i]
		if rt.ClusterCreatorDefault {
			_, err := client.Management.RoleTemplate.Update(rt, map[string]any{
				"clusterCreatorDefault": false,
			})
			p.Require().NoError(err)
		}
	}

	// Set the desired roles as cluster creator defaults.
	for _, id := range roleIDs {
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
		}
	})
}

// setProjectCreatorDefaults sets the given roles as project creator defaults. When a project is created these roles will be bound to the creator.
// Clears all projectCreatorDefault flags, then sets them on the given role IDs.
// Registers a cleanup to restore original defaults.
func (p *RTBTestSuite) setProjectCreatorDefaults(client *rancher.Client, roleIDs []string) {
	roleTemplates, err := client.Management.RoleTemplate.List(nil)
	p.Require().NoError(err)

	originals := map[string]bool{}
	for _, rt := range roleTemplates.Data {
		if rt.ProjectCreatorDefault {
			originals[rt.ID] = true
		}
	}

	for i := range roleTemplates.Data {
		rt := &roleTemplates.Data[i]
		if rt.ProjectCreatorDefault {
			_, err := client.Management.RoleTemplate.Update(rt, map[string]any{
				"projectCreatorDefault": false,
			})
			p.Require().NoError(err)
		}
	}

	for _, id := range roleIDs {
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

	// Reset the locked state of the role after the test.
	previousLocked := rt.Locked
	p.T().Cleanup(func() {
		_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{"locked": previousLocked})
	})

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

	// Reset the locked state of the role after the test.
	previousLocked := rt.Locked
	p.T().Cleanup(func() {
		_, _ = client.Management.RoleTemplate.Update(rt, map[string]any{"locked": previousLocked})
	})

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
