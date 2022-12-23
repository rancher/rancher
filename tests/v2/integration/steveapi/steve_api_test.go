package integration

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	clientv1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/secrets"
	stevesecrets "github.com/rancher/rancher/tests/framework/extensions/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	projectNamePrefix = "test-project"
	labelKey          = "test-label"
	continueToken     = "nondeterministictoken"
)

var (
	userEnabled                = true
	namespaceSecretManagerRole = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-secret-manager",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs: []string{
					"get",
					"list",
				},
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"secrets",
				},
			},
		},
	}
	mixedSecretUserRole = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mixed-secret-user",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs: []string{
					"get",
					"list",
				},
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"secrets",
				},
				ResourceNames: []string{
					"test1",
					"test2",
				},
			},
		},
	}
	testUsers = map[string]interface{}{
		"user-a": management.ProjectRoleTemplateBinding{
			RoleTemplateID: "project-owner",
		},
		"user-b": []rbacv1.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "namespace-secret-manager",
					Namespace: "test-ns-1",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "namespace-secret-manager",
				},
			},
		},
		"user-c": []rbacv1.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-secret-user",
					Namespace: "test-ns-1",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "mixed-secret-user",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-secret-user",
					Namespace: "test-ns-2",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "mixed-secret-user",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-secret-user",
					Namespace: "test-ns-3",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "mixed-secret-user",
				},
			},
		},
	}
	namespaceMap = map[string]string{
		"test-ns-1": "",
		"test-ns-2": "",
		"test-ns-3": "",
		"test-ns-4": "",
		"test-ns-5": "",
	}
)

type SteveAPITestSuite struct {
	suite.Suite
	client            *rancher.Client
	session           *session.Session
	project           *management.Project
	userClients       map[string]*rancher.Client
	lastContinueToken string
}

func (s *SteveAPITestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SteveAPITestSuite) SetupSuite() {
	testSession := session.NewSession(s.T())
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)
	s.client = client

	s.userClients = make(map[string]*rancher.Client)

	clusterName := s.client.RancherConfig.ClusterName
	require.NotEmptyf(s.T(), clusterName, "Cluster name is not set")
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(s.T(), err)

	// create project
	projectName := namegenerator.AppendRandomString(projectNamePrefix)
	s.project, err = s.client.Management.Project.Create(&management.Project{
		ClusterID: clusterID,
		Name:      projectName,
	})

	// create project namespaces
	for k, _ := range namespaceMap {
		name := namegenerator.AppendRandomString(k)
		_, err := namespaces.CreateNamespace(client, name, "", nil, nil, s.project)
		require.NoError(s.T(), err)
		namespaceMap[k] = name
	}

	// create resources in all namespaces
	for _, n := range namespaceMap {
		for i := 1; i <= 5; i++ {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("test%d", i),
				},
			}
			if i == 2 {
				secret.ObjectMeta.SetLabels(map[string]string{
					labelKey: "2",
				})
			}
			_, err := secrets.CreateSecret(s.client, secret, s.project.ClusterID, n)
			require.NoError(s.T(), err)
		}
	}

	// create test roles in all namespaces
	for _, n := range namespaceMap {
		role := namespaceSecretManagerRole
		role.Namespace = n
		_, err = rbac.CreateRole(s.client, s.project.ClusterID, &role)
		require.NoError(s.T(), err)
		role = mixedSecretUserRole
		role.Namespace = n
		_, err = rbac.CreateRole(s.client, s.project.ClusterID, &role)
		require.NoError(s.T(), err)
	}

	// create users and assign access
	for user, access := range testUsers {
		username := namegenerator.AppendRandomString(user)
		password := password.GenerateUserPassword("testpass")
		userObj := &management.User{
			Username: username,
			Password: password,
			Name:     username,
			Enabled:  &userEnabled,
		}
		userObj, err := s.client.Management.User.Create(userObj)
		require.NoError(s.T(), err)
		userObj.Password = password
		// users either have access to a whole project or to select namespaces or resources in a project
		switch binding := access.(type) {
		case management.ProjectRoleTemplateBinding:
			users.AddProjectMember(client, s.project, userObj, binding.RoleTemplateID)
		case []rbacv1.RoleBinding:
			for _, rb := range binding {
				subject := rbacv1.Subject{
					Kind: "User",
					Name: userObj.ID,
				}
				_, err = rbac.CreateRoleBinding(s.client, s.project.ClusterID, namegenerator.AppendRandomString(rb.Name), namespaceMap[rb.Namespace], rb.RoleRef.Name, subject)
			}
		}
		s.userClients[user], err = s.client.AsUser(userObj)
		require.NoError(s.T(), err)
	}
}

func (s *SteveAPITestSuite) TestList() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		description string
		user        string
		namespace   string
		query       string
		expect      []map[string]string
	}{
		// user-a
		{
			description: "user:user-a,namespace:none,query:none",
			user:        "user-a",
			namespace:   "",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:none",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-5,query:none",
			user:        "user-a",
			namespace:   "test-ns-5",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:labelSelector=test-label=2",
			user:        "user-a",
			namespace:   "",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-2,query:labelSelector=test-label=2",
			user:        "user-a",
			namespace:   "test-ns-2",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:fieldSelector=metadata.name=test1",
			user:        "user-a",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.name=test1",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:limit=8",
			user:        "user-a",
			namespace:   "",
			query:       "limit=8",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:limit=8&continue=" + continueToken,
			user:        "user-a",
			namespace:   "",
			query:       "limit=8&continue=" + continueToken,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:limit=3",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "limit=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:limit=3&continue=" + continueToken,
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "limit=3&continue=" + continueToken,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},

		// user-b
		{
			description: "user:user-b,namespace:none,query:none",
			user:        "user-b",
			namespace:   "",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:none",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-5,query:none",
			user:        "user-b",
			namespace:   "test-ns-5",
			query:       "",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:labelSelector=test-label=2",
			user:        "user-b",
			namespace:   "",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:labelSelector=test-label=2",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-2,query:labelSelector=test-label=2",
			user:        "user-b",
			namespace:   "test-ns-2",
			query:       "labelSelector=test-label=2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-b",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:fieldSelector=metadata.name=test1",
			user:        "user-b",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.name=test1",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.name=test1",
			user:        "user-b",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.name=test1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:limit=3",
			user:        "user-b",
			namespace:   "",
			query:       "limit=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:limit=3&continue=" + continueToken,
			user:        "user-b",
			namespace:   "",
			query:       "limit=3&continue=" + continueToken,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:limit=3",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "limit=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:limit=3&continue=" + continueToken,
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "limit=3&continue=" + continueToken,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-5,query:limit=3",
			user:        "user-b",
			namespace:   "test-ns-5",
			query:       "limit=3",
			expect:      []map[string]string{},
		},

		// user-c
		{
			description: "user:user-c,namespace:none,query:none",
			user:        "user-c",
			namespace:   "",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:none",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:none",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:labelSelector=test-label=2",
			user:        "user-c",
			namespace:   "",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:labelSelector=test-label=2",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:labelSelector=test-label=2",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "labelSelector=test-label=2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-5",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-5",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.name=test1",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.name=test5",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test5",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test1",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:fieldSelector=metadata.name=test1",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "fieldSelector=metadata.name=test1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test5",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test5",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:limit=3",
			user:        "user-c",
			namespace:   "",
			query:       "limit=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:limit=3&continue=" + continueToken,
			user:        "user-c",
			namespace:   "",
			query:       "limit=3&continue=" + continueToken,
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:limit=3",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "limit=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:limit=3",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "limit=3",
			expect:      []map[string]string{},
		},
	}

	for _, test := range tests {
		s.Run(test.description, func() {
			client, err := s.userClients[test.user].Steve.ProxyDownstream(s.project.ClusterID)
			require.NoError(s.T(), err)
			var secretClient clientv1.SteveOperations
			secretClient = client.SteveType(stevesecrets.SecretSteveType)
			if test.namespace != "" {
				secretClient = secretClient.(*clientv1.SteveClient).NamespacedSteveClient(namespaceMap[test.namespace])
			}
			query, err := url.ParseQuery(test.query)
			require.NoError(s.T(), err)
			if _, ok := query["continue"]; ok {
				query["continue"] = []string{s.lastContinueToken}
			}
			if fs, ok := query["fieldSelector"]; ok {
				if strings.Contains(fs[0], "metadata.namespace") {
					fieldParts := strings.Split(fs[0], "=")
					ns := namespaceMap[fieldParts[1]]
					query["fieldSelector"] = []string{"metadata.namespace=" + ns}
				}
			}
			secretList, err := secretClient.List(query)
			require.NoError(s.T(), err)

			if secretList.Continue != "" {
				s.lastContinueToken = secretList.Continue
			}

			assert.Equal(s.T(), len(test.expect), len(secretList.Data))
			for i, w := range test.expect {
				assert.Equal(s.T(), w["name"], secretList.Data[i].Name)
				assert.Equal(s.T(), namespaceMap[w["namespace"]], secretList.Data[i].Namespace)
			}
		})
	}
}

func (s *SteveAPITestSuite) TestCRUD() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.Steve.ProxyDownstream(s.project.ClusterID)
	require.NoError(s.T(), err)

	s.Run("global", func() {
		secretClient := client.SteveType(stevesecrets.SecretSteveType)

		// create
		secretObj, err := secretClient.Create(corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      namegenerator.AppendRandomString("steve-secret"),
				Namespace: namespaceMap["test-ns-1"], // need to specify the namespace for a namespaced resource if using a global endpoint ("/v1/secrets")
			},
			Data: map[string][]byte{"foo": []byte("bar")},
		})
		require.NoError(s.T(), err)

		// read
		readObj, err := secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "foo")

		// update
		updatedSecret := secretObj.JSONResp
		updatedSecret["data"] = map[string][]byte{"lorem": []byte("ipsum")}
		secretObj, err = secretClient.Update(secretObj, &updatedSecret)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "lorem")

		// delete
		err = secretClient.Delete(readObj)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.Error(s.T(), err)
		assert.Nil(s.T(), readObj)
	})

	s.Run("namespaced", func() {
		secretClient := client.SteveType(stevesecrets.SecretSteveType).NamespacedSteveClient(namespaceMap["test-ns-1"])

		// create
		secretObj, err := secretClient.Create(corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: namegenerator.AppendRandomString("steve-secret"),
				// no need to provide a namespace since using a namespaced endpoint ("/v1/secrets/test-ns-1")
			},
			Data: map[string][]byte{"foo": []byte("bar")},
		})
		require.NoError(s.T(), err)

		// read
		readObj, err := secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "foo")

		// update
		updatedSecret := secretObj.JSONResp
		updatedSecret["data"] = map[string][]byte{"lorem": []byte("ipsum")}
		secretObj, err = secretClient.Update(secretObj, &updatedSecret)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "lorem")

		// delete
		err = secretClient.Delete(readObj)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.Error(s.T(), err)
		assert.Nil(s.T(), readObj)
	})
}

func TestSteve(t *testing.T) {
	suite.Run(t, new(SteveAPITestSuite))
}
