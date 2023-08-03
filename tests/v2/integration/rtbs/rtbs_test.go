package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	extnamespaces "github.com/rancher/rancher/tests/framework/extensions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

type RTBTestSuite struct {
	suite.Suite
	testUser            *management.User
	client              *rancher.Client
	project             *management.Project
	session             *session.Session
	downstreamClusterID string
}

func (p *RTBTestSuite) TearDownSuite() {
	p.session.Cleanup()
}

func (p *RTBTestSuite) SetupSuite() {
	p.downstreamClusterID = "local"
	testSession := session.NewSession()
	p.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(p.T(), err)

	p.client = client

	projectConfig := &management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      "TestProject",
	}

	testProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(p.T(), err)

	p.project = testProject

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(p.T(), err)
	newUser.Password = user.Password
	p.testUser = newUser
}

func (p *RTBTestSuite) TestPRTBRoleTemplateInheritance() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

	createdNamespace, err := extnamespaces.CreateNamespace(client, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{}, p.project)
	require.NoError(p.T(), err)

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	// Test that user can get a specified secret once granted the permission to do so via roletemplate inheritance bounded
	// by a PRTB.

	secret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "rtb-test-s-"}}, "local", createdNamespace.Name)
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.Error(p.T(), err)

	rtB, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context: "project",
			Name:    "RoleB",
			Rules: []management.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"secrets"},
					ResourceNames: []string{secret.Name},
					Verbs:         []string{"get"},
				},
			},
		})
	require.NoError(p.T(), err)

	rtA, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "project",
			Name:            "RoleA",
			RoleTemplateIDs: []string{rtB.ID},
		})
	require.NoError(p.T(), err)

	err = users.AddProjectMember(client, p.project, p.testUser, rtA.ID)
	require.NoError(p.T(), err)

	secret, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.NoError(p.T(), err)

	err = users.RemoveProjectMember(client, p.testUser)
	require.NoError(p.T(), err)

	// Test that user can get a specified secret once granted the permission to do so via a chain of
	// roletemplate inheritance bounded by a PRTB. Here a chain means the permission is not directly inherited from the
	// parent.

	rtC, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "project",
			Name:            "RoleC",
			RoleTemplateIDs: []string{rtA.ID},
		})
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.Error(p.T(), err)

	err = users.AddProjectMember(client, p.project, p.testUser, rtC.ID)
	require.NoError(p.T(), err)

	secret, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.NoError(p.T(), err)

	anotherSecret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "rtb-test-s-"}}, p.downstreamClusterID, createdNamespace.Name)
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, anotherSecret.Name, metav1.GetOptions{})
	require.Error(p.T(), err)

	// Test that permissions are updated when inherited roletemplate bound by PRTB is changed.

	updatedRTB := *rtB
	updatedRTB.Rules = append(rtB.Rules, management.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"secrets"},
		ResourceNames: []string{anotherSecret.Name},
		Verbs:         []string{"get"},
	})
	dynamicClient, err := client.GetRancherDynamicClient()
	require.NoError(p.T(), err)
	var eg errgroup.Group
	timeout := int64(15)

	watchInterface, err := dynamicClient.Resource(rbac.ClusterRoleGroupVersionResource).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + rtB.ID,
		TimeoutSeconds: &timeout,
	})
	eg.Go(func() error {
		err = wait.WatchWait(watchInterface, func(event watch.Event) (bool, error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting cluster")
			}
			unstructuredCR := event.Object.(*unstructured.Unstructured)
			cr := new(rbacv1.ClusterRole)
			err := scheme.Scheme.Convert(unstructuredCR, cr, unstructuredCR.GroupVersionKind())
			if err != nil {
				return false, err
			}
			if len(cr.Rules) == 2 {
				return true, nil
			}
			return false, nil
		})
		return err
	})

	_, err = client.Management.RoleTemplate.Update(rtB, updatedRTB)
	require.NoError(p.T(), err)

	err = eg.Wait()
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, anotherSecret.Name, metav1.GetOptions{})
	require.NoError(p.T(), err)
}

func (p *RTBTestSuite) TestCRTBRoleTemplateInheritance() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

	// Test that user can get a specified namespace once granted the permission to do so via roletemplate inheritance bounded
	// by a CRTB.

	ns, err := extnamespaces.CreateNamespace(client, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{}, p.project)
	require.NoError(p.T(), err)

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.Error(p.T(), err)

	rtB, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context: "",
			Name:    "RoleB",
			Rules: []management.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{ns.Name},
					Verbs:         []string{"get"},
				},
			},
		})
	require.NoError(p.T(), err)

	rtA, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "cluster",
			Name:            "RoleA",
			RoleTemplateIDs: []string{rtB.ID},
		})
	require.NoError(p.T(), err)

	localCluster, err := p.client.Management.Cluster.ByID(p.downstreamClusterID)
	require.NoError(p.T(), err)
	err = users.AddClusterRoleToUser(client, localCluster, p.testUser, rtA.ID)
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.NoError(p.T(), err)

	err = users.RemoveClusterRoleFromUser(client, p.testUser)
	require.NoError(p.T(), err)

	// Test that user can get a specified namespace once granted the permission to do so via a chain of
	// roletemplate inheritance bounded by a CRTB. Here a chain means the permission is not directly inherited from the
	// parent.

	rtC, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "cluster",
			Name:            "RoleC",
			RoleTemplateIDs: []string{rtA.ID},
		})
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.Error(p.T(), err)

	err = users.AddClusterRoleToUser(client, localCluster, p.testUser, rtC.ID)
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.NoError(p.T(), err)

	anotherNS, err := extnamespaces.CreateNamespace(client, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{}, p.project)
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, anotherNS.Name)
	require.Error(p.T(), err)

	// Test that permissions are updated when inherited roletemplate bound by CRTB is changed.

	updatedRTB := *rtB
	updatedRTB.Rules = append(rtB.Rules, management.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"namespaces"},
		ResourceNames: []string{anotherNS.Name},
		Verbs:         []string{"get"},
	})

	dynamicClient, err := client.GetRancherDynamicClient()
	require.NoError(p.T(), err)
	var eg errgroup.Group
	timeout := int64(15)

	watchInterface, err := dynamicClient.Resource(rbac.ClusterRoleGroupVersionResource).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + rtB.ID,
		TimeoutSeconds: &timeout,
	})
	eg.Go(func() error {
		err = wait.WatchWait(watchInterface, func(event watch.Event) (bool, error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting cluster")
			}
			unstructuredCR := event.Object.(*unstructured.Unstructured)
			cr := new(rbacv1.ClusterRole)
			err := scheme.Scheme.Convert(unstructuredCR, cr, unstructuredCR.GroupVersionKind())
			if err != nil {
				return false, err
			}
			if len(cr.Rules) == 2 {
				return true, nil
			}
			return false, nil
		})
		return err
	})

	_, err = client.Management.RoleTemplate.Update(rtB, updatedRTB)
	require.NoError(p.T(), err)

	err = eg.Wait()
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, anotherNS.Name)
	require.NoError(p.T(), err)
}

func TestRTBTestSuite(t *testing.T) {
	suite.Run(t, new(RTBTestSuite))
}
