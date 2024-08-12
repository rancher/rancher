package integration

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	kubenamespaces "github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/secrets"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	stevesecrets "github.com/rancher/rancher/tests/v2/actions/secrets"
	"github.com/rancher/rancher/tests/v2/actions/serviceaccounts"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	clientv1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

const (
	labelKey          = "test-label"
	labelGTEKey       = "test-label-gte"
	continueToken     = "nondeterministictoken"
	revisionNum       = "nondeterministicint"
	fakeTestID        = "nondeterministicid"
	defautlUrlString  = "https://rancherurl/"
	steveAPITestLabel = "test.cattle.io/steveapi"
)

var (
	testID                     = namegenerator.RandStringLower(5)
	userEnabled                = true
	impersonationNamespace     = "cattle-impersonation-system"
	impersonationSABase        = "cattle-impersonation-"
	urlRegex                   = regexp.MustCompile(`https://([\w.:]+)/`)
	continueReg                = regexp.MustCompile(`(continue=)[\w]+(%3D){0,2}`)
	revisionReg                = regexp.MustCompile(`(revision=)[\d]+`)
	testLabelReg               = regexp.MustCompile(`(labelSelector=test.cattle.io%2Fsteveapi%3D)[\w]+`)
	projectTag                 = regexp.MustCompile(`(test-prj-[1-9])`)
	namespaceTag               = regexp.MustCompile(`(test-ns-[1-9])`)
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
	testUsers = map[string][]interface{}{
		"user-a": {
			management.ProjectRoleTemplateBinding{
				RoleTemplateID: "project-owner",
				ProjectID:      "test-prj-1",
			},
		},
		"user-b": {
			rbacv1.RoleBinding{
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
		"user-c": {
			rbacv1.RoleBinding{
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
			rbacv1.RoleBinding{
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
			rbacv1.RoleBinding{
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
		"user-d": {
			management.ProjectRoleTemplateBinding{
				RoleTemplateID: "project-owner",
				ProjectID:      "test-prj-1",
			},
			management.ProjectRoleTemplateBinding{
				RoleTemplateID: "project-owner",
				ProjectID:      "test-prj-2",
			},
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "namespace-secret-manager",
					Namespace: "test-ns-8",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "namespace-secret-manager",
				},
			},
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "namespace-secret-manager",
					Namespace: "test-ns-9",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "namespace-secret-manager",
				},
			},
		},
		"user-e": {
			management.ClusterRoleTemplateBinding{
				RoleTemplateID: "cluster-owner",
			},
		},
	}
	namespaceMap = map[string]string{
		"test-ns-1": "",
		"test-ns-2": "",
		"test-ns-3": "",
		"test-ns-4": "",
		"test-ns-5": "",
		"test-ns-6": "",
		"test-ns-7": "",
		"test-ns-8": "",
		"test-ns-9": "",
	}
	projectMap = map[string]*management.Project{
		"test-prj-1": nil,
		"test-prj-2": nil,
	}
	projectNamespaceMap = map[string]string{
		"test-ns-1": "test-prj-1",
		"test-ns-2": "test-prj-1",
		"test-ns-3": "test-prj-1",
		"test-ns-4": "test-prj-1",
		"test-ns-5": "test-prj-1",
		"test-ns-6": "test-prj-2",
		"test-ns-7": "test-prj-2",
		"test-ns-8": "",
		"test-ns-9": "",
	}
)

type steveAPITestSuite struct {
	suite.Suite
	client            *rancher.Client
	session           *session.Session
	clusterID         string
	userClients       map[string]*rancher.Client
	lastContinueToken string
	lastRevision      string
}

type LocalSteveAPITestSuite struct {
	steveAPITestSuite
}

type DownstreamSteveAPITestSuite struct {
	steveAPITestSuite
}

func (s *steveAPITestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *LocalSteveAPITestSuite) SetupSuite() {
	s.steveAPITestSuite.setupSuite("local")
}

func (s *DownstreamSteveAPITestSuite) SetupSuite() {
	s.steveAPITestSuite.setupSuite("")
}

func (s *steveAPITestSuite) setupSuite(clusterName string) {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)
	s.client = client

	s.userClients = make(map[string]*rancher.Client)

	if clusterName == "" {
		clusterName = s.client.RancherConfig.ClusterName
	}
	s.clusterID, err = clusters.GetClusterIDByName(client, clusterName)
	require.NoError(s.T(), err)

	mgmtCluster, err := client.Management.Cluster.ByID(s.clusterID)
	require.NoError(s.T(), err)

	// create projects
	for p := range projectMap {
		project, err := s.client.Management.Project.Create(&management.Project{
			ClusterID: s.clusterID,
			Name:      p,
		})
		require.NoError(s.T(), err)
		projectMap[p] = project
	}

	userID, err := users.GetUserIDByName(client, "admin")
	require.NoError(s.T(), err)

	impersonationSA := impersonationSABase + userID
	err = serviceaccounts.IsServiceAccountReady(client, s.clusterID, impersonationNamespace, impersonationSA)
	require.NoError(s.T(), err)

	// create project namespaces
	for n := range namespaceMap {
		if projectMap[projectNamespaceMap[n]] == nil {
			continue
		}
		name := namegenerator.AppendRandomString(n)
		_, err := namespaces.CreateNamespace(client, name, "", nil, nil, projectMap[projectNamespaceMap[n]])
		require.NoError(s.T(), err)
		namespaceMap[n] = name
	}
	// create non project namespaces
	for n := range namespaceMap {
		if projectMap[projectNamespaceMap[n]] != nil {
			continue
		}
		name := namegenerator.AppendRandomString(n)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		dynamicClient, err := client.GetDownStreamClusterClient(s.clusterID)
		require.NoError(s.T(), err)
		namespaceResource := dynamicClient.Resource(kubenamespaces.NamespaceGroupVersionResource)
		resp, err := namespaceResource.Create(context.TODO(), unstructured.MustToUnstructured(ns), metav1.CreateOptions{})
		require.NoError(s.T(), err)
		s.client.Session.RegisterCleanupFunc(func() error {
			err := namespaceResource.Delete(context.TODO(), resp.GetName(), metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		})
		err = scheme.Scheme.Convert(resp, ns, resp.GroupVersionKind())
		require.NoError(s.T(), err)
		err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
			ns, _ := kubenamespaces.GetNamespaceByName(s.client, s.clusterID, ns.Name)
			if ns != nil {
				return true, nil
			}
			return false, nil
		})
		require.NoError(s.T(), err)
		namespaceMap[n] = name
	}

	// create resources in all namespaces
	for name, n := range namespaceMap {
		for i := 1; i <= 5; i++ {
			if i > 2 && (projectNamespaceMap[name] == "test-prj-2" || projectNamespaceMap[name] == "") {
				break
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("test%d", i),
				},
			}
			labels := map[string]string{steveAPITestLabel: testID}
			if i == 2 {
				labels[labelKey] = "2"
			}
			if i >= 3 {
				labels[labelGTEKey] = "3"
			}
			secret.ObjectMeta.SetLabels(labels)
			err := retryRequest(func() error {
				_, err := secrets.CreateSecretForCluster(s.client, secret, s.clusterID, n)
				if apierrors.IsAlreadyExists(err) {
					return nil
				}
				return err
			})
			require.NoError(s.T(), err)
		}
	}

	// create test roles in all namespaces
	for _, n := range namespaceMap {
		role := namespaceSecretManagerRole
		role.Namespace = n
		err := retryRequest(func() error {
			_, err = rbac.CreateRole(s.client, s.clusterID, &role)
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		})
		require.NoError(s.T(), err)
		role = mixedSecretUserRole
		role.Namespace = n
		err = retryRequest(func() error {
			_, err = rbac.CreateRole(s.client, s.clusterID, &role)
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		})
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
		for _, binding := range access {
			switch b := binding.(type) {
			case management.ClusterRoleTemplateBinding:
				err = users.AddClusterRoleToUser(client, mgmtCluster, userObj, b.RoleTemplateID, nil)
				require.NoError(s.T(), err)
			case management.ProjectRoleTemplateBinding:
				err = users.AddProjectMember(client, projectMap[b.ProjectID], userObj, b.RoleTemplateID, nil)
				require.NoError(s.T(), err)
			case rbacv1.RoleBinding:
				_ = users.AddClusterRoleToUser(client, mgmtCluster, userObj, "cluster-member", nil)
				subject := rbacv1.Subject{
					Kind: "User",
					Name: userObj.ID,
				}
				err := retryRequest(func() error {
					_, err = rbac.CreateRoleBinding(s.client, s.clusterID, namegenerator.AppendRandomString(b.Name), namespaceMap[b.Namespace], b.RoleRef.Name, subject)
					if apierrors.IsAlreadyExists(err) {
						return nil
					}
					return err
				})
				require.NoError(s.T(), err)
			}
		}
		s.userClients[user], err = s.client.AsUser(userObj)
		require.NoError(s.T(), err)
	}
}

func (s *steveAPITestSuite) TestList() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		description    string
		user           string
		namespace      string
		query          string
		expect         []map[string]string
		expectExcludes bool
		expectContains bool
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
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name=test1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name=test6",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name=test6",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:filter=metadata.name=test1",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name=1,metadata.namespace=1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name=1,metadata.namespace=1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name!=1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name!=1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:sort=metadata.name",
			user:        "user-a",
			namespace:   "",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:sort=-metadata.name",
			user:        "user-a",
			namespace:   "",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:sort=metadata.name,metadata.namespace",
			user:        "user-a",
			namespace:   "",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:sort=metadata.name,-metadata.namespace",
			user:        "user-a",
			namespace:   "",
			query:       "sort=metadata.name,-metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:sort=metadata.name",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:sort=-metadata.name",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:pagesize=8",
			user:        "user-a",
			namespace:   "",
			query:       "pagesize=8",
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
			description: "user:user-a,namespace:none,query:pagesize=8&page=2&revision=" + revisionNum,
			user:        "user-a",
			namespace:   "",
			query:       "pagesize=8&page=2&revision=" + revisionNum,
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
			description: "user:user-a,namespace:test-ns-1,query:pagesize=3",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&limit=20",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&limit=20",
			// limit is applied BEFORE filter and pagesize, which is why not all test5 secrets appear in the result
			expect: []map[string]string{
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test4"},
				{"name": "test4"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=2&revision=" + revisionNum + "&limit=20",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=2&revision=" + revisionNum + "&limit=20",
			expect: []map[string]string{
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=1&limit=20&continue=" + continueToken,
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=1&limit=20&continue=" + continueToken,
			// the remaining chunk is all from test-ns-5
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
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
		{
			description: "user:user-b,namespace:none,query:filter=metadata.name=test1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:filter=metadata.name=test1",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:filter=metadata.name=test6",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test6",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.name=1,metadata.namespace=1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.name=1,metadata.namespace=1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.name!=1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.name!=1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=metadata.name",
			user:        "user-b",
			namespace:   "",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=-metadata.name",
			user:        "user-b",
			namespace:   "",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=metadata.name,metadata.namespace",
			user:        "user-b",
			namespace:   "",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=-metadata.name,metadata.namespace",
			user:        "user-b",
			namespace:   "",
			query:       "sort=-metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:sort=metadata.name",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:sort=-metadata.name",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-5,query:sort=metadata.name",
			user:        "user-b",
			namespace:   "test-ns-5",
			query:       "sort=metadata.name",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:pagesize=3",
			user:        "user-b",
			namespace:   "",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-b",
			namespace:   "",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:pagesize=3",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-5,query:pagesize=3",
			user:        "user-b",
			namespace:   "test-ns-5",
			query:       "pagesize=3",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2&page=2&revision=" + revisionNum,
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test3", "namespace": "test-ns-1"},
			},
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
		{
			description: "user:user-c,namespace:none,query:filter=metadata.name=test1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:filter=metadata.name=test1",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:filter=metadata.name=test3",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test3",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.name=1,metadata.namespace=1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.name=1,metadata.namespace=1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:filter=metadata.name!=test1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.name!=test1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=metadata.name",
			user:        "user-c",
			namespace:   "",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=-metadata.name",
			user:        "user-c",
			namespace:   "",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=metadata.name,metadata.namespace",
			user:        "user-c",
			namespace:   "",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=metadata.name,-metadata.namespace",
			user:        "user-c",
			namespace:   "",
			query:       "sort=metadata.name,-metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=metadata.name",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=-metadata.name",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=metadata.name,metadata.namespace",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=metadata.name,-metadata.namespace",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name,-metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:sort=metadata.name",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "sort=metadata.name",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:pagesize=3",
			user:        "user-c",
			namespace:   "",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-c",
			namespace:   "",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:pagesize=3",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:pagesize=3",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "pagesize=3",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1&page=2&revision=" + revisionNum,
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},

		// user-d
		{
			description: "user:user-d,namespace:none,query:none",
			user:        "user-d",
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-prj-2",
			user:        "user-d",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-prj-1,test-prj-2",
			user:        "user-d",
			query:       "projectsornamespaces=test-prj-1,test-prj-2",
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-ns-1",
			user:        "user-d",
			query:       "projectsornamespaces=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-ns-1,test-ns-2",
			user:        "user-d",
			query:       "projectsornamespaces=test-ns-1,test-ns-2",
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
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			user:        "user-d",
			query:       "projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			expect: []map[string]string{
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-ns-8,test-ns-9",
			user:        "user-d",
			query:       "projectsornamespaces=test-ns-8,test-ns-9",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces!=test-prj-1",
			user:        "user-d",
			query:       "projectsornamespaces!=test-prj-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-d",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
			user:        "user-d",
			query:       "projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-6,query:none",
			user:        "user-d",
			namespace:   "test-ns-6",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-6,query:projectsornamespaces=test-prj-2",
			user:        "user-d",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-6,query:projectsornamespaces=test-prj-2",
			user:        "user-d",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-d,namespace:test-ns-1,query:projectsornamespaces=test-ns-1,test-ns-2,-test-prj-2,test-ns-7",
			user:        "user-d",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces=test-ns-1,test-ns-2,test-prj-2,test-ns-7",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1",
			user:        "user-d",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-d,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-d",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
			expect:      []map[string]string{},
		},

		// user-e
		{
			description: "user:user-e,namespace:none,query:none",
			user:        "user-e",
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
			expectContains: true,
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-prj-2",
			user:        "user-e",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-prj-1,test-prj-2",
			user:        "user-e",
			query:       "projectsornamespaces=test-prj-1,test-prj-2",
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-ns-1",
			user:        "user-e",
			query:       "projectsornamespaces=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-ns-1,test-ns-2",
			user:        "user-e",
			query:       "projectsornamespaces=test-ns-1,test-ns-2",
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
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			user:        "user-e",
			query:       "projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			expect: []map[string]string{
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-ns-8,test-ns-9",
			user:        "user-e",
			query:       "projectsornamespaces=test-ns-8,test-ns-9",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces!=test-prj-1",
			user:        "user-e",
			query:       "projectsornamespaces!=test-prj-1",
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
			expectExcludes: true,
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-e",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
			expectExcludes: true,
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
			user:        "user-e",
			query:       "projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
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
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
			},
			expectExcludes: true,
		},
		{
			description: "user:user-e,namespace:test-ns-6,query:none",
			user:        "user-e",
			namespace:   "test-ns-6",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-e,namespace:test-ns-6,query:projectsornamespaces=test-prj-2",
			user:        "user-e",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-e,namespace:test-ns-6,query:projectsornamespaces=test-prj-1",
			user:        "user-e",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces=test-ns-1,test-ns-2,test-prj-2,test-ns-7",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces=test-ns-1,test-ns-2,test-prj-2,test-ns-7",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1,test-ns-2,test-ns-8",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1,test-ns-2,test-ns-8",
			expect:      []map[string]string{},
		},
	}

	var csvWriter *csv.Writer
	var jsonDir string
	if s.clusterID == "local" {
		var fp *os.File
		var err error
		csvWriter, fp, jsonDir, err = setUpResults()
		defer fp.Close()
		defer func() {
			csvWriter.Flush()
			require.NoError(s.T(), csvWriter.Error())
		}()
		require.NoError(s.T(), err)
	}

	for _, test := range tests {
		s.Run(test.description, func() {
			userClient := s.userClients[test.user]

			client, err := userClient.Steve.ProxyDownstream(s.clusterID)
			require.NoError(s.T(), err)
			var secretClient clientv1.SteveOperations
			secretClient = client.SteveType(stevesecrets.SecretSteveType)
			if test.namespace != "" {
				secretClient = secretClient.(*clientv1.SteveClient).NamespacedSteveClient(namespaceMap[test.namespace])
			}
			query, err := url.ParseQuery(test.query)
			require.NoError(s.T(), err)
			if _, ok := query["sort"]; !ok && s.clusterID != "local" {
				// k8s does not guarantee any particular order but usually returns results sorted by namespace and name.
				// k3d seems to have its own ideas, so we can't rely on a consistent order when testing on the downstream cluster.
				query["sort"] = []string{"metadata.namespace,metadata.name"}
			}
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
			key := "projectsornamespaces"
			projectsOrNamespaces, ok := query[key]
			if !ok {
				key += "!"
				projectsOrNamespaces = query[key]
			}
			if len(projectsOrNamespaces) != 0 {
				groups := projectTag.FindAllStringSubmatch(projectsOrNamespaces[0], -1)
				for _, g := range groups {
					name := string(g[1])
					projectID := projectMap[name].ID
					projectID = strings.Split(projectID, ":")[1]
					projectsOrNamespaces[0] = strings.ReplaceAll(projectsOrNamespaces[0], name, projectID)
				}
				groups = namespaceTag.FindAllStringSubmatch(projectsOrNamespaces[0], -1)
				for _, g := range groups {
					name := string(g[1])
					projectsOrNamespaces[0] = strings.ReplaceAll(projectsOrNamespaces[0], name, namespaceMap[name])
				}
				query[key] = projectsOrNamespaces
			}
			if _, ok := query["revision"]; ok {
				query["revision"] = []string{s.lastRevision}
			}
			query["labelSelector"] = append(query["labelSelector"], steveAPITestLabel+"="+testID)
			secretList, err := secretClient.List(query)
			require.NoError(s.T(), err)

			if secretList.Continue != "" {
				s.lastContinueToken = secretList.Continue
			}
			s.lastRevision = secretList.Revision

			if test.expectContains {
				s.assertListContains(test.expect, secretList.Data)
			} else if test.expectExcludes {
				s.assertListExcludes(test.expect, secretList.Data)
			} else {
				s.assertListIsEqual(test.expect, secretList.Data)
			}

			// Write human-readable request and response examples
			if s.clusterID == "local" {
				curlURL, err := getCurlURL(client, test.namespace, test.query)
				require.NoError(s.T(), err)
				jsonResp, err := formatJSON(secretList)
				require.NoError(s.T(), err)
				jsonFilePath := filepath.Join(jsonDir, getFileName(test.user, test.namespace, test.query))
				err = writeResp(csvWriter, test.user, curlURL, jsonFilePath, jsonResp)
				require.NoError(s.T(), err)
			}
		})
	}
}

func getFileName(user, ns, query string) string {
	if user == "" {
		user = "none"
	}
	if ns == "" {
		ns = "none"
	}
	if query == "" {
		query = "none"
	}
	return user + "_" + ns + "_" + query + ".json"
}

func getCurlURL(client *clientv1.Client, namespace, query string) (string, error) {
	curlURL, err := client.APIBaseClient.Ops.GetCollectionURL(stevesecrets.SecretSteveType, "GET")
	if err != nil {
		return "", err
	}
	if namespace != "" {
		curlURL += "/" + namespace
	}
	if query != "" {
		curlURL += "?" + query
	}

	curlURL = urlRegex.ReplaceAllString(curlURL, defautlUrlString)
	return curlURL, nil
}

func formatJSON(obj *clientv1.SteveCollection) ([]byte, error) {
	jsonResp, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var mapResp map[string]interface{}
	err = json.Unmarshal(jsonResp, &mapResp)
	if err != nil {
		return nil, err
	}

	mapResp["revision"] = "100"
	if _, ok := mapResp["continue"]; ok {
		mapResp["continue"] = continueToken
	}
	if pagination, ok := mapResp["pagination"].(map[string]interface{}); ok {
		if next, ok := pagination["next"].(string); ok {
			next = continueReg.ReplaceAllString(next, "${1}"+continueToken)
			next = revisionReg.ReplaceAllString(next, "${1}"+revisionNum)
			next = testLabelReg.ReplaceAllString(next, "${1}"+fakeTestID)
			pagination["next"] = next
			mapResp["pagination"] = pagination
		}
	}
	data, ok := mapResp["data"].([]interface{})
	if ok {
		for i := range data {
			delete(data[i].(map[string]interface{}), "JSONResp")
			delete(data[i].(map[string]interface{})["metadata"].(map[string]interface{}), "creationTimestamp")
			delete(data[i].(map[string]interface{})["metadata"].(map[string]interface{}), "managedFields")
			delete(data[i].(map[string]interface{})["metadata"].(map[string]interface{}), "uid")
			data[i].(map[string]interface{})["metadata"].(map[string]interface{})["labels"].(map[string]interface{})[steveAPITestLabel] = fakeTestID
			data[i].(map[string]interface{})["metadata"].(map[string]interface{})["resourceVersion"] = "1000"
		}
		mapResp["data"] = data
	}
	jsonBytes, err := json.MarshalIndent(mapResp, "", "  ")
	if err != nil {
		return nil, err
	}
	jsonString := string(jsonBytes)
	for k, v := range namespaceMap {
		jsonString = strings.ReplaceAll(jsonString, v, k)
	}
	jsonString = urlRegex.ReplaceAllString(jsonString, defautlUrlString)
	return []byte(jsonString), nil
}

func setUpResults() (*csv.Writer, *os.File, string, error) {
	outputFile := "output.csv"
	fields := []string{"user", "url", "response"}
	csvFile, err := os.OpenFile(outputFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, "", err
	}
	csvWriter := csv.NewWriter(bufio.NewWriter(csvFile))
	csvWriter.Write(fields)
	if csvWriter.Error() != nil {
		return nil, csvFile, "", err
	}
	jsonDir := "json"
	err = os.MkdirAll(jsonDir, 0755)
	if err != nil {
		return nil, csvFile, "", err
	}
	return csvWriter, csvFile, jsonDir, nil
}

func writeResp(csvWriter *csv.Writer, user, url, path string, resp []byte) error {
	jsonFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	jsonFile.Write(resp)
	csvWriter.Write([]string{user, url, fmt.Sprintf("[%s](%s)", path, path)})
	return nil
}

func (s *steveAPITestSuite) TestCRUD() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.Steve.ProxyDownstream(s.clusterID)
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
func (s *steveAPITestSuite) assertListIsEqual(expect []map[string]string, list []clientv1.SteveAPIObject) {
	assert.Equal(s.T(), len(expect), len(list))
	for i, w := range expect {
		if name, ok := w["name"]; ok {
			assert.Equal(s.T(), name, list[i].Name)
		}
		if ns, ok := w["namespace"]; ok {
			assert.Equal(s.T(), namespaceMap[ns], list[i].Namespace)
		}
	}
}

func (s *steveAPITestSuite) assertListContains(expect []map[string]string, list []clientv1.SteveAPIObject) {
	assert.GreaterOrEqual(s.T(), len(list), len(expect))
	matches := true
	for _, w := range expect {
		found := false
		for _, obj := range list {
			if obj.Name == w["name"] && obj.Namespace == namespaceMap[w["namespace"]] {
				found = true
				break
			}
		}
		if !found {
			matches = false
		}
	}
	assert.True(s.T(), matches, "list did not contain expected results")
}

func (s *steveAPITestSuite) assertListExcludes(expect []map[string]string, list []clientv1.SteveAPIObject) {
	found := false
	for _, w := range expect {
		for _, obj := range list {
			if obj.Name == w["name"] && obj.Namespace == namespaceMap[w["namespace"]] {
				found = true
				break
			}
		}
		if found == true {
			break
		}
	}
	assert.False(s.T(), found, "list contained unexpected results")
}

func retryRequest(fn func() error) error {
	retriable := func(err error) bool { return strings.Contains(err.Error(), "tunnel disconnect") }
	return retry.OnError(retry.DefaultBackoff, retriable, fn)
}

func TestSteveLocal(t *testing.T) {
	suite.Run(t, new(LocalSteveAPITestSuite))
}

func TestSteveDownstream(t *testing.T) {
	// TODO: Re-enable the test when the bug is fixed
	t.Skip()
	suite.Run(t, new(DownstreamSteveAPITestSuite))
}
